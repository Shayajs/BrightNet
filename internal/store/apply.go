package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net"
	"strings"
	"time"

	"brightnet/internal/contract"
	"brightnet/internal/id"
	"brightnet/internal/ingest"
)

type ApplyResult struct {
	Activities []contract.Activity
	Changed    bool
}

func (s *Store) ApplySnapshot(ctx context.Context, agentID string, snap contract.Snapshot) (ApplyResult, error) {
	ingest.Prepare(&snap)
	at := snap.CollectedAt.UTC()
	if at.IsZero() {
		at = time.Now().UTC()
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return ApplyResult{}, err
	}
	defer tx.Rollback()

	var existing int
	if err := tx.QueryRow(`SELECT COUNT(*) FROM entities WHERE source_agent_id=?`, agentID).Scan(&existing); err != nil {
		return ApplyResult{}, err
	}
	quiet := existing == 0
	var activities []contract.Activity
	changed := false

	hostKey := ingest.HostKey(agentID, snap.Host.MachineID)
	hostID, hostChanged, hostEvents, err := upsertEntity(tx, entityWrite{
		Kind: "host", Name: fallback(snap.Host.Hostname, "hôte"), Key: hostKey, Status: "online",
		Simulated: snap.Simulated, AgentID: agentID, At: at,
		Properties: map[string]any{
			"hostname": snap.Host.Hostname, "os": snap.Host.OS, "os_version": snap.Host.OSVersion,
			"arch": snap.Host.Arch, "kernel": snap.Host.Kernel, "machine_id": snap.Host.MachineID,
			"virtualization": snap.Host.Virtualization, "observation_scope": snap.Host.ObservationScope,
		},
	}, quiet)
	if err != nil {
		return ApplyResult{}, err
	}
	changed = changed || hostChanged
	activities = append(activities, hostEvents...)
	if err := observe(tx, hostID, agentID, "host", "observed", map[string]any{"status": "online"}, snap.Simulated, at); err != nil {
		return ApplyResult{}, err
	}
	if err := shareHost(tx, hostID, hostKey, snap.Host.MachineID, at); err != nil {
		return ApplyResult{}, err
	}

	if ingest.RunStatus(snap.CollectorRuns, "host") == "ok" {
		seen := map[string]bool{hostKey: true}
		for _, iface := range snap.Host.Interfaces {
			if iface.Name == "" || strings.Contains(iface.Flags, "loopback") {
				continue
			}
			key := "iface:" + agentID + ":" + iface.Name
			status := "offline"
			if strings.Contains(iface.Flags, "up") {
				status = "online"
			}
			ifaceID, ch, ev, err := upsertEntity(tx, entityWrite{
				Kind: "network_interface", Name: iface.Name, Key: key, ParentID: hostID, Status: status,
				Simulated: snap.Simulated, AgentID: agentID, At: at,
				Properties: map[string]any{"mac": iface.MAC, "mtu": iface.MTU, "flags": iface.Flags, "addrs": iface.Addrs},
			}, quiet)
			if err != nil {
				return ApplyResult{}, err
			}
			changed = changed || ch
			activities = append(activities, ev...)
			seen[key] = true
			if _, err := relate(tx, ifaceID, hostID, "BELONGS_TO", "observed", 1, nil, agentID, at); err != nil {
				return ApplyResult{}, err
			}
			for _, raw := range iface.Addrs {
				ip, mask, ok := usableIP(raw)
				if !ok {
					continue
				}
				akey := "addr:" + agentID + ":" + iface.Name + ":" + ip
				addrID, ch, ev, err := upsertEntity(tx, entityWrite{
					Kind: "address", Name: ip, Key: akey, ParentID: ifaceID, Status: "online",
					Simulated: snap.Simulated, AgentID: agentID, At: at,
					Properties: map[string]any{"ip": ip, "mask": mask, "interface": iface.Name},
				}, quiet)
				if err != nil {
					return ApplyResult{}, err
				}
				changed = changed || ch
				activities = append(activities, ev...)
				seen[akey] = true
				if _, err := relate(tx, addrID, ifaceID, "BELONGS_TO", "observed", 1, nil, agentID, at); err != nil {
					return ApplyResult{}, err
				}
			}
		}
		abs, err := markAbsent(tx, agentID, snap.Simulated, map[string]bool{"network_interface": true, "address": true}, seen, quiet, at)
		if err != nil {
			return ApplyResult{}, err
		}
		if len(abs) > 0 {
			changed = true
		}
		activities = append(activities, abs...)
		if err := staleRelations(tx, agentID, []string{"BELONGS_TO"}, at); err != nil {
			return ApplyResult{}, err
		}
	}

	dockerOK := ingest.RunStatus(snap.CollectorRuns, "docker") == "ok" && snap.Docker != nil && snap.Docker.Available && snap.Docker.Engine != nil && snap.Docker.Engine.ID != ""
	if dockerOK {
		engine := snap.Docker.Engine
		engKey := "docker-engine:" + engine.ID
		engID, ch, ev, err := upsertEntity(tx, entityWrite{
			Kind: "docker_engine", Name: fallback(engine.Name, "Docker"), Key: engKey, ParentID: hostID, Status: "online",
			Simulated: snap.Simulated, AgentID: agentID, At: at,
			Properties: map[string]any{
				"engine_id": engine.ID, "version": engine.Version, "os": engine.OS, "arch": engine.Arch,
				"operating_system": engine.OperatingSystem, "cpus": engine.CPUs, "memory_bytes": engine.MemoryBytes,
			},
		}, quiet)
		if err != nil {
			return ApplyResult{}, err
		}
		changed = changed || ch
		activities = append(activities, ev...)
		if _, err := relate(tx, hostID, engID, "RUNS", "observed", 1, nil, agentID, at); err != nil {
			return ApplyResult{}, err
		}
		seen := map[string]bool{engKey: true}
		netIDs := map[string]string{}
		volIDs := map[string]string{}
		for _, network := range snap.Docker.Networks {
			if network.ID == "" {
				continue
			}
			key := "docker-network:" + engine.ID + ":" + network.ID
			nid, ch, ev, err := upsertEntity(tx, entityWrite{
				Kind: "docker_network", Name: fallback(network.Name, network.ID[:min(12, len(network.ID))]), Key: key, ParentID: engID, Status: "present",
				Simulated: snap.Simulated, AgentID: agentID, At: at,
				Properties: map[string]any{"network_id": network.ID, "driver": network.Driver, "scope": network.Scope, "subnets": network.Subnets},
			}, quiet)
			if err != nil {
				return ApplyResult{}, err
			}
			changed = changed || ch
			activities = append(activities, ev...)
			seen[key] = true
			netIDs[network.ID] = nid
			if _, err := relate(tx, engID, nid, "HOSTS", "observed", 1, nil, agentID, at); err != nil {
				return ApplyResult{}, err
			}
		}
		for _, volume := range snap.Docker.Volumes {
			if volume.Name == "" {
				continue
			}
			key := "docker-volume:" + engine.ID + ":" + volume.Name
			vid, ch, ev, err := upsertEntity(tx, entityWrite{
				Kind: "docker_volume", Name: volume.Name, Key: key, ParentID: engID, Status: "present",
				Simulated: snap.Simulated, AgentID: agentID, At: at,
				Properties: map[string]any{"driver": volume.Driver, "scope": volume.Scope},
			}, quiet)
			if err != nil {
				return ApplyResult{}, err
			}
			changed = changed || ch
			activities = append(activities, ev...)
			seen[key] = true
			volIDs[volume.Name] = vid
			if _, err := relate(tx, engID, vid, "HOSTS", "observed", 1, nil, agentID, at); err != nil {
				return ApplyResult{}, err
			}
		}
		for _, container := range snap.Docker.Containers {
			labels := ingest.FilterLabels(container.Labels)
			key, identity := ingest.ContainerKey(engine.ID, container.Name, labels)
			state := container.State
			if state == "" {
				state = "unknown"
			}
			props := map[string]any{
				"container_id": container.ID, "image": container.Image, "image_id": container.ImageID,
				"status_text": container.StatusText, "labels": labels, "ports": container.Ports,
				"networks": container.Networks, "mounts": container.Mounts, "identity_evidence": identity,
			}
			cid, ch, ev, err := upsertEntity(tx, entityWrite{
				Kind: "docker_container", Name: fallback(container.Name, container.ID), Key: key, ParentID: engID, Status: state,
				Simulated: snap.Simulated, AgentID: agentID, At: at, Properties: props,
			}, quiet)
			if err != nil {
				return ApplyResult{}, err
			}
			changed = changed || ch
			activities = append(activities, ev...)
			seen[key] = true
			if _, err := relate(tx, engID, cid, "HOSTS", "observed", 1, nil, agentID, at); err != nil {
				return ApplyResult{}, err
			}
			if err := observe(tx, cid, agentID, "docker", "observed", map[string]any{"status": state, "image": container.Image}, snap.Simulated, at); err != nil {
				return ApplyResult{}, err
			}
			for _, network := range container.Networks {
				nid := netIDs[network.NetworkID]
				if nid == "" && network.NetworkID != "" {
					nkey := "docker-network:" + engine.ID + ":" + network.NetworkID
					var ch bool
					var ev []contract.Activity
					nid, ch, ev, err = upsertEntity(tx, entityWrite{
						Kind: "docker_network", Name: fallback(network.Name, "réseau"), Key: nkey, ParentID: engID, Status: "present",
						Simulated: snap.Simulated, AgentID: agentID, At: at,
						Properties: map[string]any{"network_id": network.NetworkID},
					}, quiet)
					if err != nil {
						return ApplyResult{}, err
					}
					changed = changed || ch
					activities = append(activities, ev...)
					seen[nkey] = true
					netIDs[network.NetworkID] = nid
					if _, err := relate(tx, engID, nid, "HOSTS", "observed", 1, nil, agentID, at); err != nil {
						return ApplyResult{}, err
					}
				}
				if nid == "" {
					continue
				}
				if _, err := relate(tx, cid, nid, "MEMBER_OF", "observed", 1, map[string]any{"ip": network.IPAddress, "ipv6": network.IPv6}, agentID, at); err != nil {
					return ApplyResult{}, err
				}
			}
			for _, mount := range container.Mounts {
				if mount.Type != "volume" || mount.Name == "" {
					continue
				}
				vid := volIDs[mount.Name]
				if vid == "" {
					vkey := "docker-volume:" + engine.ID + ":" + mount.Name
					var ch bool
					var ev []contract.Activity
					vid, ch, ev, err = upsertEntity(tx, entityWrite{
						Kind: "docker_volume", Name: mount.Name, Key: vkey, ParentID: engID, Status: "present",
						Simulated: snap.Simulated, AgentID: agentID, At: at,
						Properties: map[string]any{"driver": "unknown"},
					}, quiet)
					if err != nil {
						return ApplyResult{}, err
					}
					changed = changed || ch
					activities = append(activities, ev...)
					seen[vkey] = true
					volIDs[mount.Name] = vid
					if _, err := relate(tx, engID, vid, "HOSTS", "observed", 1, nil, agentID, at); err != nil {
						return ApplyResult{}, err
					}
				}
				if _, err := relate(tx, cid, vid, "MOUNTS", "observed", 1, map[string]any{"destination": mount.Destination, "rw": mount.RW}, agentID, at); err != nil {
					return ApplyResult{}, err
				}
			}
			published := make([]contract.PortSnapshot, 0, len(container.Ports))
			for _, port := range container.Ports {
				if port.PublicPort == 0 {
					continue
				}
				published = append(published, port)
			}
			if len(published) > 0 {
				skey := "service:" + key
				name := fmt.Sprintf("%d/%s", published[0].PublicPort, fallback(published[0].Type, "tcp"))
				if len(published) > 1 {
					name = fmt.Sprintf("%d ports", len(published))
				}
				sid, ch, ev, err := upsertEntity(tx, entityWrite{
					Kind: "service", Name: name, Key: skey, ParentID: cid, Status: "online",
					Simulated: snap.Simulated, AgentID: agentID, At: at,
					Properties: map[string]any{"ports": published},
				}, quiet)
				if err != nil {
					return ApplyResult{}, err
				}
				changed = changed || ch
				activities = append(activities, ev...)
				seen[skey] = true
				if _, err := relate(tx, cid, sid, "EXPOSES", "observed", 1, map[string]any{"ports": len(published)}, agentID, at); err != nil {
					return ApplyResult{}, err
				}
			}
		}
		abs, err := markAbsent(tx, agentID, snap.Simulated, map[string]bool{
			"docker_engine": true, "docker_container": true, "docker_network": true, "docker_volume": true,
		}, seen, quiet, at)
		if err != nil {
			return ApplyResult{}, err
		}
		if len(abs) > 0 {
			changed = true
		}
		activities = append(activities, abs...)
		removed, err := dropUnseen(tx, agentID, snap.Simulated, "service", seen)
		if err != nil {
			return ApplyResult{}, err
		}
		if removed > 0 {
			changed = true
		}
		if err := staleRelations(tx, agentID, []string{"RUNS", "HOSTS", "MEMBER_OF", "MOUNTS", "EXPOSES"}, at); err != nil {
			return ApplyResult{}, err
		}
	}

	for _, run := range snap.CollectorRuns {
		if _, err := tx.Exec(`INSERT INTO collector_runs (id, agent_id, collector, status, error, started_at, finished_at) VALUES (?,?,?,?,?,?,?)`,
			id.New(), agentID, run.Collector, run.Status, run.Error, at.Format(timeLayout), at.Format(timeLayout)); err != nil {
			return ApplyResult{}, err
		}
	}
	if _, err := tx.Exec(`UPDATE agents SET last_seen_at=?, hostname=?, machine_id=? WHERE id=?`,
		at.Format(timeLayout), snap.Host.Hostname, snap.Host.MachineID, agentID); err != nil {
		return ApplyResult{}, err
	}
	if err := retain(tx, at); err != nil {
		return ApplyResult{}, err
	}
	if quiet {
		activities = []contract.Activity{mustActivity(tx, agentID, hostID, "inventory", "Inventaire initial reçu pour "+fallback(snap.Host.Hostname, "cet hôte"), snap.Simulated, at)}
		changed = true
	}
	if err := tx.Commit(); err != nil {
		return ApplyResult{}, err
	}
	return ApplyResult{Activities: compact(activities), Changed: changed}, nil
}

