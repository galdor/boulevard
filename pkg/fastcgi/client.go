package fastcgi

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"slices"
	"sync"
	"time"

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

	valuesListeners     []chan NameValuePairs
	valuesListenerMutex sync.Mutex
}

func NewClient(cfg *ClientCfg) (*Client, error) {
	c := Client{
		Cfg: cfg,
		Log: cfg.Log,
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

func (c *Client) FetchValues(names []string) (NameValuePairs, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	return c.FetchValuesWithContext(ctx, names)
}

func (c *Client) FetchValuesWithContext(ctx context.Context, names []string) (NameValuePairs, error) {
	body := GetValuesBody{
		Names: names,
	}

	record := Record{
		Version:    1,
		RecordType: RecordTypeGetValues,
		RequestId:  0,
		Body:       &body,
	}

	listener := c.createValuesListener()

	if err := c.writeRecord(&record); err != nil {
		c.deleteValuesListener(listener)
		return nil, fmt.Errorf("cannot write record: %w", err)
	}

	select {
	case pairs := <-listener:
		return pairs, nil

	case <-ctx.Done():
		c.deleteValuesListener(listener)
		return nil, ctx.Err()
	}
}

func (c *Client) reconnect() error {
	c.Close()

	conn, err := net.Dial("tcp", c.Cfg.Address)
	if err != nil {
		err = netutils.UnwrapOpError(err, "dial")
		return fmt.Errorf("cannot connect to %q: %v", c.Cfg.Address, err)
	}

	c.conn = conn

	c.readerWg.Add(1)
	go c.read()

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
		// TODO

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
	c.notifyValuesListeners(body.Pairs)
	return nil
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

func (c *Client) createValuesListener() chan NameValuePairs {
	listener := make(chan NameValuePairs, 1)

	c.valuesListenerMutex.Lock()
	c.valuesListeners = append(c.valuesListeners, listener)
	c.valuesListenerMutex.Unlock()

	return listener
}

func (c *Client) deleteValuesListener(listener chan NameValuePairs) {
	c.valuesListenerMutex.Lock()
	c.valuesListeners = slices.DeleteFunc(c.valuesListeners,
		func(l chan NameValuePairs) bool {
			return l == listener
		})
	c.valuesListenerMutex.Unlock()

	close(listener)
}

func (c *Client) notifyValuesListeners(pairs NameValuePairs) {
	c.valuesListenerMutex.Lock()
	defer c.valuesListenerMutex.Unlock()

	for _, listener := range c.valuesListeners {
		listener <- pairs
		close(listener)
	}

	c.valuesListeners = nil
}
