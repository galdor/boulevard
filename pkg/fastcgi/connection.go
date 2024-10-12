package fastcgi

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"slices"
	"sync"

	"go.n16f.net/boulevard/pkg/netutils"
)

var (
	ErrServerOverloaded = errors.New("server overloaded")
	ErrRequestCancelled = errors.New("request cancelled")
	ErrRequestTimeout   = errors.New("request timeout")
)

type Connection struct {
	Client *Client

	conn net.Conn

	values     NameValuePairs
	valueMutex sync.Mutex
}

func NewConnection(ctx context.Context, client *Client) (*Connection, error) {
	c := Connection{
		Client: client,
	}

	address := client.Cfg.Address

	var dialer net.Dialer
	conn, err := dialer.DialContext(ctx, "tcp", address)
	if err != nil {
		err = netutils.UnwrapOpError(err, "dial")
		return nil, fmt.Errorf("cannot connect to %q: %v", address, err)
	}
	c.conn = conn

	if err := c.fetchValues(); err != nil {
		c.Close()
		return nil, fmt.Errorf("cannot fetch values: %w", err)
	}

	return &c, nil
}

func (c *Connection) Close() {
	if c.conn != nil {
		c.conn.Close()
		c.conn = nil
	}
}

func (c *Connection) Values() NameValuePairs {
	c.valueMutex.Lock()
	values := slices.Clone(c.values)
	c.valueMutex.Unlock()

	return values
}

func (c *Connection) fetchValues() error {
	names := []string{
		"FCGI_MAX_CONNS",
		"FCGI_MAX_REQS",
		"FCGI_MPXS_CONNS",
	}

	body := GetValuesBody{
		Names: names,
	}

	if err := c.writeRecord(RecordTypeGetValues, 0, &body); err != nil {
		return fmt.Errorf("cannot write record: %w", err)
	}

	var r Record
	if err := r.Read(c.conn); err != nil {
		return fmt.Errorf("cannot read record: %w", err)
	}

	if r.RecordType != RecordTypeGetValuesResult {
		return fmt.Errorf("received unexpected record %q, expected record %q",
			r.RecordType, RecordTypeGetValuesResult)
	}

	if err := c.processValues(r.Body.(*GetValuesResultBody)); err != nil {
		return fmt.Errorf("cannot process values: %w", err)
	}

	return nil
}

func (c *Connection) processValues(body *GetValuesResultBody) error {
	c.valueMutex.Lock()
	c.values = body.Pairs
	c.valueMutex.Unlock()

	return nil
}

func (c *Connection) SendRequest(ctx context.Context, role Role, params NameValuePairs, stdin, data io.Reader, stdout io.Writer) (*Header, error) {
	var header *Header

	errChan := make(chan error)
	defer close(errChan)

	go func() {
		if err := c.writeRequest(role, params, stdin, data, 1); err != nil {
			errChan <- fmt.Errorf("cannot write request: %w", err)
			return
		}

		var err error
		header, err = c.readResponse(stdout)
		if err != nil {
			errChan <- fmt.Errorf("cannot read response: %w", err)
			return
		}

		errChan <- nil
	}()

	select {
	case err := <-errChan:
		return header, err

	case <-ctx.Done():
		c.Close()
		<-errChan

		err := ctx.Err()
		if err == context.Canceled {
			err = ErrRequestCancelled
		} else if err == context.DeadlineExceeded {
			err = ErrRequestTimeout
		}

		return nil, err
	}
}