func (s *Store) ApplyDockerEvents(ctx context.Context, agentID string, events []contract.DockerEvent) (ApplyResult, error) {
	if len(events) == 0 {
		return ApplyResult{}, nil
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return ApplyResult{}, err
	}
	defer tx.Rollback()
	var activities []contract.Activity
	changed := false
	for _, event := range events {
		if !event.Relevant() {
			continue
		}
		at := event.Time.UTC()
		if at.IsZero() {
			at = time.Now().UTC()
		}
		message := fmt.Sprintf("Docker %s %s sur %s", event.Type, event.Action, fallback(event.ActorName, event.ActorID))
		status, ok := containerStatus(event.Action)
		if event.Type == "container" && ok && len(event.ActorID) >= 12 {
			rows, err := tx.Query(`
				SELECT id, status, display_name FROM entities
				WHERE source_agent_id=? AND kind='docker_container'
				AND instr(COALESCE(json_extract(properties,'$.container_id'), ''), ?) = 1`, agentID, event.ActorID)
			if err != nil {
				return ApplyResult{}, err
			}
			type hit struct{ id, status, name string }
			var hits []hit
			for rows.Next() {
				var h hit
				if err := rows.Scan(&h.id, &h.status, &h.name); err != nil {
					rows.Close()
					return ApplyResult{}, err
				}
				hits = append(hits, h)
			}
			rows.Close()
			if len(hits) == 1 && hits[0].status != status {
				if _, err := tx.Exec(`UPDATE entities SET status=?, last_seen_at=? WHERE id=?`, status, at.Format(timeLayout), hits[0].id); err != nil {
					return ApplyResult{}, err
				}
				message = statusMessage("docker_container", hits[0].name, status)
				activities = append(activities, mustActivity(tx, agentID, hits[0].id, "docker.event", message, false, at))
				changed = true
				continue
			}
		}
		activities = append(activities, mustActivity(tx, agentID, "", "docker.event", message, false, at))
	}
	if _, err := tx.Exec(`UPDATE agents SET last_seen_at=? WHERE id=?`, time.Now().UTC().Format(timeLayout), agentID); err != nil {
		return ApplyResult{}, err
	}
	if err := tx.Commit(); err != nil {
		return ApplyResult{}, err
	}
	return ApplyResult{Activities: activities, Changed: changed}, nil
}

