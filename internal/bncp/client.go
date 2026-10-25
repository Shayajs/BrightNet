package bncp

import (
	"context"
	"errors"
	"net"
	"sync"
	"time"
)

var ErrUnauthorized = errors.New("jeton d'agent invalide")

type Client struct {
	Addr string
	Dial func(context.Context, string, string) (net.Conn, error)
	mu   sync.Mutex
	conn net.Conn
}

func (c *Client) RoundTrip(ctx context.Context, typ uint8, payload any, dest any) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	body, err := Marshal(payload)
	if err != nil {
		return err
	}
	if err := c.ensure(ctx); err != nil {
		return err
	}
	_ = c.conn.SetDeadline(time.Now().Add(30 * time.Second))
	if err := Write(c.conn, Frame{Type: typ, Body: body}); err != nil {
		c.close()
		return err
	}
	frame, err := Read(c.conn)
	if err != nil {
		c.close()
		return err
	}
	if frame.Type == TypeError {
		var msg struct {
			Error string `json:"error"`
		}
		_ = Unmarshal(frame.Body, &msg)
		c.close()
		if msg.Error == ErrUnauthorized.Error() {
			return ErrUnauthorized
		}
		if msg.Error == "" {
			msg.Error = "échange BNCP refusé"
		}
		return errors.New(msg.Error)
	}
	if dest != nil {
		if err := Unmarshal(frame.Body, dest); err != nil {
			c.close()
			return err
		}
	}
	return nil
}

func (c *Client) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.close()
}

func (c *Client) ensure(ctx context.Context) error {
	if c.conn != nil {
		return nil
	}
	dial := c.Dial
	if dial == nil {
		dial = (&net.Dialer{Timeout: 5 * time.Second}).DialContext
	}
	conn, err := dial(ctx, "tcp", c.Addr)
	if err != nil {
		return err
	}
	c.conn = conn
	return nil
}

func (c *Client) close() error {
	if c.conn == nil {
		return nil
	}
	err := c.conn.Close()
	c.conn = nil
	return err
}
