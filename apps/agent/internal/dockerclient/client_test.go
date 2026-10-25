package dockerclient

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestConvertDropsSecretsAndHostPaths(t *testing.T) {
	raw := []byte(`[{
		"Id":"abc","Names":["/brightnet-signal"],"Image":"nginx:alpine","State":"running","Status":"Up",
		"Labels":{"password":"secret","com.docker.compose.project":"brightnet-signal","com.docker.compose.service":"signal"},
		"Env":["PASSWORD=super"],
		"Ports":[{"PrivatePort":80,"PublicPort":8080,"Type":"tcp"}],
		"NetworkSettings":{"Networks":{"bridge":{"NetworkID":"n1","IPAddress":"172.17.0.2"}}},
		"Mounts":[{"Type":"bind","Source":"/secret/path","Name":"","Destination":"/data","RW":true}]
	}]`)
	var parsed []apiContainer
	if err := json.Unmarshal(raw, &parsed); err != nil {
		t.Fatal(err)
	}
	got := convertContainer(parsed[0])
	encoded, _ := json.Marshal(got)
	text := string(encoded)
	for _, forbidden := range []string{"PASSWORD", "secret", "/secret/path"} {
		if strings.Contains(text, forbidden) {
			t.Fatalf("donnée sensible conservée (%s): %s", forbidden, text)
		}
	}
	if got.Name != "/brightnet-signal" && got.Name != "brightnet-signal" {
		t.Fatalf("nom %s", got.Name)
	}
	if got.Labels["com.docker.compose.service"] != "signal" {
		t.Fatalf("label Compose perdu: %#v", got.Labels)
	}
}
