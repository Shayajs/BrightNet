package ingest

import "testing"

func TestHostKeyGroupsMachine(t *testing.T) {
	if HostKey("a", "machine") != HostKey("b", "machine") {
		t.Fatal("deux Node avec le même machine-id doivent partager l'hôte")
	}
	if HostKey("a", "") == HostKey("b", "") {
		t.Fatal("sans machine-id, chaque Node reste un hôte")
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
