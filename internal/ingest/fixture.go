package ingest

import (
	"time"

	"brightnet/internal/contract"
)

func Fixture(now time.Time) contract.Snapshot {
	return contract.Snapshot{
		Version:     contract.Version,
		CollectedAt: now,
		Simulated:   true,
		Host: contract.HostSnapshot{
			Hostname:         "maison-simulee",
			OS:               "linux",
			OSVersion:        "fixture",
			Arch:             "amd64",
			Kernel:           "fixture",
			MachineID:        "fixture-machine",
			Virtualization:   "fixture",
			ObservationScope: "fixture",
			Interfaces: []contract.InterfaceSnapshot{{
				Name:  "eth0",
				MAC:   "02:00:00:00:00:01",
				MTU:   1500,
				Flags: "up|broadcast",
				Addrs: []string{"198.51.100.10/24"},
			}},
		},
		Docker: &contract.DockerSnapshot{
			Available: true,
			Engine: &contract.EngineSnapshot{
				ID:              "fixture-engine",
				Name:            "moteur-simulé",
				Version:         "0",
				OS:              "linux",
				Arch:            "amd64",
				OperatingSystem: "fixture",
			},
			Networks: []contract.NetworkSnapshot{{
				ID: "fixture-net", Name: "demo-net", Driver: "bridge", Scope: "local", Subnets: []string{"198.51.100.0/24"},
			}},
			Volumes: []contract.VolumeSnapshot{{
				Name: "demo-data", Driver: "local", Scope: "local",
			}},
			Containers: []contract.ContainerSnapshot{{
				ID: "fixture-container", Name: "demo-web", Image: "fixture/demo:latest", State: "running", StatusText: "simulation",
				Labels: map[string]string{
					"com.docker.compose.project": "demo",
					"com.docker.compose.service": "web",
					"password":                   "ne-pas-conserver",
				},
				Ports: []contract.PortSnapshot{{PrivatePort: 80, PublicPort: 8080, Type: "tcp"}},
				Networks: []contract.ContainerNet{{
					Name: "demo-net", NetworkID: "fixture-net", IPAddress: "198.51.100.20",
				}},
				Mounts: []contract.MountSnapshot{{
					Type: "volume", Name: "demo-data", Destination: "/data", RW: true,
				}},
			}},
		},
		CollectorRuns: []contract.CollectorRun{
			{Collector: "host", Status: "ok"},
			{Collector: "docker", Status: "ok"},
		},
	}
}
