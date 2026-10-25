package runtime

import (
	"context"
	"errors"

	"brightnet/internal/bncp"
	"brightnet/internal/contract"
)

var errUnauthorized = errors.New("jeton d'agent invalide")

type wire interface {
	Enroll(ctx context.Context, req contract.EnrollRequest) (contract.AgentConfig, error)
	Heartbeat(ctx context.Context, agentID, token string) (contract.AgentConfig, error)
	Snapshot(ctx context.Context, agentID, token string, snap contract.Snapshot) (contract.AgentConfig, error)
	Events(ctx context.Context, agentID, token string, events []contract.DockerEvent) error
	Direct(ctx context.Context, agentID, token, to, payload string) error
	Close() error
}

type httpWire struct{ agent *Agent }

func (w httpWire) Enroll(ctx context.Context, req contract.EnrollRequest) (contract.AgentConfig, error) {
	var cfg contract.AgentConfig
	status, err := w.agent.post(ctx, "/api/v1/agents/enroll", req, &cfg, false)
	if status == 401 {
		return cfg, errUnauthorized
	}
	return cfg, err
}

func (w httpWire) Heartbeat(ctx context.Context, agentID, token string) (contract.AgentConfig, error) {
	var cfg contract.AgentConfig
	status, err := w.agent.post(ctx, "/api/v1/agents/"+agentID+"/heartbeat", map[string]any{}, &cfg, true)
	if status == 401 {
		return cfg, errUnauthorized
	}
	return cfg, err
}

func (w httpWire) Snapshot(ctx context.Context, agentID, token string, snap contract.Snapshot) (contract.AgentConfig, error) {
	var cfg contract.AgentConfig
	status, err := w.agent.post(ctx, "/api/v1/agents/"+agentID+"/snapshot", snap, &cfg, true)
	if status == 401 {
		return cfg, errUnauthorized
	}
	return cfg, err
}

func (w httpWire) Events(ctx context.Context, agentID, token string, events []contract.DockerEvent) error {
	status, err := w.agent.post(ctx, "/api/v1/agents/"+agentID+"/events", contract.EventBatch{Events: events}, nil, true)
	if status == 401 {
		return errUnauthorized
	}
	return err
}

func (w httpWire) Close() error { return nil }

func (w httpWire) Direct(context.Context, string, string, string, string) error { return nil }

type bncpWire struct{ client *bncp.Client }

func (w bncpWire) Enroll(ctx context.Context, req contract.EnrollRequest) (contract.AgentConfig, error) {
	var cfg contract.AgentConfig
	err := w.client.RoundTrip(ctx, bncp.TypeEnroll, req, &cfg)
	return cfg, err
}

func (w bncpWire) Heartbeat(ctx context.Context, agentID, token string) (contract.AgentConfig, error) {
	var cfg contract.AgentConfig
	err := w.client.RoundTrip(ctx, bncp.TypeHeartbeat, map[string]string{"agent_id": agentID, "token": token}, &cfg)
	if errors.Is(err, bncp.ErrUnauthorized) {
		return cfg, errUnauthorized
	}
	return cfg, err
}

func (w bncpWire) Snapshot(ctx context.Context, agentID, token string, snap contract.Snapshot) (contract.AgentConfig, error) {
	var cfg contract.AgentConfig
	err := w.client.RoundTrip(ctx, bncp.TypeSnapshot, map[string]any{
		"agent_id": agentID, "token": token, "snapshot": snap,
	}, &cfg)
	if errors.Is(err, bncp.ErrUnauthorized) {
		return cfg, errUnauthorized
	}
	return cfg, err
}

func (w bncpWire) Events(ctx context.Context, agentID, token string, events []contract.DockerEvent) error {
	err := w.client.RoundTrip(ctx, bncp.TypeEvents, map[string]any{
		"agent_id": agentID, "token": token, "events": events,
	}, nil)
	if errors.Is(err, bncp.ErrUnauthorized) {
		return errUnauthorized
	}
	return err
}

func (w bncpWire) Close() error { return w.client.Close() }

func (w bncpWire) Direct(ctx context.Context, agentID, token, to, payload string) error {
	return w.client.RoundTrip(ctx, bncp.TypeDirect, map[string]string{
		"agent_id": agentID, "token": token, "to": to, "payload": payload,
	}, nil)
}
