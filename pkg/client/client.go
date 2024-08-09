package client

import (
	"encoding/gob"
	"net"
	"parkerdgabel/sockd/pkg/container"
	"parkerdgabel/sockd/pkg/message"
)

type Client struct {
	conn    net.Conn
	encoder *gob.Encoder
	decoder *gob.Decoder
}

type Option func(*Client)

func NewClient(opts ...Option) *Client {
	c := &Client{}
	for _, opt := range opts {
		opt(c)
	}
	c.encoder = gob.NewEncoder(c.conn)
	c.decoder = gob.NewDecoder(c.conn)
	return c
}

func WithConn(conn net.Conn) Option {
	return func(c *Client) {
		c.conn = conn
	}
}

func (c *Client) Close() error {
	return c.conn.Close()
}

func (c *Client) Send(msg *message.Request) error {
	return c.encoder.Encode(msg)
}

func (c *Client) Receive(msg *message.Response) error {
	return c.decoder.Decode(msg)
}

func (c *Client) SendReceive(req *message.Request, res *message.Response) error {
	if err := c.Send(req); err != nil {
		return err
	}
	return c.Receive(res)
}

// Command methods
func (c *Client) Create(meta container.Meta, name string) (*message.Response, error) {
	req := &message.Request{
		Command: message.CommandCreate,
		Payload: message.PayloadCreate{Meta: meta, Name: name},
	}
	res := &message.Response{}
	err := c.SendReceive(req, res)
	return res, err
}

func (c *Client) Delete(id string) (*message.Response, error) {
	req := &message.Request{
		Command: message.CommandDelete,
		Payload: message.PayloadDelete{Id: id},
	}
	res := &message.Response{}
	err := c.SendReceive(req, res)
	return res, err
}

func (c *Client) Start(id string) (*message.Response, error) {
	req := &message.Request{
		Command: message.CommandStart,
		Payload: message.PayloadStart{Id: id},
	}
	res := &message.Response{}
	err := c.SendReceive(req, res)
	return res, err
}

func (c *Client) Stop(id string) (*message.Response, error) {
	req := &message.Request{
		Command: message.CommandStop,
		Payload: message.PayloadStop{Id: id},
	}
	res := &message.Response{}
	err := c.SendReceive(req, res)
	return res, err
}

func (c *Client) List() (*message.Response, error) {
	req := &message.Request{
		Command: message.CommandList,
		Payload: message.PayloadList{},
	}
	res := &message.Response{}
	err := c.SendReceive(req, res)
	return res, err
}

func (c *Client) Inspect(id string) (*message.Response, error) {
	req := &message.Request{
		Command: message.CommandInspect,
		Payload: message.PayloadInspect{Id: id},
	}
	res := &message.Response{}
	err := c.SendReceive(req, res)
	return res, err
}

func (c *Client) Logs(id string) (*message.Response, error) {
	req := &message.Request{
		Command: message.CommandLogs,
		Payload: message.PayloadLogs{Id: id},
	}
	res := &message.Response{}
	err := c.SendReceive(req, res)
	return res, err
}

func (c *Client) Fork(id string) (*message.Response, error) {
	req := &message.Request{
		Command: message.CommandFork,
		Payload: message.PayloadFork{Id: id},
	}
	res := &message.Response{}
	err := c.SendReceive(req, res)
	return res, err
}

func (c *Client) Pause(id string) (*message.Response, error) {
	req := &message.Request{
		Command: message.CommandPause,
		Payload: message.PayloadPause{Id: id},
	}
	res := &message.Response{}
	err := c.SendReceive(req, res)
	return res, err
}

func (c *Client) Unpause(id string) (*message.Response, error) {
	req := &message.Request{
		Command: message.CommandUnpause,
		Payload: message.PayloadUnpause{Id: id},
	}
	res := &message.Response{}
	err := c.SendReceive(req, res)
	return res, err
}
