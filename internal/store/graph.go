package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"

	"brightnet/internal/contract"
)

var viewKinds = map[string]map[string]bool{
	"docker":   {"host": true, "docker_engine": true, "docker_container": true, "docker_network": true, "docker_volume": true, "service": true},
	"physical": {"host": true, "network_interface": true, "router": true},
	"logical":  {"host": true, "network_interface": true, "address": true, "router": true},
	"network":  {"network": true, "router": true, "host": true, "device": true},
	"complex":  {"network": true, "router": true, "host": true, "device": true, "network_interface": true, "address": true, "docker_engine": true, "docker_container": true, "docker_network": true, "docker_volume": true, "service": true},
}

type rawEntity struct {
	node    contract.GraphNode
	agentID string
}

func (s *Store) Graph(ctx context.Context, view, focus string) (contract.Graph, error) {
	graph := contract.Graph{
		View: view, GeneratedAt: time.Now().UTC(), DataOrigin: "live",
		Nodes: []contract.GraphNode{}, Edges: []contract.GraphEdge{},
	}
	kinds, known := viewKinds[view]
	if !known {
		graph.Available = false
		graph.Notice = "Cette vue n'est pas encore alimentée."
		return graph, nil
	}
	graph.Available = true
	rows, err := s.db.QueryContext(ctx, `SELECT id, kind, display_name, COALESCE(parent_id,''), status, simulated, technical_key, properties, last_seen_at, COALESCE(source_agent_id,'') FROM entities`)
	if err != nil {
		return graph, err
	}
	defer rows.Close()
	var all []rawEntity
	byID := map[string]rawEntity{}
	for rows.Next() {
		var item rawEntity
		var props, seen string
		var simulated int
		if err := rows.Scan(&item.node.ID, &item.node.Kind, &item.node.Label, &item.node.ParentID, &item.node.Status, &simulated, &item.node.TechnicalKey, &props, &seen, &item.agentID); err != nil {
			return graph, err
		}
		item.node.Simulated = simulated == 1
		item.node.Properties = map[string]any{}
		_ = json.Unmarshal([]byte(props), &item.node.Properties)
		item.node.LastSeenAt, _ = time.Parse(time.RFC3339Nano, seen)
		all = append(all, item)
		byID[item.node.ID] = item
	}
	if err := rows.Err(); err != nil {
		return graph, err
	}
	agents, err := s.agentSeen(ctx)
	if err != nil {
		return graph, err
	}
	now := time.Now().UTC()
	included := map[string]bool{}
	extra := focusedDescendants(all, focus)
	simulated, real := 0, 0
	for _, item := range all {
		if !kinds[item.node.Kind] && !extra[item.node.ID] {
			continue
		}
		if view == "network" && item.node.Status == "absent" {
			continue
		}
		if item.node.Kind == "host" {
			if seen, ok := agents[item.agentID]; ok && now.Sub(seen) > 45*time.Second && item.node.Status == "online" {
				item.node.Status = "offline"
			}
		}
		if item.node.ParentID != "" {
			parent, ok := byID[item.node.ParentID]
			if !ok || (!kinds[parent.node.Kind] && !extra[parent.node.ID]) {
				item.node.ParentID = ""
			}
		}
		graph.Nodes = append(graph.Nodes, item.node)
		included[item.node.ID] = true
		if item.node.Simulated {
			simulated++
		} else {
			real++
		}
	}
	erows, err := s.db.QueryContext(ctx, `SELECT id, source_id, target_id, kind, evidence, confidence, status, properties, last_seen_at FROM relations WHERE status='active'`)
	if err != nil {
		return graph, err
	}
	defer erows.Close()
	for erows.Next() {
		var edge contract.GraphEdge
		var props, seen string
		if err := erows.Scan(&edge.ID, &edge.Source, &edge.Target, &edge.Kind, &edge.Evidence, &edge.Confidence, &edge.Status, &props, &seen); err != nil {
			return graph, err
		}
		if !included[edge.Source] || !included[edge.Target] {
			continue
		}
		edge.Properties = map[string]any{}
		_ = json.Unmarshal([]byte(props), &edge.Properties)
		edge.LastSeenAt, _ = time.Parse(time.RFC3339Nano, seen)
		graph.Edges = append(graph.Edges, edge)
	}
	if simulated > 0 && real > 0 {
		graph.DataOrigin = "mixed"
	} else if simulated > 0 {
		graph.DataOrigin = "fixture"
	}
	if len(graph.Nodes) == 0 {
		graph.Notice = "Aucune entité dans cette vue. L'agent n'a peut-être pas encore envoyé d'inventaire."
	}
	return graph, erows.Err()
}

