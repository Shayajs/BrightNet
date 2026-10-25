package httpserver

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"brightnet/internal/contract"
	"brightnet/internal/hub"
	"brightnet/internal/store"
)

func newServer(t *testing.T) (*httptest.Server, *store.Store) {
	t.Helper()
	st, err := store.Open(filepath.Join(t.TempDir(), "brightnet.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	if err := st.EnsureAdmin(t.Context(), "admin", "brightnet-dev"); err != nil {
		t.Fatal(err)
	}
	if err := st.UpsertDevPairingCode(t.Context(), "pair-test", time.Hour); err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(New(st, hub.New()).Handler())
	t.Cleanup(srv.Close)
	return srv, st
}

func login(t *testing.T, base string) *http.Cookie {
	t.Helper()
	res := post(t, base+"/api/v1/login", nil, map[string]string{"username": "admin", "password": "brightnet-dev"})
	if res.StatusCode != http.StatusOK {
		t.Fatalf("login %d", res.StatusCode)
	}
	for _, cookie := range res.Cookies() {
		if cookie.Name == sessionCookie {
			return cookie
		}
	}
	t.Fatal("cookie absent")
	return nil
}

func enroll(t *testing.T, base string) (string, string) {
	t.Helper()
	res := post(t, base+"/api/v1/agents/enroll", nil, contract.EnrollRequest{PairingCode: "pair-test", Hostname: "dev", Version: "test"})
	if res.StatusCode != http.StatusCreated {
		t.Fatalf("enroll %d", res.StatusCode)
	}
	var body contract.AgentConfig
	readJSON(t, res, &body)
	return body.AgentID, body.Token
}

func post(t *testing.T, url string, cookie *http.Cookie, body any) *http.Response {
	t.Helper()
	return send(t, http.MethodPost, url, cookie, "", body)
}

func send(t *testing.T, method, url string, cookie *http.Cookie, bearer string, body any) *http.Response {
	t.Helper()
	var buf bytes.Buffer
	if body != nil {
		if err := json.NewEncoder(&buf).Encode(body); err != nil {
			t.Fatal(err)
		}
	}
	req, err := http.NewRequest(method, url, &buf)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	if cookie != nil {
		req.AddCookie(cookie)
	}
	if bearer != "" {
		req.Header.Set("Authorization", "Bearer "+bearer)
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = res.Body.Close() })
	return res
}

func readJSON(t *testing.T, res *http.Response, dest any) {
	t.Helper()
	if err := json.NewDecoder(res.Body).Decode(dest); err != nil {
		t.Fatal(err)
	}
}

func snapshot(hostIP, containerID, state string) contract.Snapshot {
	return contract.Snapshot{
		Version: contract.Version, CollectedAt: time.Now().UTC(),
		Host: contract.HostSnapshot{
			Hostname: "poste", OS: "linux", Arch: "amd64", MachineID: "mid-" + hostIP,
			ObservationScope: "host", Virtualization: "wsl2",
			Interfaces: []contract.InterfaceSnapshot{{
				Name: "eth0", MAC: "02:00:00:00:00:11", MTU: 1500, Flags: "up|broadcast",
				Addrs: []string{hostIP + "/24"},
			}},
		},
		Docker: &contract.DockerSnapshot{
			Available: true,
			Engine:    &contract.EngineSnapshot{ID: "engine-1", Name: "docker", Version: "29", OS: "linux", Arch: "amd64"},
			Networks:  []contract.NetworkSnapshot{{ID: "net-1", Name: "bridge", Driver: "bridge", Subnets: []string{"172.17.0.0/16"}}},
			Volumes:   []contract.VolumeSnapshot{{Name: "data", Driver: "local", Scope: "local"}},
			Containers: []contract.ContainerSnapshot{{
				ID: containerID, Name: "brightnet-signal", Image: "nginx:alpine", State: state, StatusText: state,
				Labels:   map[string]string{"com.docker.compose.project": "brightnet-signal", "com.docker.compose.service": "signal", "password": "secret"},
				Ports:    []contract.PortSnapshot{{PrivatePort: 80, PublicPort: 8080, Type: "tcp"}},
				Networks: []contract.ContainerNet{{Name: "bridge", NetworkID: "net-1", IPAddress: "172.17.0.2"}},
				Mounts:   []contract.MountSnapshot{{Type: "volume", Name: "data", Destination: "/usr/share/nginx/html", RW: false}},
			}},
		},
		CollectorRuns: []contract.CollectorRun{{Collector: "host", Status: "ok"}, {Collector: "docker", Status: "ok"}},
	}
}

