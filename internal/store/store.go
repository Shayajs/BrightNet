package store

import (
	"context"
	"crypto/subtle"
	"database/sql"
	_ "embed"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"brightnet/internal/contract"
	"brightnet/internal/id"

	_ "modernc.org/sqlite"
)

//go:embed migrations/001_init.sql
var migration001 string

type Store struct {
	db *sql.DB
}

func Open(path string) (*Store, error) {
	if path != ":memory:" {
		path = filepath.Clean(path)
	}
	dsn := "file:" + path + "?_pragma=foreign_keys(1)&_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)"
	if path == ":memory:" {
		dsn = "file:brightnet?mode=memory&cache=shared&_pragma=foreign_keys(1)"
	}
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(4)
	if err := db.Ping(); err != nil {
		_ = db.Close()
		return nil, err
	}
	s := &Store{db: db}
	if err := s.migrate(); err != nil {
		_ = db.Close()
		return nil, err
	}
	return s, nil
}

func (s *Store) Close() error { return s.db.Close() }

func (s *Store) migrate() error {
	var version int
	err := s.db.QueryRow(`SELECT COALESCE(MAX(version), 0) FROM schema_migrations`).Scan(&version)
	if err != nil {
		if execErr := execScript(s.db, migration001); execErr != nil {
			return execErr
		}
		_, err = s.db.Exec(`INSERT INTO schema_migrations (version, applied_at) VALUES (1, ?)`, time.Now().UTC().Format(timeLayout))
		return err
	}
	if version >= 1 {
		return nil
	}
	if err := execScript(s.db, migration001); err != nil {
		return err
	}
	_, err = s.db.Exec(`INSERT INTO schema_migrations (version, applied_at) VALUES (1, ?)`, time.Now().UTC().Format(timeLayout))
	return err
}

func (s *Store) NeedsSetup(ctx context.Context) (bool, error) {
	var n int
	err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM users`).Scan(&n)
	return n == 0, err
}

func (s *Store) EnsureAdmin(ctx context.Context, username, password string) error {
	if username == "" || password == "" {
		return nil
	}
	needs, err := s.NeedsSetup(ctx)
	if err != nil || !needs {
		return err
	}
	return s.CreateUser(ctx, username, password)
}

func (s *Store) CreateUser(ctx context.Context, username, password string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var n int
	if err := tx.QueryRow(`SELECT COUNT(*) FROM users`).Scan(&n); err != nil {
		return err
	}
	if n > 0 {
		return errors.New("un administrateur existe déjà")
	}
	hash, err := HashPassword(password)
	if err != nil {
		return err
	}
	_, err = tx.Exec(`INSERT INTO users (id, username, password_hash, created_at) VALUES (?,?,?,?)`,
		id.New(), username, hash, now())
	if err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) Login(ctx context.Context, username, password string) (string, error) {
	var userID, hash string
	err := s.db.QueryRowContext(ctx, `SELECT id, password_hash FROM users WHERE username=?`, username).Scan(&userID, &hash)
	if err != nil || !CheckPassword(hash, password) {
		return "", errors.New("identifiants invalides")
	}
	token, err := RandomToken()
	if err != nil {
		return "", err
	}
	_, err = s.db.ExecContext(ctx, `INSERT INTO sessions (id, user_id, token_hash, expires_at, created_at) VALUES (?,?,?,?,?)`,
		id.New(), userID, HashSecret(token), time.Now().UTC().Add(12*time.Hour).Format(timeLayout), now())
	if err != nil {
		return "", err
	}
	_ = s.audit(ctx, username, "login", "session ouverte")
	return token, nil
}

func (s *Store) Logout(ctx context.Context, token string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM sessions WHERE token_hash=?`, HashSecret(token))
	return err
}

func (s *Store) UserFromToken(ctx context.Context, token string) (string, string, error) {
	var userID, username, expires string
	err := s.db.QueryRowContext(ctx, `
		SELECT users.id, users.username, sessions.expires_at
		FROM sessions JOIN users ON users.id = sessions.user_id
		WHERE sessions.token_hash=?`, HashSecret(token)).Scan(&userID, &username, &expires)
	if err != nil {
		return "", "", err
	}
	exp, err := time.Parse(time.RFC3339Nano, expires)
	if err != nil || time.Now().After(exp) {
		_, _ = s.db.ExecContext(ctx, `DELETE FROM sessions WHERE token_hash=?`, HashSecret(token))
		return "", "", errors.New("session expirée")
	}
	return userID, username, nil
}

func (s *Store) CreatePairingCode(ctx context.Context, actor string) (string, time.Time, error) {
	plain, err := RandomToken()
	if err != nil {
		return "", time.Time{}, err
	}
	plain = plain[:12]
	expires := time.Now().UTC().Add(10 * time.Minute)
	_, err = s.db.ExecContext(ctx, `INSERT INTO pairing_codes (code_hash, expires_at, created_at, reusable) VALUES (?,?,?,0)`,
		HashSecret(plain), expires.Format(timeLayout), now())
	if err != nil {
		return "", time.Time{}, err
	}
	_ = s.audit(ctx, actor, "pairing_code", "code créé")
	return plain, expires, nil
}

