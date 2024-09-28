package fastcgi

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"math"
	"net"
	"slices"
	"strconv"
	"sync"

	"go.n16f.net/boulevard/pkg/netutils"
	"go.n16f.net/ejson"
	"go.n16f.net/log"
)

var (
	ErrClientShutdown            = errors.New("client shutdown in progress")
	ErrServerOverloaded          = errors.New("server overloaded")
	ErrTooManyConcurrentRequests = errors.New("too many concurrent requests")
)

type ClientCfg struct {
	Log *log.Logger `json:"-"`

	Address string `json:"address"`
}

func (cfg *ClientCfg) ValidateJSON(v *ejson.Validator) {
	v.CheckNetworkAddress("address", cfg.Address)
}

type Client struct {
	Cfg *ClientCfg
	Log *log.Logger `json:"-"`

	conn net.Conn

	readerWg sync.WaitGroup

	values     NameValuePairs
	valueMutex sync.Mutex

	maxConcurrentConnections int
	maxConcurrentRequests    int
	multiplexConnections     bool

	requests     []*Request
	requestMutex sync.Mutex
}

type Request struct {
	Id uint16

	Stdout io.Writer
	Stderr io.Writer

	ResponseChan chan<- *Response
}

type Response struct {
	AppStatus      uint32
	ProtocolStatus ProtocolStatus
	Error          error
}

func NewClient(cfg *ClientCfg) (*Client, error) {
	c := Client{
		Cfg: cfg,
		Log: cfg.Log,

		maxConcurrentConnections: 1,
		maxConcurrentRequests:    1,
		multiplexConnections:     false,
	}

	// We do not need to establish a connection until the first request, but
	// doing so lets us validate the address.
	if err := c.reconnect(); err != nil {
		return nil, err
	}

	return &c, nil
}

func (c *Client) Close() {
	// Careful, the function must not be closed from the reader goroutine since
	// it waits for the reader to terminate.

	c.deleteRequests()

	if c.conn != nil {
		c.conn.Close()
		c.readerWg.Wait()

		c.conn = nil
	}
}

func (c *Client) Values() NameValuePairs {
	c.valueMutex.Lock()
	values := slices.Clone(c.values)
	c.valueMutex.Unlock()

	return values
}

func (c *Client) SendRequest(role Role, params NameValuePairs, stdin, data io.Reader, stdout, stderr io.Writer) (uint32, error) {
	if c.conn == nil {
		if err := c.reconnect(); err != nil {
			return 0, err
		}
	}

	reqId, resChan, found := c.createRequest(stdout, stderr)
	if !found {
		return 0, ErrTooManyConcurrentRequests
	}
	defer close(resChan)

	beginReq := &BeginRequestBody{
		Role:           role,
		KeepConnection: true,
	}

	err := c.writeRecord(RecordTypeBeginRequest, reqId, beginReq)
	if err != nil {
		c.deleteRequest(reqId)
		return 0, fmt.Errorf("cannot write %q record: %w",
			RecordTypeBeginRequest, err)
	}

	paramData, err := params.Encode()
	if err != nil {
		c.deleteRequest(reqId)
		return 0, fmt.Errorf("cannot encode parameters: %w", err)
	}

	paramReader := bytes.NewReader(paramData)
	if err := c.writeStream(RecordTypeParams, reqId, paramReader); err != nil {
		c.deleteRequest(reqId)
		return 0, fmt.Errorf("cannot write parameter stream: %w", err)
	}

	// 6.1 "When a role protocol calls for transmitting a stream other than
	// FCGI_STDERR, at least one record of the stream type is always
	// transmitted, even if the stream is empty."

	if (role == RoleResponder || role == RoleFilter) && stdin == nil {
		stdin = bytes.NewReader([]byte(nil))
	}

	if stdin != nil {
		if err := c.writeStream(RecordTypeStdin, reqId, stdin); err != nil {
			c.deleteRequest(reqId)
			return 0, fmt.Errorf("cannot write stdin stream: %w", err)
		}
	}

	if role == RoleFilter && data == nil {
		data = bytes.NewReader([]byte(nil))
	}

	if data != nil {
		if err := c.writeStream(RecordTypeData, reqId, data); err != nil {
			c.deleteRequest(reqId)
			return 0, fmt.Errorf("cannot write data stream: %w", err)
		}
	}

	res := <-resChan
	if res.Error != nil {
		return 0, res.Error
	}

	switch res.ProtocolStatus {
	case ProtocolStatusCannotMultiplexConnection:
		// It is not clear if having a separate error type for this error is
		// useful. Ultimately it means that the server cannot handle this
		// connection, i.e. it is overloaded.
		return 0, ErrServerOverloaded
	case ProtocolStatusOverloaded:
		return 0, ErrServerOverloaded
	case ProtocolStatusUnknownRole:
		return 0, fmt.Errorf("unknown role %q", role)
	}

	return res.AppStatus, nil
}

