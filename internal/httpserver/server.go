package httpserver

import (
	"encoding/json"
	"io"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"

	"brightnet/internal/bus"
	"brightnet/internal/contract"
	"brightnet/internal/store"
)

const sessionCookie = "brightnet_session"

type Server struct {
	Store     *store.Store
	Bus       *bus.Bus
	Downloads string
	limit     limiter
}

func New(st *store.Store, b *bus.Bus) *Server {
	return &Server{Store: st, Bus: b, limit: limiter{hits: map[string][]time.Time{}}}
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v1/health", s.health)
	mux.HandleFunc("GET /api/v1/downloads", s.downloads)
	mux.HandleFunc("GET /api/v1/downloads/{file}", s.downloadFile)
	mux.HandleFunc("GET /api/v1/setup/status", s.setupStatus)
	mux.HandleFunc("POST /api/v1/setup", s.setup)
	mux.HandleFunc("POST /api/v1/login", s.login)
	mux.HandleFunc("POST /api/v1/logout", s.logout)
	mux.HandleFunc("GET /api/v1/me", s.me)
	mux.HandleFunc("GET /api/v1/summary", s.summary)
	mux.HandleFunc("GET /api/v1/graph", s.graph)
	mux.HandleFunc("GET /api/v1/entities/{id}", s.entity)
	mux.HandleFunc("PATCH /api/v1/entities/{id}", s.rename)
	mux.HandleFunc("GET /api/v1/events", s.events)
	mux.HandleFunc("GET /api/v1/events/stream", s.stream)
	mux.HandleFunc("GET /api/v1/layouts/{view}", s.getLayout)
	mux.HandleFunc("PUT /api/v1/layouts/{view}", s.putLayout)
	mux.HandleFunc("POST /api/v1/pairing-codes", s.pairing)
	mux.HandleFunc("POST /api/v1/fixtures/load", s.fixture)
	mux.HandleFunc("POST /api/v1/agents/enroll", s.enroll)
	mux.HandleFunc("POST /api/v1/agents/{id}/heartbeat", s.heartbeat)
	mux.HandleFunc("POST /api/v1/agents/{id}/snapshot", s.snapshot)
	mux.HandleFunc("POST /api/v1/agents/{id}/events", s.dockerEvents)
	return mux
}

func (s *Server) health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) setupStatus(w http.ResponseWriter, r *http.Request) {
	needs, err := s.Store.NeedsSetup(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "état illisible")
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"needs_setup": needs})
}

func (s *Server) setup(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if !decode(w, r, &body) {
		return
	}
	if len(body.Username) < 2 || len(body.Password) < 8 {
		writeError(w, http.StatusBadRequest, "identifiant ou mot de passe trop court")
		return
	}
	if err := s.Store.CreateUser(r.Context(), body.Username, body.Password); err != nil {
		writeError(w, http.StatusConflict, "configuration déjà effectuée")
		return
	}
	writeJSON(w, http.StatusCreated, map[string]string{"status": "ok"})
}

func (s *Server) login(w http.ResponseWriter, r *http.Request) {
	if !s.limit.allow(clientIP(r)+":login", 8, time.Minute) {
		writeError(w, http.StatusTooManyRequests, "trop de tentatives")
		return
	}
	var body struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if !decode(w, r, &body) {
		return
	}
	token, err := s.Store.Login(r.Context(), body.Username, body.Password)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "identifiants invalides")
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name: sessionCookie, Value: token, Path: "/", HttpOnly: true,
		SameSite: http.SameSiteLaxMode, Expires: time.Now().Add(12 * time.Hour),
	})
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) logout(w http.ResponseWriter, r *http.Request) {
	if c, err := r.Cookie(sessionCookie); err == nil {
		_ = s.Store.Logout(r.Context(), c.Value)
	}
	http.SetCookie(w, &http.Cookie{Name: sessionCookie, Value: "", Path: "/", HttpOnly: true, MaxAge: -1})
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) me(w http.ResponseWriter, r *http.Request) {
	_, username, ok := s.user(w, r)
	if !ok {
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"username": username})
}

func (s *Server) summary(w http.ResponseWriter, r *http.Request) {
	if _, _, ok := s.user(w, r); !ok {
		return
	}
	summary, err := s.Store.Summary(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "résumé indisponible")
		return
	}
	writeJSON(w, http.StatusOK, summary)
}

func (s *Server) graph(w http.ResponseWriter, r *http.Request) {
	if _, _, ok := s.user(w, r); !ok {
		return
	}
	view := r.URL.Query().Get("view")
	if view == "" {
		view = "network"
	}
	graph, err := s.Store.Graph(r.Context(), view, r.URL.Query().Get("focus"))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "graphe indisponible")
		return
	}
	writeJSON(w, http.StatusOK, graph)
}

