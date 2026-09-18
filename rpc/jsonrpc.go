package rpc

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"testing"
)

// Request is a JSON-RPC 2.0 request object.
type Request struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      any             `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

// Response is a JSON-RPC 2.0 response object.
type Response struct {
	JSONRPC string `json:"jsonrpc"`
	ID      any    `json:"id,omitempty"`
	Result  any    `json:"result,omitempty"`
	Error   *Error `json:"error,omitempty"`
}

// Error represents a JSON-RPC error object.
type Error struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func (e *Error) Error() string { return e.Message }

// Client is a JSON-RPC 2.0 test client connected to an in-memory pipe.
type Client struct {
	t      testing.TB
	conn   net.Conn
	enc    *json.Encoder
	dec    *json.Decoder
	cancel context.CancelFunc
	done   chan struct{}
}

// DialJSONRPC creates a client and runs serve in a goroutine over net.Pipe.
func DialJSONRPC(t testing.TB, serve func(ctx context.Context, in io.Reader, out io.Writer) error) *Client {
	t.Helper()

	serverConn, clientConn := net.Pipe()
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})

	go func() {
		defer close(done)
		defer serverConn.Close()

		err := serve(ctx, serverConn, serverConn)
		if err != nil && !errors.Is(err, io.EOF) && !errors.Is(err, context.Canceled) {
			t.Errorf("rpc: server error: %v", err)
		}
	}()

	c := &Client{
		t:      t,
		conn:   clientConn,
		enc:    json.NewEncoder(clientConn),
		dec:    json.NewDecoder(clientConn),
		cancel: cancel,
		done:   done,
	}

	t.Cleanup(func() {
		cancel()
		_ = clientConn.Close()
		<-done
	})

	return c
}

// Call sends a JSON-RPC request and waits for the matching response.
func (c *Client) Call(method string, params, id any) Response {
	c.t.Helper()

	raw, err := marshalParams(params)
	if err != nil {
		c.t.Fatalf("rpc: marshal params failed: %v", err)
	}

	req := Request{
		JSONRPC: "2.0",
		ID:      id,
		Method:  method,
		Params:  raw,
	}

	if err := c.enc.Encode(req); err != nil {
		c.t.Fatalf("rpc: write request failed: %v", err)
	}

	var resp Response
	if err := c.dec.Decode(&resp); err != nil {
		c.t.Fatalf("rpc: read response failed: %v", err)
	}

	return resp
}

// CallRaw sends a raw payload line and reads a response.
func (c *Client) CallRaw(payload string) Response {
	c.t.Helper()

	if _, err := io.WriteString(c.conn, payload+"\n"); err != nil {
		c.t.Fatalf("rpc: write raw request failed: %v", err)
	}

	var resp Response
	if err := c.dec.Decode(&resp); err != nil {
		c.t.Fatalf("rpc: read response failed: %v", err)
	}

	return resp
}

// Notify sends a JSON-RPC notification (no ID, no response).
func (c *Client) Notify(method string, params any) error {
	raw, err := marshalParams(params)
	if err != nil {
		return err
	}

	req := Request{
		JSONRPC: "2.0",
		Method:  method,
		Params:  raw,
	}

	return c.enc.Encode(req)
}

// Close closes the client connection.
func (c *Client) Close() error {
	c.cancel()
	err := c.conn.Close()
	<-c.done
	return err
}

// BindResult unmarshals a successful result into v.
func (r *Response) BindResult(t testing.TB, v any) {
	t.Helper()

	if r.Error != nil {
		t.Fatalf("rpc: response contains error: %+v", r.Error)
	}
	if r.Result == nil {
		t.Fatalf("rpc: response result is nil")
	}

	data, err := json.Marshal(r.Result)
	if err != nil {
		t.Fatalf("rpc: marshal result failed: %v", err)
	}
	if err := json.Unmarshal(data, v); err != nil {
		t.Fatalf("rpc: unmarshal result into %T failed: %v", v, err)
	}
}

func marshalParams(params any) (json.RawMessage, error) {
	if params == nil {
		return nil, nil
	}

	data, err := json.Marshal(params)
	if err != nil {
		return nil, err
	}

	return json.RawMessage(data), nil
}