func (c *Client) createRequest(stdout, stderr io.Writer) (uint16, chan *Response, bool) {
	c.requestMutex.Lock()
	defer c.requestMutex.Unlock()

	for id, req := range c.requests {
		if req == nil {
			resChan := make(chan *Response, 1)

			c.requests[id] = &Request{
				Id: uint16(id),

				Stdout: stdout,
				Stderr: stderr,

				ResponseChan: resChan,
			}

			return uint16(id), resChan, true
		}
	}

	return 0, nil, false
}

func (c *Client) findRequest(id uint16) *Request {
	c.requestMutex.Lock()
	req := c.requests[id]
	c.requestMutex.Unlock()

	return req
}

func (c *Client) abortAndDeleteRequest(req *Request, err error) error {
	if err := c.writeRecord(RecordTypeAbortRequest, 0, nil); err != nil {
		return fmt.Errorf("cannot write %q record: %w",
			RecordTypeAbortRequest, err)
	}

	res := Response{Error: err}
	req.ResponseChan <- &res

	c.deleteRequest(req.Id)

	return nil
}

func (c *Client) deleteRequest(id uint16) {
	c.requestMutex.Lock()
	c.requests[id] = nil
	c.requestMutex.Unlock()
}

func (c *Client) deleteRequests() {
	c.requestMutex.Lock()
	defer c.requestMutex.Unlock()

	for _, req := range c.requests {
		if req != nil {
			res := Response{Error: ErrClientShutdown}
			req.ResponseChan <- &res
		}
	}

	c.requests = nil
}

func (c *Client) reconnect() error {
	c.Close()

	conn, err := net.Dial("tcp", c.Cfg.Address)
	if err != nil {
		err = netutils.UnwrapOpError(err, "dial")
		return fmt.Errorf("cannot connect to %q: %v", c.Cfg.Address, err)
	}

	c.conn = conn

	if err := c.fetchValues(); err != nil {
		conn.Close()
		c.conn = nil
		return fmt.Errorf("cannot write initial %q request: %w",
			RecordTypeGetValues, err)
	}

	c.requests = make([]*Request, c.maxConcurrentRequests)

	c.readerWg.Add(1)
	go c.read()

	return nil
}

func (c *Client) fetchValues() error {
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

	err := c.processRecordGetValuesResult(&r, r.Body.(*GetValuesResultBody))
	if err != nil {
		return fmt.Errorf("cannot process record: %w", err)
	}

	return nil
}

func (c *Client) read() {
	defer c.readerWg.Done()

	for {
		var r Record
		if err := r.Read(c.conn); err != nil {
			if errors.Is(err, io.EOF) || errors.Is(err, net.ErrClosed) {
				return
			}

			c.Log.Error("cannot read record: %v", err)
			c.conn.Close()
			return
		}

		if err := c.processRecord(&r); err != nil {
			c.Log.Error("cannot process %q record: %v", r.RecordType, err)
			c.conn.Close()
			return
		}
	}
}