func (s *Server) entity(w http.ResponseWriter, r *http.Request) {
	if _, _, ok := s.user(w, r); !ok {
		return
	}
	node, edges, err := s.Store.Entity(r.Context(), r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusNotFound, "entité introuvable")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"entity": node, "relations": edges})
}

func (s *Server) rename(w http.ResponseWriter, r *http.Request) {
	_, username, ok := s.user(w, r)
	if !ok {
		return
	}
	var body struct {
		DisplayName string `json:"display_name"`
	}
	if !decode(w, r, &body) {
		return
	}
	name := strings.TrimSpace(body.DisplayName)
	if !validName(name) {
		writeError(w, http.StatusBadRequest, "nom d'affichage invalide")
		return
	}
	if err := s.Store.RenameEntity(r.Context(), r.PathValue("id"), name, username); err != nil {
		writeError(w, http.StatusNotFound, "entité introuvable")
		return
	}
	s.Bus.Publish(map[string]any{"type": "entity.updated", "id": r.PathValue("id"), "label": name})
	s.Bus.Publish(map[string]string{"type": "graph.changed"})
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) events(w http.ResponseWriter, r *http.Request) {
	if _, _, ok := s.user(w, r); !ok {
		return
	}
	items, err := s.Store.ListEvents(r.Context(), 50)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "événements indisponibles")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"events": items})
}

func (s *Server) stream(w http.ResponseWriter, r *http.Request) {
	if _, _, ok := s.user(w, r); !ok {
		return
	}
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, http.StatusInternalServerError, "flux indisponible")
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	var since uint64
	if raw := r.Header.Get("Last-Event-ID"); raw != "" {
		since, _ = strconv.ParseUint(raw, 10, 64)
	}
	past, ch, cancel := s.Bus.Subscribe(since)
	defer cancel()
	_, _ = io.WriteString(w, ": ok\n\n")
	flusher.Flush()
	writeEvent := func(ev bus.Event) {
		_, _ = io.WriteString(w, "id: ")
		_, _ = io.WriteString(w, strconv.FormatUint(ev.ID, 10))
		_, _ = io.WriteString(w, "\ndata: ")
		_, _ = w.Write(ev.Body)
		_, _ = io.WriteString(w, "\n\n")
		flusher.Flush()
	}
	for _, ev := range past {
		writeEvent(ev)
	}
	ping := time.NewTicker(20 * time.Second)
	defer ping.Stop()
	for {
		select {
		case <-r.Context().Done():
			return
		case ev, ok := <-ch:
			if !ok {
				return
			}
			writeEvent(ev)
		case <-ping.C:
			_, _ = io.WriteString(w, ": ping\n\n")
			flusher.Flush()
		}
	}
}

func (s *Server) getLayout(w http.ResponseWriter, r *http.Request) {
	userID, _, ok := s.user(w, r)
	if !ok {
		return
	}
	view := r.PathValue("view")
	if !knownView(view) {
		writeError(w, http.StatusBadRequest, "vue inconnue")
		return
	}
	raw, err := s.Store.LoadLayout(r.Context(), userID, view)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "disposition illisible")
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_, _ = io.WriteString(w, raw)
}

func (s *Server) putLayout(w http.ResponseWriter, r *http.Request) {
	userID, _, ok := s.user(w, r)
	if !ok {
		return
	}
	view := r.PathValue("view")
	if !knownView(view) {
		writeError(w, http.StatusBadRequest, "vue inconnue")
		return
	}
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 512*1024))
	if err != nil {
		writeError(w, http.StatusBadRequest, "disposition trop volumineuse")
		return
	}
	var positions map[string]struct {
		X float64 `json:"x"`
		Y float64 `json:"y"`
	}
	if err := json.Unmarshal(body, &positions); err != nil || positions == nil {
		writeError(w, http.StatusBadRequest, "disposition invalide")
		return
	}
	if err := s.Store.SaveLayout(r.Context(), userID, view, string(body)); err != nil {
		writeError(w, http.StatusInternalServerError, "disposition non enregistrée")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) pairing(w http.ResponseWriter, r *http.Request) {
	_, username, ok := s.user(w, r)
	if !ok {
		return
	}
	code, expires, err := s.Store.CreatePairingCode(r.Context(), username)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "code non créé")
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"pairing_code": code, "expires_at": expires})
}

