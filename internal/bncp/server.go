package bncp

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net"
	"sync"
	"time"

	"brightnet/internal/bus"
	"brightnet/internal/contract"
	"brightnet/internal/store"
)

type Server struct {
	Store *store.Store
	Bus   *bus.Bus
	limit limiter
	mu    sync.Mutex
	peers map[string]*session
}

type session struct {
	conn    net.Conn
	mu      sync.Mutex
	agentID string
}

func NewServer(st *store.Store, b *bus.Bus) *Server {
	return &Server{Store: st, Bus: b, limit: limiter{hits: map[string][]time.Time{}}, peers: map[string]*session{}}
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
	sess := &session{conn: conn}
	defer func() {
		s.unbind(sess)
		_ = conn.Close()
	}()
	for {
		_ = conn.SetReadDeadline(time.Now().Add(2 * time.Minute))
		frame, err := Read(conn)
		if err != nil {
			if !errors.Is(err, io.EOF) && !errors.Is(err, ErrShort) && ctx.Err() == nil {
				slog.Debug("trame BNCP ignorée", "err", err)
			}
			return
		}
		ack, body, err := s.dispatch(ctx, sess, conn.RemoteAddr().String(), frame)
		if err != nil {
			_ = sess.write(TypeError, map[string]string{"error": err.Error()})
			return
		}
		if err := sess.write(ack, body); err != nil {
			return
		}
	}
}

func (sess *session) write(typ uint8, body any) error {
	sess.mu.Lock()
	defer sess.mu.Unlock()
	_ = sess.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
	return writeValue(sess.conn, typ, body)
}

func (s *Server) dispatch(ctx context.Context, sess *session, remote string, frame Frame) (uint8, any, error) {
	switch frame.Type {
	case TypeEnroll:
		if !s.limit.allow(remote, 8, time.Minute) {
			return 0, nil, errors.New("trop de tentatives")
		}
		var req contract.EnrollRequest
		if err := Unmarshal(frame.Body, &req); err != nil {
			return 0, nil, errors.New("appairage refusé")
		}
		agentID, token, err := s.Store.Enroll(ctx, req)
		if err != nil {
			return 0, nil, errors.New("code d'appairage invalide")
		}
		s.bind(sess, agentID)
		return TypeEnrollAck, contract.NodeAck(agentID, token), nil
	case TypeHeartbeat:
		id, err := s.auth(ctx, frame.Body)
		if err != nil {
			return 0, nil, err
		}
		s.bind(sess, id)
		if err := s.Store.TouchAgent(ctx, id); err != nil {
			return 0, nil, errors.New("battement refusé")
		}
		ack := contract.NodeAck("", "")
		ids := s.peerIDs(id)
		ack.Peers = ids
		ack.PeerNames = s.peerNames(ctx, ids)
		return TypeHeartbeatAck, ack, nil
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
		s.bind(sess, id)
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
		s.bind(sess, id)
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
	case TypeDirect:
		id, err := s.auth(ctx, frame.Body)
		if err != nil {
			return 0, nil, err
		}
		s.bind(sess, id)
		var msg struct {
			To      string `json:"to"`
			Payload string `json:"payload"`
		}
		if err := Unmarshal(frame.Body, &msg); err != nil || msg.To == "" || msg.To == id || len(msg.Payload) > 1024 {
			return 0, nil, errors.New("message direct refusé")
		}
		target := s.peer(msg.To)
		if target == nil {
			return 0, nil, errors.New("pair hors ligne")
		}
		if err := target.write(TypeDirect, map[string]string{"from": id, "payload": msg.Payload}); err != nil {
			return 0, nil, errors.New("pair hors ligne")
		}
		s.current(ctx, id, msg.To)
		return TypeDirectAck, map[string]bool{"ok": true}, nil
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
		s.Bus.Publish(map[string]any{"type": "activity", "event": activity})
	}
	if result.Changed {
		s.Bus.Publish(map[string]string{"type": "graph.changed"})
	}
}

func (s *Server) bind(sess *session, agentID string) {
	if agentID == "" {
		return
	}
	s.mu.Lock()
	if current := s.peers[agentID]; current != sess {
		s.peers[agentID] = sess
		sess.agentID = agentID
		s.mu.Unlock()
		s.syncPeers()
		return
	}
	s.mu.Unlock()
}

func (s *Server) unbind(sess *session) {
	s.mu.Lock()
	if sess.agentID != "" && s.peers[sess.agentID] == sess {
		delete(s.peers, sess.agentID)
		s.mu.Unlock()
		s.syncPeers()
		return
	}
	s.mu.Unlock()
}

func (s *Server) peer(agentID string) *session {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.peers[agentID]
}

func (s *Server) peerNames(ctx context.Context, ids []string) []string {
	names := make([]string, len(ids))
	for i, id := range ids {
		names[i] = s.Store.AgentHostname(ctx, id)
	}
	return names
}

func (s *Server) current(ctx context.Context, fromAgent, toAgent string) {
	from := s.Store.HostID(ctx, fromAgent)
	to := s.Store.HostID(ctx, toAgent)
	if from == "" || to == "" || from == to {
		return
	}
	s.Bus.Publish(map[string]string{"type": "current", "from": from, "to": to})
}

func (s *Server) peerIDs(self string) []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	ids := []string{}
	for id := range s.peers {
		if id != self {
			ids = append(ids, id)
		}
	}
	return ids
}

func (s *Server) syncPeers() {
	s.mu.Lock()
	ids := make([]string, 0, len(s.peers))
	for id := range s.peers {
		ids = append(ids, id)
	}
	s.mu.Unlock()
	if err := s.Store.SyncDirect(context.Background(), ids); err != nil {
		slog.Debug("dBNCP non relié", "err", err)
		return
	}
	s.Bus.Publish(map[string]string{"type": "graph.changed"})
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
