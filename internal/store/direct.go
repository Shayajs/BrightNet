package store

import (
	"context"
	"time"
)

func (s *Store) SyncDirect(ctx context.Context, agentIDs []string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	hosts := map[string]string{}
	for _, agentID := range agentIDs {
		var machine string
		if err := tx.QueryRow(`SELECT machine_id FROM agents WHERE id=?`, agentID).Scan(&machine); err != nil {
			continue
		}
		key := "host:agent:" + agentID
		if machine != "" {
			key = "host:machine:" + machine
		}
		var hostID string
		if err := tx.QueryRow(`SELECT id FROM entities WHERE kind='host' AND technical_key=? AND status!='absent'`, key).Scan(&hostID); err != nil {
			continue
		}
		hosts[hostID] = agentID
	}
	ids := make([]string, 0, len(hosts))
	for id := range hosts {
		ids = append(ids, id)
	}
	at := time.Now().UTC()
	for i := 0; i < len(ids); i++ {
		for j := i + 1; j < len(ids); j++ {
			left, right := ids[i], ids[j]
			if left > right {
				left, right = right, left
			}
			if _, err := relate(tx, left, right, "DIRECT", "observed", 1, map[string]any{"via": "core", "mode": "dbncp"}, "core", at); err != nil {
				return err
			}
		}
	}
	if _, err := tx.Exec(`UPDATE relations SET status='absent' WHERE kind='DIRECT' AND source_agent_id='core' AND status='active' AND last_seen_at < ?`, at.Format(timeLayout)); err != nil {
		return err
	}
	return tx.Commit()
}
