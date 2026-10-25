package ingest

import "brightnet/internal/contract"

func Prepare(snap *contract.Snapshot) {
	if snap == nil {
		return
	}
	if snap.Version == "" {
		snap.Version = contract.Version
	}
	if snap.Docker == nil {
		return
	}
	for i := range snap.Docker.Containers {
		snap.Docker.Containers[i].Labels = FilterLabels(snap.Docker.Containers[i].Labels)
		snap.Docker.Containers[i].Name = trimSlash(snap.Docker.Containers[i].Name)
	}
	for i := range snap.Docker.Networks {
		if snap.Docker.Networks[i].Subnets == nil {
			snap.Docker.Networks[i].Subnets = []string{}
		}
	}
}

func trimSlash(name string) string {
	for len(name) > 0 && name[0] == '/' {
		name = name[1:]
	}
	return name
}

func RunStatus(runs []contract.CollectorRun, name string) string {
	for _, run := range runs {
		if run.Collector == name {
			if run.Status == "" {
				return "error"
			}
			return run.Status
		}
	}
	return "missing"
}
