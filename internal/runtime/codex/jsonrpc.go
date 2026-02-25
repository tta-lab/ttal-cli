package codex

import (
	"encoding/json"
	"fmt"
	"sync"
	"sync/atomic"

	"github.com/gorilla/websocket"
)

type rpcRequest struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      int64       `json:"id"`
	Method  string      `json:"method"`
	Params  interface{} `json:"params,omitempty"`
}

type rpcResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      int64           `json:"id,omitempty"`
	Method  string          `json:"method,omitempty"` // for notifications
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *rpcError       `json:"error,omitempty"`
	Params  json.RawMessage `json:"params,omitempty"` // for notifications
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// Client is a JSON-RPC 2.0 client over WebSocket.
type Client struct {
	conn    *websocket.Conn
	nextID  atomic.Int64
	pending map[int64]chan rpcResponse
	notify  chan rpcResponse
	mu      sync.Mutex
	done    chan struct{}
}

// NewClient dials a WebSocket and starts reading.
func NewClient(url string) (*Client, error) {
	conn, _, err := websocket.DefaultDialer.Dial(url, nil)
	if err != nil {
		return nil, fmt.Errorf("dial codex: %w", err)
	}

	c := &Client{
		conn:    conn,
		pending: make(map[int64]chan rpcResponse),
		notify:  make(chan rpcResponse, 64),
		done:    make(chan struct{}),
	}
	go c.readLoop()
	return c, nil
}

// Call sends a JSON-RPC request and waits for the response.
func (c *Client) Call(method string, params interface{}) (json.RawMessage, error) {
	id := c.nextID.Add(1)
	req := rpcRequest{JSONRPC: "2.0", ID: id, Method: method, Params: params}

	ch := make(chan rpcResponse, 1)
	c.mu.Lock()
	c.pending[id] = ch
	c.mu.Unlock()

	data, _ := json.Marshal(req)
	if err := c.conn.WriteMessage(websocket.TextMessage, data); err != nil {
		c.mu.Lock()
		delete(c.pending, id)
		c.mu.Unlock()
		return nil, err
	}

	select {
	case resp, ok := <-ch:
		if !ok {
			return nil, fmt.Errorf("client closed")
		}
		if resp.Error != nil {
			return nil, fmt.Errorf("RPC error %d: %s", resp.Error.Code, resp.Error.Message)
		}
		return resp.Result, nil
	case <-c.done:
		c.mu.Lock()
		delete(c.pending, id)
		c.mu.Unlock()
		return nil, fmt.Errorf("client closed")
	}
}

// Notifications returns the channel for server-push notifications.
func (c *Client) Notifications() <-chan rpcResponse { return c.notify }

// Close shuts down the WebSocket connection.
func (c *Client) Close() error {
	select {
	case <-c.done:
	default:
		close(c.done)
	}
	return c.conn.Close()
}

func (c *Client) readLoop() {
	defer func() {
		select {
		case <-c.done:
		default:
			close(c.done)
		}
		// Unblock any callers waiting on pending responses.
		c.mu.Lock()
		for id, ch := range c.pending {
			close(ch)
			delete(c.pending, id)
		}
		c.mu.Unlock()
		close(c.notify)
	}()

	for {
		_, data, err := c.conn.ReadMessage()
		if err != nil {
			return
		}

		var resp rpcResponse
		if err := json.Unmarshal(data, &resp); err != nil {
			continue
		}

		if resp.ID != 0 {
			c.mu.Lock()
			ch, ok := c.pending[resp.ID]
			if ok {
				delete(c.pending, resp.ID)
			}
			c.mu.Unlock()
			if ok {
				ch <- resp
			}
		} else if resp.Method != "" {
			select {
			case c.notify <- resp:
			default:
				// drop if buffer full
			}
		}
	}
}