func TestAuthAndIngest(t *testing.T) {
	srv, _ := newServer(t)
	res := send(t, http.MethodGet, srv.URL+"/api/v1/graph", nil, "", nil)
	if res.StatusCode != http.StatusUnauthorized {
		t.Fatalf("graphe sans session: %d", res.StatusCode)
	}
	res = post(t, srv.URL+"/api/v1/agents/enroll", nil, contract.EnrollRequest{PairingCode: "mauvais"})
	if res.StatusCode != http.StatusUnauthorized {
		t.Fatalf("mauvais code: %d", res.StatusCode)
	}
	agentID, token := enroll(t, srv.URL)
	res = send(t, http.MethodPost, srv.URL+"/api/v1/agents/"+agentID+"/snapshot", nil, "", snapshot("192.0.2.10", "abc", "running"))
	if res.StatusCode != http.StatusUnauthorized {
		t.Fatalf("instantané sans jeton: %d", res.StatusCode)
	}
	raw := map[string]any{"version": contract.Version, "host": map[string]any{"hostname": "poste", "interfaces": []any{}}, "docker": map[string]any{"available": true, "containers": []any{map[string]any{"id": "abc", "name": "x", "env": []string{"PASSWORD=secret"}}}}, "collector_runs": []any{}}
	res = send(t, http.MethodPost, srv.URL+"/api/v1/agents/"+agentID+"/snapshot", nil, token, raw)
	if res.StatusCode != http.StatusBadRequest {
		t.Fatalf("champ env accepté: %d", res.StatusCode)
	}
	res = send(t, http.MethodPost, srv.URL+"/api/v1/agents/"+agentID+"/snapshot", nil, token, snapshot("192.0.2.10", "container-old", "running"))
	if res.StatusCode != http.StatusOK {
		t.Fatalf("instantané: %d", res.StatusCode)
	}
	res = send(t, http.MethodPost, srv.URL+"/api/v1/agents/"+agentID+"/snapshot", nil, token, snapshot("192.0.2.10", "container-new", "exited"))
	if res.StatusCode != http.StatusOK {
		t.Fatalf("recréation: %d", res.StatusCode)
	}

	cookie := login(t, srv.URL)
	res = send(t, http.MethodGet, srv.URL+"/api/v1/graph?view=docker", cookie, "", nil)
	var graph contract.Graph
	readJSON(t, res, &graph)
	if graph.DataOrigin != "live" {
		t.Fatalf("origine %s", graph.DataOrigin)
	}
	var containers int
	var sawSecret bool
	for _, node := range graph.Nodes {
		if node.Kind == "docker_container" {
			containers++
			if node.Status != "exited" {
				t.Fatalf("état %s", node.Status)
			}
			encoded, _ := json.Marshal(node.Properties)
			if bytes.Contains(encoded, []byte("secret")) || bytes.Contains(encoded, []byte("PASSWORD")) {
				sawSecret = true
			}
			if node.Properties["container_id"] != "container-new" {
				t.Fatalf("identifiant courant %v", node.Properties["container_id"])
			}
		}
	}
	if containers != 1 {
		t.Fatalf("conteneurs = %d", containers)
	}
	if sawSecret {
		t.Fatal("secret présent dans le graphe")
	}

	broken := snapshot("192.0.2.10", "container-new", "running")
	broken.Docker.Available = false
	broken.Docker.Error = "socket indisponible"
	broken.Docker.Containers = nil
	broken.CollectorRuns[1].Status = "error"
	res = send(t, http.MethodPost, srv.URL+"/api/v1/agents/"+agentID+"/snapshot", nil, token, broken)
	if res.StatusCode != http.StatusOK {
		t.Fatalf("collecteur en erreur: %d", res.StatusCode)
	}
	res = send(t, http.MethodGet, srv.URL+"/api/v1/graph?view=docker", cookie, "", nil)
	readJSON(t, res, &graph)
	containers = 0
	for _, node := range graph.Nodes {
		if node.Kind == "docker_container" {
			containers++
			if node.Status == "absent" {
				t.Fatal("une panne de collecteur ne doit pas faire disparaître le conteneur")
			}
		}
	}
	if containers != 1 {
		t.Fatalf("conteneur perdu: %d", containers)
	}
}

