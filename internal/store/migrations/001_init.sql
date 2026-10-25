CREATE TABLE schema_migrations (
  version INTEGER PRIMARY KEY,
  applied_at TEXT NOT NULL
);

CREATE TABLE users (
  id TEXT PRIMARY KEY,
  username TEXT NOT NULL UNIQUE,
  password_hash TEXT NOT NULL,
  created_at TEXT NOT NULL
);

CREATE TABLE sessions (
  id TEXT PRIMARY KEY,
  user_id TEXT NOT NULL REFERENCES users(id),
  token_hash TEXT NOT NULL UNIQUE,
  expires_at TEXT NOT NULL,
  created_at TEXT NOT NULL
);

CREATE TABLE pairing_codes (
  code_hash TEXT PRIMARY KEY,
  expires_at TEXT NOT NULL,
  used_at TEXT,
  created_at TEXT NOT NULL,
  reusable INTEGER NOT NULL DEFAULT 0
);

CREATE TABLE agents (
  id TEXT PRIMARY KEY,
  token_hash TEXT NOT NULL UNIQUE,
  display_name TEXT NOT NULL,
  hostname TEXT NOT NULL DEFAULT '',
  machine_id TEXT NOT NULL DEFAULT '',
  version TEXT NOT NULL DEFAULT '',
  last_seen_at TEXT,
  created_at TEXT NOT NULL
);

CREATE TABLE entities (
  id TEXT PRIMARY KEY,
  kind TEXT NOT NULL,
  display_name TEXT NOT NULL,
  name_locked INTEGER NOT NULL DEFAULT 0,
  technical_key TEXT NOT NULL UNIQUE,
  parent_id TEXT REFERENCES entities(id),
  status TEXT NOT NULL,
  simulated INTEGER NOT NULL DEFAULT 0,
  properties TEXT NOT NULL DEFAULT '{}',
  first_seen_at TEXT NOT NULL,
  last_seen_at TEXT NOT NULL,
  source_agent_id TEXT
);

CREATE TABLE observations (
  id TEXT PRIMARY KEY,
  entity_id TEXT NOT NULL REFERENCES entities(id),
  agent_id TEXT,
  collector TEXT NOT NULL,
  evidence TEXT NOT NULL,
  payload TEXT NOT NULL,
  observed_at TEXT NOT NULL
);

CREATE TABLE relations (
  id TEXT PRIMARY KEY,
  source_id TEXT NOT NULL REFERENCES entities(id),
  target_id TEXT NOT NULL REFERENCES entities(id),
  kind TEXT NOT NULL,
  evidence TEXT NOT NULL,
  confidence REAL NOT NULL,
  status TEXT NOT NULL,
  properties TEXT NOT NULL DEFAULT '{}',
  first_seen_at TEXT NOT NULL,
  last_seen_at TEXT NOT NULL,
  source_agent_id TEXT,
  UNIQUE (source_id, target_id, kind)
);

CREATE TABLE collector_runs (
  id TEXT PRIMARY KEY,
  agent_id TEXT NOT NULL,
  collector TEXT NOT NULL,
  status TEXT NOT NULL,
  error TEXT NOT NULL DEFAULT '',
  started_at TEXT NOT NULL,
  finished_at TEXT NOT NULL
);

CREATE TABLE events (
  id TEXT PRIMARY KEY,
  agent_id TEXT,
  entity_id TEXT,
  kind TEXT NOT NULL,
  message TEXT NOT NULL,
  payload TEXT NOT NULL DEFAULT '{}',
  simulated INTEGER NOT NULL DEFAULT 0,
  observed_at TEXT NOT NULL
);

CREATE TABLE graph_layouts (
  user_id TEXT NOT NULL,
  view TEXT NOT NULL,
  positions TEXT NOT NULL,
  updated_at TEXT NOT NULL,
  PRIMARY KEY (user_id, view)
);

CREATE TABLE audit_log (
  id TEXT PRIMARY KEY,
  actor TEXT NOT NULL,
  action TEXT NOT NULL,
  detail TEXT NOT NULL,
  created_at TEXT NOT NULL
);

CREATE TABLE metrics (
  id TEXT PRIMARY KEY,
  entity_id TEXT NOT NULL,
  name TEXT NOT NULL,
  value REAL NOT NULL,
  observed_at TEXT NOT NULL
);

CREATE TABLE alerts (
  id TEXT PRIMARY KEY,
  entity_id TEXT,
  severity TEXT NOT NULL,
  message TEXT NOT NULL,
  status TEXT NOT NULL,
  created_at TEXT NOT NULL
);

CREATE INDEX idx_entities_kind ON entities(kind);
CREATE INDEX idx_entities_agent ON entities(source_agent_id);
CREATE INDEX idx_events_time ON events(observed_at);
CREATE INDEX idx_relations_source ON relations(source_id);
CREATE INDEX idx_relations_target ON relations(target_id);
