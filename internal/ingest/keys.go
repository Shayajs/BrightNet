package ingest

import "strings"

func HostKey(agentID, machineID string) string {
	if machineID != "" {
		return "host:machine:" + machineID
	}
	return "host:agent:" + agentID
}

func ContainerKey(engineID, name string, labels map[string]string) (string, string) {
	project := labels["com.docker.compose.project"]
	service := labels["com.docker.compose.service"]
	if project != "" && service != "" {
		return "container:" + engineID + ":" + project + ":" + service, "configured"
	}
	return "container:" + engineID + ":name:" + name, "observed"
}

func FilterLabels(in map[string]string) map[string]string {
	if len(in) == 0 {
		return nil
	}
	allow := []string{
		"com.docker.compose.project",
		"com.docker.compose.service",
		"com.docker.compose.container-number",
	}
	out := map[string]string{}
	for _, key := range allow {
		if value, ok := in[key]; ok && value != "" && !looksSecret(key, value) {
			out[key] = value
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func looksSecret(key, value string) bool {
	k := strings.ToLower(key)
	if strings.Contains(k, "password") || strings.Contains(k, "secret") || strings.Contains(k, "token") {
		return true
	}
	return strings.Contains(strings.ToLower(value), "password=")
}