func (s *Store) LoadFixture(ctx context.Context, actor string) (ApplyResult, error) {
	result, err := s.ApplySnapshot(ctx, "fixture", ingest.Fixture(time.Now().UTC()))
	if err != nil {
		return result, err
	}
	_ = s.audit(ctx, actor, "fixture", "démonstration chargée")
	return result, nil
}

type entityWrite struct {
	Kind       string
	Name       string
	Key        string
	ParentID   string
	Status     string
	Simulated  bool
	Properties map[string]any
	AgentID    string
	At         time.Time
}

func shareHost(tx *sql.Tx, hostID, hostKey, machineID string, at time.Time) error {
	if machineID == "" || hostKey == "" {
		return nil
	}
	rows, err := tx.Query(`SELECT id FROM entities WHERE kind='host' AND technical_key<>? AND json_extract(properties,'$.machine_id')=?`, hostKey, machineID)
	if err != nil {
		return err
	}
	defer rows.Close()
	var old []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return err
		}
		old = append(old, id)
	}
	if err := rows.Err(); err != nil {
		return err
	}
	for _, id := range old {
		if _, err := tx.Exec(`UPDATE entities SET parent_id=? WHERE parent_id=?`, hostID, id); err != nil {
			return err
		}
		if _, err := tx.Exec(`UPDATE entities SET status='absent', last_seen_at=? WHERE id=?`, at.Format(timeLayout), id); err != nil {
			return err
		}
	}
	return nil
}