func TestSameIPDoesNotMergeHosts(t *testing.T) {
	srv, _ := newServer(t)
	// Le code de développement est réutilisable : deux agents peuvent s'inscrire.
	firstID, firstToken := enroll(t, srv.URL)
	secondID, secondToken := enroll(t, srv.URL)
	for _, item := range []struct{ id, token string }{{firstID, firstToken}, {secondID, secondToken}} {
		body := snapshot("192.0.2.10", "c-"+item.id, "running")
		body.Host.MachineID = "machine-" + item.id
		res := send(t, http.MethodPost, srv.URL+"/api/v1/agents/"+item.id+"/snapshot", nil, item.token, body)
		if res.StatusCode != http.StatusOK {
			t.Fatalf("instantané %s: %d", item.id, res.StatusCode)
		}
	}
	cookie := login(t, srv.URL)
	res := send(t, http.MethodGet, srv.URL+"/api/v1/graph?view=logical", cookie, "", nil)
	var graph contract.Graph
	readJSON(t, res, &graph)
	hosts, addrs := 0, 0
	for _, node := range graph.Nodes {
		if node.Kind == "host" && !node.Simulated {
			hosts++
		}
		if node.Kind == "address" && node.Label == "192.0.2.10" {
			addrs++
		}
	}
	if hosts != 2 || addrs != 2 {
		t.Fatalf("hôtes=%d adresses=%d, une IP identique ne doit pas fusionner", hosts, addrs)
	}
}

func TestFixtureIsMarked(t *testing.T) {
	srv, _ := newServer(t)
	cookie := login(t, srv.URL)
	res := send(t, http.MethodPost, srv.URL+"/api/v1/fixtures/load", cookie, "", map[string]any{})
	if res.StatusCode != http.StatusOK {
		t.Fatalf("fixture %d", res.StatusCode)
	}
	res = send(t, http.MethodGet, srv.URL+"/api/v1/graph?view=docker", cookie, "", nil)
	var graph contract.Graph
	readJSON(t, res, &graph)
	if graph.DataOrigin != "fixture" {
		t.Fatalf("origine %s", graph.DataOrigin)
	}
	for _, node := range graph.Nodes {
		if !node.Simulated {
			t.Fatalf("nœud non simulé: %s", node.Label)
		}
		encoded, _ := json.Marshal(node.Properties)
		if bytes.Contains(encoded, []byte("ne-pas-conserver")) {
			t.Fatal("label secret de fixture conservé")
		}
	}
}

func TestDNSViewEmpty(t *testing.T) {
	srv, _ := newServer(t)
	cookie := login(t, srv.URL)
	res := send(t, http.MethodGet, srv.URL+"/api/v1/graph?view=dns", cookie, "", nil)
	var graph contract.Graph
	readJSON(t, res, &graph)
	if graph.Available || len(graph.Nodes) != 0 {
		t.Fatalf("la vue DNS ne doit pas être alimentée: %+v", graph)
	}
}