func (c *Client) processRecord(r *Record) error {
	var err error

	switch r.RecordType {
	case RecordTypeGetValuesResult:
		err = c.processRecordGetValuesResult(r, r.Body.(*GetValuesResultBody))

	case RecordTypeUnknownType:
		err = c.processRecordUnknownType(r, r.Body.(*UnknownTypeBody))

	case RecordTypeEndRequest:
		err = c.processRecordEndRequest(r, r.Body.(*EndRequestBody))

	case RecordTypeStdout:
		err = c.processRecordStdout(r, r.Body.([]byte))

	case RecordTypeStderr:
		err = c.processRecordStderr(r, r.Body.([]byte))

	default:
		err = fmt.Errorf("unhandled record type %q", r.RecordType)
	}

	return err
}

func (c *Client) processRecordGetValuesResult(r *Record, body *GetValuesResultBody) error {
	parseUInt := func(p NameValuePair, max int64) (int, error) {
		i, err := strconv.ParseInt(p.Value, 10, 64)
		if err != nil {
			return 0, fmt.Errorf("invalid integer %q for value %q",
				p.Value, p.Name)
		} else if i < 0 {
			return 0, fmt.Errorf("invalid negative integer %d for value %q",
				i, p.Name)
		} else if i > max {
			return 0, fmt.Errorf("integer %d too large for value %q "+
				"(must be lower or equal to %d)",
				i, p.Name, max)
		}

		return int(i), nil
	}

	for _, pair := range body.Pairs {
		switch pair.Name {
		case "FCGI_MAX_CONNS":
			i, err := parseUInt(pair, math.MaxUint32)
			if err != nil {
				return err
			}

			c.maxConcurrentConnections = i

		case "FCGI_MAX_REQS":
			i, err := parseUInt(pair, math.MaxUint16)
			if err != nil {
				return err
			}

			c.maxConcurrentRequests = i

		case "FCGI_MPXS_CONNS":
			i, err := parseUInt(pair, math.MaxUint32)
			if err != nil {
				return err
			} else if i > 1 {
				return fmt.Errorf("invalid value %d for value %q",
					i, "FCGI_MPXS_CONNS")
			}

			c.multiplexConnections = i > 0

		default:
			c.Log.Debug(1, "ignoring unknown value %q", pair.Name)
		}
	}

	c.valueMutex.Lock()
	c.values = body.Pairs
	c.valueMutex.Unlock()

	return nil
}

func (c *Client) processRecordUnknownType(r *Record, body *UnknownTypeBody) error {
	return fmt.Errorf("server does not support record type %d", body.Type)
}

func (c *Client) processRecordEndRequest(r *Record, body *EndRequestBody) error {
	req := c.findRequest(r.RequestId)
	if req == nil {
		return nil
	}

	res := Response{
		AppStatus:      body.AppStatus,
		ProtocolStatus: body.ProtocolStatus,
	}

	req.ResponseChan <- &res

	c.deleteRequest(r.RequestId)

	return nil
}

func (c *Client) processRecordStdout(r *Record, body []byte) error {
	req := c.findRequest(r.RequestId)
	if req == nil {
		return nil
	}

	if _, err := req.Stdout.Write(body); err != nil {
		reqErr := fmt.Errorf("cannot write stdout: %v", err)

		if err := c.abortAndDeleteRequest(req, reqErr); err != nil {
			return err
		}

		return nil
	}

	return nil
}

func (c *Client) processRecordStderr(r *Record, body []byte) error {
	req := c.findRequest(r.RequestId)
	if req == nil {
		return nil
	}

	if _, err := req.Stderr.Write(body); err != nil {
		reqErr := fmt.Errorf("cannot write stderr: %v", err)

		if err := c.abortAndDeleteRequest(req, reqErr); err != nil {
			return err
		}

		return nil
	}

	return nil
}

func (c *Client) writeRecord(rtype RecordType, id uint16, body any) error {
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

func (c *Client) writeStream(rtype RecordType, id uint16, stream io.Reader) error {
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
