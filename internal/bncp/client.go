package bncp

import (
	"context"
	"errors"
	"net"
	"sync"
	"time"
)

var ErrUnauthorized = errors.New("jeton d'agent invalide")

type Direct struct {
	From    string
	Payload string
}

type Client struct {
	Addr     string
	Dial     func(context.Context, string, string) (net.Conn, error)
	OnDirect func(Direct)

	mu      sync.Mutex
	conn    net.Conn
	acks    chan Frame
	waiters int
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
	_ = c.conn.SetWriteDeadline(time.Now().Add(30 * time.Second))
	if err := Write(c.conn, Frame{Type: typ, Body: body}); err != nil {
		c.close()
		return err
	}
	timer := time.NewTimer(30 * time.Second)
	defer timer.Stop()
	select {
	case frame, ok := <-c.acks:
		if !ok {
			c.close()
			return errors.New("connexion BNCP coupée")
		}
		return c.finish(frame, dest)
	case <-timer.C:
		c.close()
		return context.DeadlineExceeded
	case <-ctx.Done():
		c.close()
		return ctx.Err()
	}
}

func (c *Client) finish(frame Frame, dest any) error {
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
	c.acks = make(chan Frame, 4)
	go c.read(conn, c.acks)
	return nil
}

func (c *Client) read(conn net.Conn, acks chan Frame) {
	defer close(acks)
	for {
		frame, err := Read(conn)
		if err != nil {
			return
		}
		if frame.Type == TypeDirect {
			var msg struct {
				From    string `json:"from"`
				Payload string `json:"payload"`
			}
			if Unmarshal(frame.Body, &msg) == nil && c.OnDirect != nil {
				c.OnDirect(Direct{From: msg.From, Payload: msg.Payload})
			}
			continue
		}
		select {
		case acks <- frame:
		default:
			return
		}
	}
}

func (c *Client) close() error {
	if c.conn == nil {
		return nil
	}
	err := c.conn.Close()
	c.conn = nil
	return err
}
