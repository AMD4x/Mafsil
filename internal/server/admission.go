package server

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/modelcontextprotocol/go-sdk/jsonrpc"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const MaxMessageBytes = 8 << 20

type admission struct {
	mu            sync.Mutex
	pending       map[string]bool
	tokens, bytes float64
	last          time.Time
	handlers      chan struct{}
}

func newAdmission() *admission {
	return &admission{pending: map[string]bool{}, tokens: 128, bytes: 16 << 20, last: time.Now(), handlers: make(chan struct{}, 16)}
}
func (a *admission) release(id string) {
	if id == "" {
		return
	}
	a.mu.Lock()
	delete(a.pending, id)
	a.mu.Unlock()
}
func (a *admission) middleware(next mcp.MethodHandler) mcp.MethodHandler {
	return func(ctx context.Context, method string, req mcp.Request) (mcp.Result, error) {
		if method == "tools/call" {
			select {
			case a.handlers <- struct{}{}:
				defer func() { <-a.handlers }()
			default:
				return nil, &jsonrpc.Error{Code: -32000, Message: "too many active tool calls"}
			}
		}
		return next(ctx, method, req)
	}
}

type boundedTransport struct {
	base   mcp.Transport
	budget *admission
}

func (t *boundedTransport) Connect(ctx context.Context) (mcp.Connection, error) {
	c, e := t.base.Connect(ctx)
	if e != nil {
		return nil, e
	}
	return &boundedConnection{Connection: c, budget: t.budget}, nil
}

type boundedConnection struct {
	mcp.Connection
	budget *admission
}

func (c *boundedConnection) Read(ctx context.Context) (_ jsonrpc.Message, err error) {
	defer func() {
		if err != nil {
			// Unblock pending writes as well as reads when input is rejected.
			_ = c.Connection.Close()
		}
	}()
	msg, e := c.Connection.Read(ctx)
	if e != nil {
		return nil, e
	}
	a := c.budget
	a.mu.Lock()
	defer a.mu.Unlock()
	now := time.Now()
	elapsed := now.Sub(a.last).Seconds()
	a.last = now
	a.tokens = min(128, a.tokens+elapsed*64)
	a.bytes = min(16<<20, a.bytes+elapsed*(8<<20))
	r, ok := msg.(*jsonrpc.Request)
	if !ok {
		return nil, errors.New("unsolicited client response")
	}
	a.tokens--
	a.bytes -= float64(len(r.Params))
	if a.tokens < 0 || a.bytes < 0 {
		return nil, errors.New("MCP input rate limit exceeded")
	}
	if r.IsCall() {
		id := requestKey(r.ID)
		if len(id) > 160 {
			return nil, errors.New("request ID is too long")
		}
		if a.pending[id] || len(a.pending) >= 16 {
			return nil, errors.New("duplicate request ID or more than 16 pending requests")
		}
		a.pending[id] = true
	}
	return msg, nil
}
func (c *boundedConnection) Write(ctx context.Context, msg jsonrpc.Message) error {
	e := c.Connection.Write(ctx, msg)
	// Keep admission reserved through the transport write. A client that stops
	// reading must not create an unbounded queue of completed responses.
	if r, ok := msg.(*jsonrpc.Response); ok {
		c.budget.release(requestKey(r.ID))
	}
	return e
}
func requestKey(id jsonrpc.ID) string { return fmt.Sprintf("%T:%v", id.Raw(), id.Raw()) }