func focusedDescendants(all []rawEntity, focus string) map[string]bool {
	extra := map[string]bool{}
	if focus == "" {
		return extra
	}
	children := map[string][]string{}
	for _, item := range all {
		if item.node.ParentID != "" {
			children[item.node.ParentID] = append(children[item.node.ParentID], item.node.ID)
		}
	}
	var walk func(string)
	walk = func(id string) {
		for _, child := range children[id] {
			if extra[child] {
				continue
			}
			extra[child] = true
			walk(child)
		}
	}
	walk(focus)
	return extra
}

func (s *Store) Entity(ctx context.Context, entityID string) (contract.GraphNode, []contract.GraphEdge, error) {
	var node contract.GraphNode
	var props, seen, parent string
	var simulated int
	err := s.db.QueryRowContext(ctx, `SELECT id, kind, display_name, COALESCE(parent_id,''), status, simulated, technical_key, properties, last_seen_at FROM entities WHERE id=?`, entityID).
		Scan(&node.ID, &node.Kind, &node.Label, &parent, &node.Status, &simulated, &node.TechnicalKey, &props, &seen)
	if err != nil {
		return node, nil, err
	}
	node.ParentID = parent
	node.Simulated = simulated == 1
	node.Properties = map[string]any{}
	_ = json.Unmarshal([]byte(props), &node.Properties)
	node.LastSeenAt, _ = time.Parse(time.RFC3339Nano, seen)
	rows, err := s.db.QueryContext(ctx, `SELECT id, source_id, target_id, kind, evidence, confidence, status, properties, last_seen_at FROM relations WHERE source_id=? OR target_id=?`, entityID, entityID)
	if err != nil {
		return node, nil, err
	}
	defer rows.Close()
	edges := []contract.GraphEdge{}
	for rows.Next() {
		var edge contract.GraphEdge
		var raw, stamp string
		if err := rows.Scan(&edge.ID, &edge.Source, &edge.Target, &edge.Kind, &edge.Evidence, &edge.Confidence, &edge.Status, &raw, &stamp); err != nil {
			return node, nil, err
		}
		edge.Properties = map[string]any{}
		_ = json.Unmarshal([]byte(raw), &edge.Properties)
		edge.LastSeenAt, _ = time.Parse(time.RFC3339Nano, stamp)
		edges = append(edges, edge)
	}
	return node, edges, rows.Err()
}

func (s *Store) Summary(ctx context.Context) (contract.Summary, error) {
	summary := contract.Summary{Agents: []contract.AgentInfo{}, DataOrigin: "live"}
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM entities`).Scan(&summary.EntityCount); err != nil {
		return summary, err
	}
	var simulated, real int
	if err := s.db.QueryRowContext(ctx, `SELECT COALESCE(SUM(simulated),0), COALESCE(SUM(1-simulated),0) FROM entities`).Scan(&simulated, &real); err != nil {
		return summary, err
	}
	if simulated > 0 && real > 0 {
		summary.DataOrigin = "mixed"
	} else if simulated > 0 && real == 0 {
		summary.DataOrigin = "fixture"
	}
	rows, err := s.db.QueryContext(ctx, `SELECT id, hostname, version, last_seen_at, machine_id FROM agents ORDER BY created_at`)
	if err != nil {
		return summary, err
	}
	defer rows.Close()
	now := time.Now()
	for rows.Next() {
		var info contract.AgentInfo
		var seen sql.NullString
		if err := rows.Scan(&info.ID, &info.Hostname, &info.Version, &seen, &info.MachineID); err != nil {
			return summary, err
		}
		info.Status = "offline"
		if seen.Valid {
			if ts, err := time.Parse(time.RFC3339Nano, seen.String); err == nil {
				info.LastSeenAt = &ts
				if now.Sub(ts) <= 45*time.Second {
					info.Status = "online"
				}
			}
		}
		summary.Agents = append(summary.Agents, info)
	}
	return summary, rows.Err()
}

func (s *Store) ForgetAgent(ctx context.Context, id string) error {
	result, err := s.db.ExecContext(ctx, `DELETE FROM agents WHERE id=?`, id)
	if err != nil {
		return err
	}
	count, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if count == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func (s *Store) agentSeen(ctx context.Context) (map[string]time.Time, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, last_seen_at FROM agents WHERE last_seen_at IS NOT NULL`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]time.Time{}
	for rows.Next() {
		var id, seen string
		if err := rows.Scan(&id, &seen); err != nil {
			return nil, err
		}
		if ts, err := time.Parse(time.RFC3339Nano, seen); err == nil {
			out[id] = ts
		}
	}
	return out, rows.Err()
}