func (s *Server) fixture(w http.ResponseWriter, r *http.Request) {
	_, username, ok := s.user(w, r)
	if !ok {
		return
	}
	result, err := s.Store.LoadFixture(r.Context(), username)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "démonstration non chargée")
		return
	}
	s.publish(result)
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok", "data_origin": "fixture"})
}

func (s *Server) enroll(w http.ResponseWriter, r *http.Request) {
	if !s.limit.allow(clientIP(r)+":enroll", 8, time.Minute) {
		writeError(w, http.StatusTooManyRequests, "trop de tentatives")
		return
	}
	var body contract.EnrollRequest
	if !decode(w, r, &body) {
		return
	}
	if strings.TrimSpace(body.PairingCode) == "" {
		writeError(w, http.StatusBadRequest, "code d'appairage requis")
		return
	}
	agentID, token, err := s.Store.Enroll(r.Context(), body)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "code d'appairage invalide")
		return
	}
	writeJSON(w, http.StatusCreated, contract.NodeAck(agentID, token))
}

func (s *Server) heartbeat(w http.ResponseWriter, r *http.Request) {
	if !s.agent(w, r) {
		return
	}
	if err := s.Store.TouchAgent(r.Context(), r.PathValue("id")); err != nil {
		writeError(w, http.StatusInternalServerError, "battement refusé")
		return
	}
	writeJSON(w, http.StatusOK, contract.NodeAck("", ""))
}

func (s *Server) snapshot(w http.ResponseWriter, r *http.Request) {
	if !s.agent(w, r) {
		return
	}
	var body contract.Snapshot
	if !decode(w, r, &body) {
		return
	}
	result, err := s.Store.ApplySnapshot(r.Context(), r.PathValue("id"), body)
	if err != nil {
		writeError(w, http.StatusBadRequest, "instantané refusé")
		return
	}
	s.publish(result)
	writeJSON(w, http.StatusOK, contract.NodeAck("", ""))
}

func (s *Server) dockerEvents(w http.ResponseWriter, r *http.Request) {
	if !s.agent(w, r) {
		return
	}
	var body contract.EventBatch
	if !decode(w, r, &body) {
		return
	}
	if len(body.Events) > 100 {
		writeError(w, http.StatusBadRequest, "trop d'événements")
		return
	}
	result, err := s.Store.ApplyDockerEvents(r.Context(), r.PathValue("id"), body.Events)
	if err != nil {
		writeError(w, http.StatusBadRequest, "événements refusés")
		return
	}
	s.publish(result)
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) publish(result store.ApplyResult) {
	for _, activity := range result.Activities {
		s.Bus.Publish(map[string]any{"type": "activity", "event": activity})
	}
	if result.Changed {
		s.Bus.Publish(map[string]string{"type": "graph.changed"})
	}
}

func (s *Server) user(w http.ResponseWriter, r *http.Request) (string, string, bool) {
	cookie, err := r.Cookie(sessionCookie)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "authentification requise")
		return "", "", false
	}
	userID, username, err := s.Store.UserFromToken(r.Context(), cookie.Value)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "authentification requise")
		return "", "", false
	}
	return userID, username, true
}

func (s *Server) agent(w http.ResponseWriter, r *http.Request) bool {
	header := r.Header.Get("Authorization")
	token, ok := strings.CutPrefix(header, "Bearer ")
	if !ok || token == "" || strings.Contains(token, " ") {
		writeError(w, http.StatusUnauthorized, "jeton d'agent requis")
		return false
	}
	if err := s.Store.AuthenticateAgent(r.Context(), r.PathValue("id"), token); err != nil {
		writeError(w, http.StatusUnauthorized, "jeton d'agent invalide")
		return false
	}
	return true
}

func decode(w http.ResponseWriter, r *http.Request, dest any) bool {
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 2<<20))
	dec.DisallowUnknownFields()
	if err := dec.Decode(dest); err != nil {
		writeError(w, http.StatusBadRequest, "requête invalide")
		return false
	}
	return true
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}

func knownView(view string) bool {
	switch view {
	case "docker", "physical", "logical", "network", "complex":
		return true
	default:
		return false
	}
}

func validName(name string) bool {
	if len([]rune(name)) < 1 || len([]rune(name)) > 80 {
		return false
	}
	for _, r := range name {
		if unicode.IsControl(r) {
			return false
		}
	}
	return true
}

func clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
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
	for _, ts := range l.hits[key] {
		if now.Sub(ts) < window {
			kept = append(kept, ts)
		}
	}
	if len(kept) >= max {
		l.hits[key] = kept
		return false
	}
	l.hits[key] = append(kept, now)
	return true
}
