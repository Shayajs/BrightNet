package ingest

import "testing"

func TestHostKeyGroupsMachine(t *testing.T) {
	if HostKey("a", "machine") != HostKey("b", "machine") {
		t.Fatal("deux Node avec le même machine-id doivent partager l'hôte")
	}
	if HostKey("a", "") == HostKey("b", "") {
		t.Fatal("sans machine-id, chaque Node reste un hôte")
	}
	if RouterKey("a", "machine", "192.0.2.1") != RouterKey("b", "machine", "192.0.2.1") {
		t.Fatal("la passerelle d'une machine ne doit pas dépendre du Node")
	}
	if RouterKey("a", "machine", "192.0.2.1") == RouterKey("a", "autre", "192.0.2.1") {
		t.Fatal("la même adresse sur deux machines reste deux routeurs")
	}
}

func TestContainerKeyStableAcrossRecreate(t *testing.T) {
	labels := map[string]string{
		"com.docker.compose.project": "brightnet-signal",
		"com.docker.compose.service": "signal",
	}
	first, evidence := ContainerKey("engine", "brightnet-signal", labels)
	second, _ := ContainerKey("engine", "brightnet-signal-recreated", labels)
	if first != second {
		t.Fatalf("la recréation Compose doit conserver la clé: %s vs %s", first, second)
	}
	if evidence != "configured" {
		t.Fatalf("evidence = %s", evidence)
	}
	other, observed := ContainerKey("engine", "lonely", nil)
	if other == first {
		t.Fatal("un conteneur sans Compose ne doit pas partager la clé")
	}
	if observed != "observed" {
		t.Fatalf("evidence = %s", observed)
	}
}

func TestFilterLabelsDropsSecrets(t *testing.T) {
	got := FilterLabels(map[string]string{
		"com.docker.compose.project": "p",
		"com.docker.compose.service": "web",
		"password":                   "secret",
		"traefik.enable":             "true",
	})
	if got["com.docker.compose.project"] != "p" || got["com.docker.compose.service"] != "web" {
		t.Fatalf("labels Compose perdues: %#v", got)
	}
	if _, ok := got["password"]; ok {
		t.Fatal("mot de passe conservé")
	}
	if _, ok := got["traefik.enable"]; ok {
		t.Fatal("label arbitraire conservé")
	}
}
