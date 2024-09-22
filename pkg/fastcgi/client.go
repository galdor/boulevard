package fastcgi

import (
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

	record := Record{
		Version:    1,
		RecordType: RecordTypeGetValues,
		RequestId:  0,
		Body:       &body,
	}

	if err := c.writeRecord(&record); err != nil {
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
			if errors.Is(err, io.EOF) {
				return
			}

			c.Log.Error("cannot read record: %v", err)
			c.conn.Close()
			return
		}

		if err := c.processRecord(&r); err != nil {
			c.Log.Error("cannot process record: %v", err)
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
		// TODO

	case RecordTypeStdout:
		// TODO

	case RecordTypeStderr:
		// TODO

	default:
		err = fmt.Errorf("unhandled record type %q", r.RecordType)
	}

	return err
}

func (c *Client) processRecordGetValuesResult(r *Record, body *GetValuesResultBody) error {
	parseUInt := func(p NameValuePair) (int, error) {
		i, err := strconv.ParseInt(p.Value, 10, 64)
		if err != nil {
			return 0, fmt.Errorf("invalid integer %q for value %q",
				p.Value, p.Name)
		} else if i < 0 {
			return 0, fmt.Errorf("invalid negative integer %d for value %q",
				i, p.Name)
		} else if i > math.MaxInt {
			return 0, fmt.Errorf("integer %d too large for value %q "+
				"(must be lower or equal to %d)",
				i, p.Name, math.MaxInt)
		}

		return int(i), nil
	}

	for _, pair := range body.Pairs {
		switch pair.Name {
		case "FCGI_MAX_CONNS":
			i, err := parseUInt(pair)
			if err != nil {
				return err
			}

			c.maxConcurrentConnections = i

		case "FCGI_MAX_REQS":
			i, err := parseUInt(pair)
			if err != nil {
				return err
			}

			c.maxConcurrentRequests = i

		case "FCGI_MPXS_CONNS":
			i, err := parseUInt(pair)
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

func (c *Client) writeRecord(r *Record) error {
	if c.conn == nil {
		if err := c.reconnect(); err != nil {
			return err
		}
	}

	if err := r.Write(c.conn); err != nil {
		err = netutils.UnwrapOpError(err, "write")
		return err
	}

	return nil
}
