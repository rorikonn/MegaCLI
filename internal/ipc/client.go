package ipc

import (
	"fmt"
	"net"
	"net/rpc"
	"net/rpc/jsonrpc"
	"time"
)

// Client connects to a remote MegaCli instance via JSON-RPC/TCP.
type Client struct {
	addr string
}

// NewClient creates a client targeting the given host:port.
func NewClient(host string, port int) *Client {
	return &Client{
		addr: fmt.Sprintf("%s:%d", host, port),
	}
}

// NewClientFromInstance creates a client from a discovered InstanceInfo.
func NewClientFromInstance(info InstanceInfo) *Client {
	return NewClient("127.0.0.1", info.Port)
}

func (c *Client) dial() (*rpc.Client, error) {
	conn, err := net.DialTimeout("tcp", c.addr, 5*time.Second)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to %s: %w", c.addr, err)
	}
	return jsonrpc.NewClient(conn), nil
}

// Discover queries the remote instance for its agents.
func (c *Client) Discover() (*DiscoverResponse, error) {
	client, err := c.dial()
	if err != nil {
		return nil, err
	}
	defer client.Close()

	var reply DiscoverResponse
	if err := client.Call(MethodDiscover, &DiscoverRequest{}, &reply); err != nil {
		return nil, err
	}
	return &reply, nil
}

// Delegate sends a task to a remote agent and waits for completion.
func (c *Client) Delegate(req DelegateRequest) (*DelegateResponse, error) {
	client, err := c.dial()
	if err != nil {
		return nil, err
	}
	defer client.Close()

	var reply DelegateResponse
	if err := client.Call(MethodDelegate, &req, &reply); err != nil {
		return nil, err
	}
	return &reply, nil
}

// QueryStatus queries the remote instance's current state.
func (c *Client) QueryStatus() (*StatusResponse, error) {
	client, err := c.dial()
	if err != nil {
		return nil, err
	}
	defer client.Close()

	var reply StatusResponse
	if err := client.Call(MethodQueryStatus, &StatusRequest{}, &reply); err != nil {
		return nil, err
	}
	return &reply, nil
}