func (s *Store) UpsertDevPairingCode(ctx context.Context, plain string, ttl time.Duration) error {
	if plain == "" {
		return nil
	}
	expires := time.Now().UTC().Add(ttl)
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO pairing_codes (code_hash, expires_at, used_at, created_at, reusable)
		VALUES (?,?,NULL,?,1)
		ON CONFLICT(code_hash) DO UPDATE SET expires_at=excluded.expires_at, reusable=1, used_at=NULL`,
		HashSecret(plain), expires.Format(timeLayout), now())
	return err
}

func (s *Store) Enroll(ctx context.Context, req contract.EnrollRequest) (string, string, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return "", "", err
	}
	defer tx.Rollback()
	var reusable int
	var expires string
	err = tx.QueryRow(`SELECT reusable, expires_at FROM pairing_codes WHERE code_hash=? AND (used_at IS NULL OR reusable=1)`, HashSecret(req.PairingCode)).Scan(&reusable, &expires)
	if err != nil {
		return "", "", errors.New("code d'appairage invalide")
	}
	exp, err := time.Parse(time.RFC3339Nano, expires)
	if err != nil || time.Now().After(exp) {
		return "", "", errors.New("code d'appairage expiré")
	}
	if reusable == 0 {
		if _, err := tx.Exec(`UPDATE pairing_codes SET used_at=? WHERE code_hash=?`, now(), HashSecret(req.PairingCode)); err != nil {
			return "", "", err
		}
	}
	token, err := RandomToken()
	if err != nil {
		return "", "", err
	}
	agentID := id.New()
	name := req.Hostname
	if name == "" {
		name = "agent"
	}
	_, err = tx.Exec(`INSERT INTO agents (id, token_hash, display_name, hostname, machine_id, version, last_seen_at, created_at) VALUES (?,?,?,?,?,?,?,?)`,
		agentID, HashSecret(token), name, req.Hostname, req.MachineID, req.Version, now(), now())
	if err != nil {
		return "", "", err
	}
	if err := insertAudit(tx, "agent:"+agentID, "enroll", req.Hostname, time.Now().UTC()); err != nil {
		return "", "", err
	}
	if err := tx.Commit(); err != nil {
		return "", "", err
	}
	return agentID, token, nil
}

func (s *Store) AuthenticateAgent(ctx context.Context, agentID, token string) error {
	var hash string
	err := s.db.QueryRowContext(ctx, `SELECT token_hash FROM agents WHERE id=?`, agentID).Scan(&hash)
	if err != nil || subtle.ConstantTimeCompare([]byte(hash), []byte(HashSecret(token))) != 1 {
		return errors.New("jeton d'agent invalide")
	}
	return nil
}

func (s *Store) TouchAgent(ctx context.Context, agentID string) error {
	_, err := s.db.ExecContext(ctx, `UPDATE agents SET last_seen_at=? WHERE id=?`, now(), agentID)
	return err
}

func (s *Store) audit(ctx context.Context, actor, action, detail string) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO audit_log (id, actor, action, detail, created_at) VALUES (?,?,?,?,?)`,
		id.New(), actor, action, detail, now())
	return err
}

func insertAudit(tx *sql.Tx, actor, action, detail string, at time.Time) error {
	_, err := tx.Exec(`INSERT INTO audit_log (id, actor, action, detail, created_at) VALUES (?,?,?,?,?)`,
		id.New(), actor, action, detail, at.UTC().Format(timeLayout))
	return err
}

const timeLayout = "2006-01-02T15:04:05.000000000Z"

func now() string { return time.Now().UTC().Format(timeLayout) }

func execScript(db *sql.DB, script string) error {
	for _, stmt := range strings.Split(script, ";") {
		stmt = strings.TrimSpace(stmt)
		if stmt == "" {
			continue
		}
		if _, err := db.Exec(stmt); err != nil {
			return fmt.Errorf("migration: %w", err)
		}
	}
	return nil
}

func (s *Store) SaveLayout(ctx context.Context, userID, view string, positions string) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO graph_layouts (user_id, view, positions, updated_at) VALUES (?,?,?,?)
		ON CONFLICT(user_id, view) DO UPDATE SET positions=excluded.positions, updated_at=excluded.updated_at`,
		userID, view, positions, now())
	return err
}

func (s *Store) LoadLayout(ctx context.Context, userID, view string) (string, error) {
	var positions string
	err := s.db.QueryRowContext(ctx, `SELECT positions FROM graph_layouts WHERE user_id=? AND view=?`, userID, view).Scan(&positions)
	if errors.Is(err, sql.ErrNoRows) {
		return "{}", nil
	}
	return positions, err
}

func (s *Store) ListEvents(ctx context.Context, limit int) ([]contract.Activity, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	rows, err := s.db.QueryContext(ctx, `SELECT id, kind, message, COALESCE(entity_id,''), simulated, observed_at FROM events WHERE instr(message, 'exec_') = 0 AND instr(message, 'health_status') = 0 ORDER BY observed_at DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []contract.Activity{}
	for rows.Next() {
		var item contract.Activity
		var observed string
		var simulated int
		if err := rows.Scan(&item.ID, &item.Kind, &item.Message, &item.EntityID, &simulated, &observed); err != nil {
			return nil, err
		}
		item.Simulated = simulated == 1
		item.ObservedAt, _ = time.Parse(time.RFC3339Nano, observed)
		out = append(out, item)
	}
	return out, rows.Err()
}

func (s *Store) RenameEntity(ctx context.Context, entityID, name, actor string) error {
	res, err := s.db.ExecContext(ctx, `UPDATE entities SET display_name=?, name_locked=1 WHERE id=?`, name, entityID)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("entité introuvable")
	}
	return s.audit(ctx, actor, "rename", entityID+" -> "+name)
}
