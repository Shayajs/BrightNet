package bncp

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net"
	"sync"
	"time"

	"brightnet/internal/contract"
	"brightnet/internal/hub"
	"brightnet/internal/store"
)

type Server struct {
	Store *store.Store
	Hub   *hub.Hub
	limit limiter
}

func NewServer(st *store.Store, h *hub.Hub) *Server {
	return &Server{Store: st, Hub: h, limit: limiter{hits: map[string][]time.Time{}}}
}

func (s *Server) Serve(ctx context.Context, ln net.Listener) error {
	go func() {
		<-ctx.Done()
		_ = ln.Close()
	}()
	for {
		conn, err := ln.Accept()
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			return err
		}
		go s.handle(ctx, conn)
	}
}

func (s *Server) handle(ctx context.Context, conn net.Conn) {
	defer conn.Close()
	for {
		_ = conn.SetReadDeadline(time.Now().Add(2 * time.Minute))
		frame, err := Read(conn)
		if err != nil {
			if !errors.Is(err, io.EOF) && !errors.Is(err, ErrShort) && ctx.Err() == nil {
				slog.Debug("trame BNCP ignorée", "err", err)
			}
			return
		}
		ack, body, err := s.dispatch(ctx, conn.RemoteAddr().String(), frame)
		if err != nil {
			_ = writeValue(conn, TypeError, map[string]string{"error": err.Error()})
			return
		}
		if err := writeValue(conn, ack, body); err != nil {
			return
		}
	}
}

func (s *Server) dispatch(ctx context.Context, remote string, frame Frame) (uint8, any, error) {
	switch frame.Type {
	case TypeEnroll:
		if !s.limit.allow(remote, 8, time.Minute) {
			return 0, nil, errors.New("trop de tentatives")
		}
		var req contract.EnrollRequest
		if err := Unmarshal(frame.Body, &req); err != nil || req.PairingCode == "" {
			return 0, nil, errors.New("code d'appairage requis")
		}
		agentID, token, err := s.Store.Enroll(ctx, req)
		if err != nil {
			return 0, nil, errors.New("code d'appairage invalide")
		}
		return TypeEnrollAck, contract.NodeAck(agentID, token), nil
	case TypeHeartbeat:
		id, err := s.auth(ctx, frame.Body)
		if err != nil {
			return 0, nil, err
		}
		if err := s.Store.TouchAgent(ctx, id); err != nil {
			return 0, nil, errors.New("battement refusé")
		}
		return TypeHeartbeatAck, contract.NodeAck("", ""), nil
	case TypeSnapshot:
		id, err := s.auth(ctx, frame.Body)
		if err != nil {
			return 0, nil, err
		}
		var msg struct {
			Snapshot contract.Snapshot `json:"snapshot"`
		}
		if err := Unmarshal(frame.Body, &msg); err != nil {
			return 0, nil, errors.New("instantané refusé")
		}
		result, err := s.Store.ApplySnapshot(ctx, id, msg.Snapshot)
		if err != nil {
			return 0, nil, errors.New("instantané refusé")
		}
		s.publish(result)
		return TypeSnapshotAck, contract.NodeAck("", ""), nil
	case TypeEvents:
		id, err := s.auth(ctx, frame.Body)
		if err != nil {
			return 0, nil, err
		}
		var msg struct {
			Events []contract.DockerEvent `json:"events"`
		}
		if err := Unmarshal(frame.Body, &msg); err != nil || len(msg.Events) > 100 {
			return 0, nil, errors.New("événements refusés")
		}
		result, err := s.Store.ApplyDockerEvents(ctx, id, msg.Events)
		if err != nil {
			return 0, nil, errors.New("événements refusés")
		}
		s.publish(result)
		return TypeEventsAck, contract.NodeAck("", ""), nil
	default:
		return 0, nil, ErrType
	}
}

func (s *Server) auth(ctx context.Context, body []byte) (string, error) {
	var msg struct {
		AgentID string `json:"agent_id"`
		Token   string `json:"token"`
	}
	if err := Unmarshal(body, &msg); err != nil || msg.AgentID == "" || msg.Token == "" {
		return "", errors.New("jeton d'agent requis")
	}
	if err := s.Store.AuthenticateAgent(ctx, msg.AgentID, msg.Token); err != nil {
		return "", errors.New("jeton d'agent invalide")
	}
	return msg.AgentID, nil
}

func (s *Server) publish(result store.ApplyResult) {
	for _, activity := range result.Activities {
		s.Hub.Publish(map[string]any{"type": "activity", "event": activity})
	}
	if result.Changed {
		s.Hub.Publish(map[string]string{"type": "graph.changed"})
	}
}

func writeValue(w io.Writer, typ uint8, v any) error {
	body, err := Marshal(v)
	if err != nil {
		return err
	}
	return Write(w, Frame{Type: typ, Body: body})
}

type limiter struct {
	mu   sync.Mutex
	hits map[string][]time.Time
}

func (l *limiter) allow(key string, max int, window time.Duration) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := time.Now()
	kept := l.hits[key][:0]
	for _, at := range l.hits[key] {
		if now.Sub(at) < window {
			kept = append(kept, at)
		}
	}
	if len(kept) >= max {
		l.hits[key] = kept
		return false
	}
	l.hits[key] = append(kept, now)
	return true
}