func upsertEntity(tx *sql.Tx, w entityWrite, quiet bool) (string, bool, []contract.Activity, error) {
	if w.Properties == nil {
		w.Properties = map[string]any{}
	}
	props, err := json.Marshal(w.Properties)
	if err != nil {
		return "", false, nil, err
	}
	var entityID, prevStatus, prevProps, prevName string
	var locked int
	err = tx.QueryRow(`SELECT id, status, properties, display_name, name_locked FROM entities WHERE technical_key=?`, w.Key).
		Scan(&entityID, &prevStatus, &prevProps, &prevName, &locked)
	sim := boolInt(w.Simulated)
	stamp := w.At.UTC().Format(timeLayout)
	if err == sql.ErrNoRows {
		entityID = id.New()
		_, err = tx.Exec(`INSERT INTO entities (id, kind, display_name, name_locked, technical_key, parent_id, status, simulated, properties, first_seen_at, last_seen_at, source_agent_id) VALUES (?,?,?,0,?,?,?,?,?,?,?,?)`,
			entityID, w.Kind, w.Name, w.Key, nullIfEmpty(w.ParentID), w.Status, sim, string(props), stamp, stamp, w.AgentID)
		if err != nil {
			return "", false, nil, err
		}
		if quiet {
			return entityID, true, nil, nil
		}
		return entityID, true, []contract.Activity{mustActivity(tx, w.AgentID, entityID, "appear", appearMessage(w.Kind, w.Name), w.Simulated, w.At)}, nil
	}
	if err != nil {
		return "", false, nil, err
	}
	name := w.Name
	if locked == 1 {
		name = prevName
	}
	_, err = tx.Exec(`UPDATE entities SET kind=?, display_name=?, parent_id=?, status=?, simulated=?, properties=?, last_seen_at=?, source_agent_id=? WHERE id=?`,
		w.Kind, name, nullIfEmpty(w.ParentID), w.Status, sim, string(props), stamp, w.AgentID, entityID)
	if err != nil {
		return "", false, nil, err
	}
	changed := prevStatus != w.Status || prevProps != string(props)
	if quiet || prevStatus == w.Status {
		return entityID, changed, nil, nil
	}
	return entityID, true, []contract.Activity{mustActivity(tx, w.AgentID, entityID, "status", statusMessage(w.Kind, name, w.Status), w.Simulated, w.At)}, nil
}

