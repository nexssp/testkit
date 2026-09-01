package rpc_test

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"io"
	"testing"

	"github.com/nexssp/testkit/rpc"
)

func serveEcho(ctx context.Context, in io.Reader, out io.Writer) error {
	dec := json.NewDecoder(in)
	enc := json.NewEncoder(out)

	for {
		var req rpc.Request
		if err := dec.Decode(&req); err != nil {
			if errors.Is(err, io.EOF) {
				return nil
			}
			return err
		}

		if req.ID == nil {
			continue // notification
		}

		if err := enc.Encode(rpc.Response{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result:  map[string]any{"method": req.Method},
		}); err != nil {
			return err
		}
	}
}

func TestDialJSONRPC_Call(t *testing.T) {
	client := rpc.DialJSONRPC(t, serveEcho)

	resp := client.Call("tools/list", nil, 1)
	if resp.Error != nil {
		t.Fatalf("unexpected error: %+v", resp.Error)
	}

	var result map[string]any
	resp.BindResult(t, &result)
	if result["method"] != "tools/list" {
		t.Fatalf("unexpected result: %+v", result)
	}
}

func TestDialJSONRPC_Notify(t *testing.T) {
	client := rpc.DialJSONRPC(t, serveEcho)
	if err := client.Notify("notifications/initialized", nil); err != nil {
		t.Fatalf("notify failed: %v", err)
	}
}

func TestDialJSONRPC_Error(t *testing.T) {
	server := func(ctx context.Context, in io.Reader, out io.Writer) error {
		dec := json.NewDecoder(in)
		enc := json.NewEncoder(out)

		var req rpc.Request
		if err := dec.Decode(&req); err != nil {
			return err
		}

		return enc.Encode(rpc.Response{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error:   &rpc.RPCError{Code: -32601, Message: "method not found"},
		})
	}

	client := rpc.DialJSONRPC(t, server)

	resp := client.Call("nope", nil, 2)
	if resp.Error == nil || resp.Error.Code != -32601 {
		t.Fatalf("expected -32601 error, got %+v", resp.Error)
	}
}

func TestDialJSONRPC_CallRaw(t *testing.T) {
	server := func(ctx context.Context, in io.Reader, out io.Writer) error {
		br := bufio.NewReader(in)
		if _, err := br.ReadBytes('\n'); err != nil {
			return err
		}

		enc := json.NewEncoder(out)
		return enc.Encode(rpc.Response{
			JSONRPC: "2.0",
			Error:   &rpc.RPCError{Code: -32700, Message: "Parse error"},
		})
	}

	client := rpc.DialJSONRPC(t, server)

	resp := client.CallRaw("{invalid json}")
	if resp.Error == nil || resp.Error.Code != -32700 {
		t.Fatalf("expected parse error, got %+v", resp.Error)
	}
}
