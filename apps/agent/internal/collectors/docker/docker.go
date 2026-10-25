package docker

import (
	"context"
	"os"

	"brightnet/apps/agent/internal/dockerclient"
	"brightnet/internal/contract"
)

func Collect(ctx context.Context, socketPath string) (contract.DockerSnapshot, contract.CollectorRun) {
	defer func() {
		if rec := recover(); rec != nil {
		}
	}()
	if socketPath == "" {
		socketPath = "/var/run/docker.sock"
	}
	if _, err := os.Stat(socketPath); err != nil {
		return contract.DockerSnapshot{Available: false, Error: err.Error()}, contract.CollectorRun{Collector: "docker", Status: "error", Error: err.Error()}
	}
	client := dockerclient.New(socketPath)
	engine, err := client.Info(ctx)
	if err != nil {
		return contract.DockerSnapshot{Available: false, Error: err.Error()}, contract.CollectorRun{Collector: "docker", Status: "error", Error: err.Error()}
	}
	containers, err := client.Containers(ctx)
	if err != nil {
		return contract.DockerSnapshot{Available: false, Error: err.Error()}, contract.CollectorRun{Collector: "docker", Status: "error", Error: err.Error()}
	}
	networks, err := client.Networks(ctx)
	if err != nil {
		return contract.DockerSnapshot{Available: false, Error: err.Error()}, contract.CollectorRun{Collector: "docker", Status: "error", Error: err.Error()}
	}
	volumes, err := client.Volumes(ctx)
	if err != nil {
		return contract.DockerSnapshot{Available: false, Error: err.Error()}, contract.CollectorRun{Collector: "docker", Status: "error", Error: err.Error()}
	}
	return contract.DockerSnapshot{
		Available: true, Engine: &engine, Containers: containers, Networks: networks, Volumes: volumes,
	}, contract.CollectorRun{Collector: "docker", Status: "ok"}
}