func (c *Connection) readResponse(stdout io.Writer) (*Header, error) {
	var header Header
	var headerRead bool
	var stderr bytes.Buffer

loop:
	for {
		var r Record
		if err := r.Read(c.conn); err != nil {
			return nil, fmt.Errorf("cannot read record: %w", err)
		}

		switch r.RecordType {
		case RecordTypeGetValuesResult:
			err := c.processValues(r.Body.(*GetValuesResultBody))
			if err != nil {
				return nil, fmt.Errorf("cannot process values: %w", err)
			}

		case RecordTypeUnknownType:
			return nil, fmt.Errorf("server does not support record type %d",
				r.Body.(*UnknownTypeBody).Type)

		case RecordTypeEndRequest:
			body := r.Body.(*EndRequestBody)

			var err error

			if body.ProtocolStatus == ProtocolStatusCannotMultiplexConnection {
				err = ErrServerOverloaded
			} else if body.ProtocolStatus == ProtocolStatusOverloaded {
				err = ErrServerOverloaded
			} else if body.ProtocolStatus == ProtocolStatusUnknownRole {
				err = errors.New("unknown request role")
			} else if body.AppStatus != 0 {
				err = &AppError{Status: body.AppStatus, Stderr: stderr.String()}
			} else if !headerRead {
				err = fmt.Errorf("request ended without response")
			}

			if err != nil {
				return nil, err
			}

			break loop

		case RecordTypeStdout:
			body := r.Body.([]byte)

			if !headerRead {
				headerEnd, rest, err := header.Parse(body)
				if err != nil {
					return nil, fmt.Errorf("cannot parse response header: %w",
						err)
				}

				if headerEnd {
					headerRead = true
					body = rest
				}
			}

			if _, err := stdout.Write(body); err != nil {
				return nil, fmt.Errorf("cannot write stdout: %w", err)
			}

		case RecordTypeStderr:
			stderr.Write(r.Body.([]byte))

		default:
			return nil, fmt.Errorf("unhandled record type %q", r.RecordType)
		}
	}

	return &header, nil
}

func (c *Connection) writeRequest(role Role, params NameValuePairs, stdin, data io.Reader, reqId uint16) error {
	beginReq := BeginRequestBody{
		Role:           role,
		KeepConnection: true,
	}

	err := c.writeRecord(RecordTypeBeginRequest, reqId, &beginReq)
	if err != nil {
		return fmt.Errorf("cannot write %q record: %w",
			RecordTypeBeginRequest, err)
	}

	paramData, err := params.Encode()
	if err != nil {
		return fmt.Errorf("cannot encode parameters: %w", err)
	}

	paramReader := bytes.NewReader(paramData)
	if err := c.writeStream(RecordTypeParams, reqId, paramReader); err != nil {
		return fmt.Errorf("cannot write parameter stream: %w", err)
	}

	// 6.1 "When a role protocol calls for transmitting a stream other than
	// FCGI_STDERR, at least one record of the stream type is always
	// transmitted, even if the stream is empty."

	if (role == RoleResponder || role == RoleFilter) && stdin == nil {
		stdin = bytes.NewReader([]byte(nil))
	}

	if stdin != nil {
		if err := c.writeStream(RecordTypeStdin, reqId, stdin); err != nil {
			return fmt.Errorf("cannot write stdin stream: %w", err)
		}
	}

	if role == RoleFilter && data == nil {
		data = bytes.NewReader([]byte(nil))
	}

	if data != nil {
		if err := c.writeStream(RecordTypeData, reqId, data); err != nil {
			return fmt.Errorf("cannot write data stream: %w", err)
		}
	}

	return nil
}

func (c *Connection) writeRecord(rtype RecordType, id uint16, body any) error {
	record := Record{
		Version:    1,
		RecordType: rtype,
		RequestId:  id,
		Body:       body,
	}

	if err := record.Write(c.conn); err != nil {
		err = netutils.UnwrapOpError(err, "writev")
		err = netutils.UnwrapOpError(err, "write")
		return err
	}

	return nil
}

func (c *Connection) writeStream(rtype RecordType, id uint16, stream io.Reader) error {
	// It would be interesting to benchmark different buffer size. Maximum is
	// 65535 bytes since the record size is an unsigned 16 bit integer.
	const bufSize = 16 * 1024

	var buf [bufSize]byte

	for {
		n, err := stream.Read(buf[:])
		if err != nil {
			if err == io.EOF {
				break
			}

			return fmt.Errorf("cannot read stream: %w", err)
		}

		if err := c.writeRecord(rtype, id, buf[:n]); err != nil {
			return fmt.Errorf("cannot write record: %w", err)
		}
	}

	if err := c.writeRecord(rtype, id, nil); err != nil {
		return fmt.Errorf("cannot write record: %w", err)
	}

	return nil
}

type AppError struct {
	Status uint32
	Stderr string
}

func (err *AppError) Error() string {
	msg := fmt.Sprintf("application failed with status %d", err.Status)
	if err.Stderr != "" {
		msg += ": " + err.Stderr
	}

	return msg
}