func relate(tx *sql.Tx, source, target, kind, evidence string, confidence float64, props map[string]any, agentID string, at time.Time) (string, error) {
	if props == nil {
		props = map[string]any{}
	}
	raw, err := json.Marshal(props)
	if err != nil {
		return "", err
	}
	stamp := at.UTC().Format(timeLayout)
	var relID string
	err = tx.QueryRow(`SELECT id FROM relations WHERE source_id=? AND target_id=? AND kind=?`, source, target, kind).Scan(&relID)
	if err == sql.ErrNoRows {
		relID = id.New()
		_, err = tx.Exec(`INSERT INTO relations (id, source_id, target_id, kind, evidence, confidence, status, properties, first_seen_at, last_seen_at, source_agent_id) VALUES (?,?,?,?,?,?, 'active', ?,?,?,?)`,
			relID, source, target, kind, evidence, confidence, string(raw), stamp, stamp, agentID)
		return relID, err
	}
	if err != nil {
		return "", err
	}
	_, err = tx.Exec(`UPDATE relations SET evidence=?, confidence=?, status='active', properties=?, last_seen_at=?, source_agent_id=? WHERE id=?`,
		evidence, confidence, string(raw), stamp, agentID, relID)
	return relID, err
}

func markAbsent(tx *sql.Tx, agentID string, simulated bool, kinds map[string]bool, seen map[string]bool, quiet bool, at time.Time) ([]contract.Activity, error) {
	rows, err := tx.Query(`SELECT id, kind, display_name, technical_key, status FROM entities WHERE source_agent_id=? AND simulated=? AND status!='absent'`, agentID, boolInt(simulated))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	type row struct{ id, kind, name, key, status string }
	var doomed []row
	for rows.Next() {
		var item row
		if err := rows.Scan(&item.id, &item.kind, &item.name, &item.key, &item.status); err != nil {
			return nil, err
		}
		if kinds[item.kind] && !seen[item.key] {
			doomed = append(doomed, item)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	var activities []contract.Activity
	for _, item := range doomed {
		if _, err := tx.Exec(`UPDATE entities SET status='absent', last_seen_at=? WHERE id=?`, at.UTC().Format(timeLayout), item.id); err != nil {
			return nil, err
		}
		if !quiet {
			activities = append(activities, mustActivity(tx, agentID, item.id, "status", statusMessage(item.kind, item.name, "absent"), simulated, at))
		}
	}
	return activities, nil
}

func dropUnseen(tx *sql.Tx, agentID string, simulated bool, kind string, seen map[string]bool) (int, error) {
	rows, err := tx.Query(`SELECT id, technical_key FROM entities WHERE source_agent_id=? AND simulated=? AND kind=?`, agentID, boolInt(simulated), kind)
	if err != nil {
		return 0, err
	}
	defer rows.Close()
	var ids []string
	for rows.Next() {
		var id, key string
		if err := rows.Scan(&id, &key); err != nil {
			return 0, err
		}
		if !seen[key] {
			ids = append(ids, id)
		}
	}
	if err := rows.Err(); err != nil {
		return 0, err
	}
	for _, id := range ids {
		if _, err := tx.Exec(`DELETE FROM relations WHERE source_id=? OR target_id=?`, id, id); err != nil {
			return 0, err
		}
		if _, err := tx.Exec(`DELETE FROM observations WHERE entity_id=?`, id); err != nil {
			return 0, err
		}
		if _, err := tx.Exec(`UPDATE entities SET parent_id=NULL WHERE parent_id=?`, id); err != nil {
			return 0, err
		}
		if _, err := tx.Exec(`DELETE FROM entities WHERE id=?`, id); err != nil {
			return 0, err
		}
	}
	return len(ids), nil
}

func staleRelations(tx *sql.Tx, agentID string, kinds []string, at time.Time) error {
	for _, kind := range kinds {
		if _, err := tx.Exec(`UPDATE relations SET status='absent' WHERE source_agent_id=? AND kind=? AND status='active' AND last_seen_at < ?`,
			agentID, kind, at.UTC().Format(timeLayout)); err != nil {
			return err
		}
	}
	return nil
}

func observe(tx *sql.Tx, entityID, agentID, collector, evidence string, payload map[string]any, simulated bool, at time.Time) error {
	raw, _ := json.Marshal(payload)
	_, err := tx.Exec(`INSERT INTO observations (id, entity_id, agent_id, collector, evidence, payload, observed_at) VALUES (?,?,?,?,?,?,?)`,
		id.New(), entityID, agentID, collector, evidence, string(raw), at.UTC().Format(timeLayout))
	_ = simulated
	return err
}

func retain(tx *sql.Tx, at time.Time) error {
	if _, err := tx.Exec(`DELETE FROM observations WHERE observed_at < ?`, at.Add(-48*time.Hour).Format(timeLayout)); err != nil {
		return err
	}
	_, err := tx.Exec(`DELETE FROM events WHERE observed_at < ?`, at.Add(-7*24*time.Hour).Format(timeLayout))
	return err
}

func mustActivity(tx *sql.Tx, agentID, entityID, kind, message string, simulated bool, at time.Time) contract.Activity {
	item := contract.Activity{ID: id.New(), Kind: kind, Message: message, EntityID: entityID, Simulated: simulated, ObservedAt: at.UTC()}
	_, _ = tx.Exec(`INSERT INTO events (id, agent_id, entity_id, kind, message, payload, simulated, observed_at) VALUES (?,?,?,?,?,'{}',?,?)`,
		item.ID, nullIfEmpty(agentID), nullIfEmpty(entityID), kind, message, boolInt(simulated), at.UTC().Format(timeLayout))
	return item
}

func compact(in []contract.Activity) []contract.Activity {
	if in == nil {
		return []contract.Activity{}
	}
	return in
}

func nullIfEmpty(v string) any {
	if v == "" {
		return nil
	}
	return v
}

func boolInt(v bool) int {
	if v {
		return 1
	}
	return 0
}

func fallback(v, alt string) string {
	if strings.TrimSpace(v) == "" {
		return alt
	}
	return v
}

func usableIP(raw string) (string, string, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", "", false
	}
	ipPart, mask := raw, ""
	if slash := strings.Index(raw, "/"); slash >= 0 {
		ipPart, mask = raw[:slash], raw[slash+1:]
	}
	ip := net.ParseIP(ipPart)
	if ip == nil || ip.IsLoopback() || ip.IsLinkLocalUnicast() || ip.IsUnspecified() {
		return "", "", false
	}
	return ip.String(), mask, true
}

func containerStatus(action string) (string, bool) {
	switch action {
	case "start", "unpause", "restart":
		return "running", true
	case "die", "stop", "kill", "oom":
		return "exited", true
	case "pause":
		return "paused", true
	case "destroy", "remove":
		return "absent", true
	case "create":
		return "created", true
	default:
		return "", false
	}
}

func appearMessage(kind, name string) string {
	switch kind {
	case "docker_container":
		return "Le conteneur " + name + " est apparu"
	case "host":
		return "L'hôte " + name + " est apparu"
	default:
		return name + " est apparu"
	}
}

func statusMessage(kind, name, status string) string {
	if kind == "docker_container" {
		switch status {
		case "running":
			return "Le conteneur " + name + " est en cours d'exécution"
		case "exited":
			return "Le conteneur " + name + " est arrêté"
		case "absent":
			return "Le conteneur " + name + " a disparu du moteur"
		}
	}
	return name + " : " + status
}
