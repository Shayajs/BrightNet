package contract

import "time"

const Version = "brightnet.ingest.v1"

type Snapshot struct {
	Version       string           `json:"version"`
	CollectedAt   time.Time        `json:"collected_at"`
	Simulated     bool             `json:"simulated"`
	Host          HostSnapshot     `json:"host"`
	Devices       []DeviceSnapshot `json:"devices,omitempty"`
	Docker        *DockerSnapshot  `json:"docker,omitempty"`
	CollectorRuns []CollectorRun   `json:"collector_runs"`
}

type HostSnapshot struct {
	Hostname         string              `json:"hostname"`
	OS               string              `json:"os"`
	OSVersion        string              `json:"os_version"`
	Arch             string              `json:"arch"`
	Kernel           string              `json:"kernel"`
	MachineID        string              `json:"machine_id"`
	Virtualization   string              `json:"virtualization"`
	ObservationScope string              `json:"observation_scope"`
	Interfaces       []InterfaceSnapshot `json:"interfaces"`
	Gateways         []string            `json:"gateways,omitempty"`
	DHCP             string              `json:"dhcp,omitempty"`
	Networks         []NetworkPrefix     `json:"networks,omitempty"`
}

type NetworkPrefix struct {
	Name string `json:"name"`
	CIDR string `json:"cidr"`
	DHCP string `json:"dhcp,omitempty"`
}

type DeviceSnapshot struct {
	Name   string `json:"name"`
	IP     string `json:"ip,omitempty"`
	MAC    string `json:"mac,omitempty"`
	Source string `json:"source"`
	Detail string `json:"detail,omitempty"`
}

type InterfaceSnapshot struct {
	Name  string   `json:"name"`
	MAC   string   `json:"mac"`
	MTU   int      `json:"mtu"`
	Flags string   `json:"flags"`
	Addrs []string `json:"addrs"`
}

type DockerSnapshot struct {
	Available  bool                `json:"available"`
	Error      string              `json:"error,omitempty"`
	Engine     *EngineSnapshot     `json:"engine,omitempty"`
	Containers []ContainerSnapshot `json:"containers,omitempty"`
	Networks   []NetworkSnapshot   `json:"networks,omitempty"`
	Volumes    []VolumeSnapshot    `json:"volumes,omitempty"`
}

type EngineSnapshot struct {
	ID              string `json:"id"`
	Name            string `json:"name"`
	Version         string `json:"version"`
	OS              string `json:"os"`
	Arch            string `json:"arch"`
	OperatingSystem string `json:"operating_system"`
	CPUs            int    `json:"cpus"`
	MemoryBytes     int64  `json:"memory_bytes"`
}

type ContainerSnapshot struct {
	ID         string            `json:"id"`
	Name       string            `json:"name"`
	Image      string            `json:"image"`
	ImageID    string            `json:"image_id"`
	State      string            `json:"state"`
	StatusText string            `json:"status_text"`
	Labels     map[string]string `json:"labels,omitempty"`
	Ports      []PortSnapshot    `json:"ports,omitempty"`
	Networks   []ContainerNet    `json:"networks,omitempty"`
	Mounts     []MountSnapshot   `json:"mounts,omitempty"`
}

type PortSnapshot struct {
	PrivatePort int    `json:"private_port"`
	PublicPort  int    `json:"public_port"`
	Type        string `json:"type"`
	IP          string `json:"ip,omitempty"`
}

type ContainerNet struct {
	Name       string `json:"name"`
	NetworkID  string `json:"network_id"`
	IPAddress  string `json:"ip_address"`
	IPv6       string `json:"ipv6,omitempty"`
	MacAddress string `json:"mac_address"`
}

type MountSnapshot struct {
	Type        string `json:"type"`
	Name        string `json:"name"`
	Destination string `json:"destination"`
	RW          bool   `json:"rw"`
}

type NetworkSnapshot struct {
	ID      string   `json:"id"`
	Name    string   `json:"name"`
	Driver  string   `json:"driver"`
	Scope   string   `json:"scope"`
	Subnets []string `json:"subnets"`
}

type VolumeSnapshot struct {
	Name   string `json:"name"`
	Driver string `json:"driver"`
	Scope  string `json:"scope"`
}

type CollectorRun struct {
	Collector string `json:"collector"`
	Status    string `json:"status"`
	Error     string `json:"error,omitempty"`
}

type DockerEvent struct {
	Type       string            `json:"type"`
	Action     string            `json:"action"`
	ActorID    string            `json:"actor_id"`
	ActorName  string            `json:"actor_name"`
	Time       time.Time         `json:"time"`
	Attributes map[string]string `json:"attributes,omitempty"`
}

func (event DockerEvent) Relevant() bool {
	switch event.Action {
	case "create", "start", "die", "destroy", "kill", "stop", "pause", "unpause", "restart", "remove", "oom", "connect", "disconnect", "mount", "unmount":
		return true
	default:
		return false
	}
}

type EventBatch struct {
	Events []DockerEvent `json:"events"`
}

type EnrollRequest struct {
	PairingCode string `json:"pairing_code"`
	Hostname    string `json:"hostname"`
	MachineID   string `json:"machine_id"`
	Version     string `json:"version"`
}

type AgentConfig struct {
	AgentID                 string   `json:"agent_id,omitempty"`
	Token                   string   `json:"token,omitempty"`
	Collectors              []string `json:"collectors"`
	SnapshotIntervalSeconds int      `json:"snapshot_interval_seconds"`
	Peers                   []string `json:"peers,omitempty"`
	PeerNames               []string `json:"peer_names,omitempty"`
}

func NodeAck(agentID, token string) AgentConfig {
	cfg := AgentConfig{
		Collectors:              []string{"host", "docker"},
		SnapshotIntervalSeconds: 20,
	}
	if agentID != "" {
		cfg.AgentID = agentID
		cfg.Token = token
	}
	return cfg
}

type Activity struct {
	ID         string    `json:"id"`
	Kind       string    `json:"kind"`
	Message    string    `json:"message"`
	EntityID   string    `json:"entity_id,omitempty"`
	Simulated  bool      `json:"simulated"`
	ObservedAt time.Time `json:"observed_at"`
}

type GraphNode struct {
	ID           string         `json:"id"`
	Kind         string         `json:"kind"`
	Label        string         `json:"label"`
	ParentID     string         `json:"parent_id,omitempty"`
	Status       string         `json:"status"`
	Simulated    bool           `json:"simulated"`
	TechnicalKey string         `json:"technical_key"`
	Properties   map[string]any `json:"properties"`
	LastSeenAt   time.Time      `json:"last_seen_at"`
}

type GraphEdge struct {
	ID         string         `json:"id"`
	Source     string         `json:"source"`
	Target     string         `json:"target"`
	Kind       string         `json:"kind"`
	Evidence   string         `json:"evidence"`
	Confidence float64        `json:"confidence"`
	Status     string         `json:"status"`
	Properties map[string]any `json:"properties"`
	LastSeenAt time.Time      `json:"last_seen_at"`
}

type Graph struct {
	View        string      `json:"view"`
	Available   bool        `json:"available"`
	GeneratedAt time.Time   `json:"generated_at"`
	DataOrigin  string      `json:"data_origin"`
	Notice      string      `json:"notice,omitempty"`
	Nodes       []GraphNode `json:"nodes"`
	Edges       []GraphEdge `json:"edges"`
}

type AgentInfo struct {
	ID         string     `json:"id"`
	Hostname   string     `json:"hostname"`
	Version    string     `json:"version"`
	LastSeenAt *time.Time `json:"last_seen_at,omitempty"`
	MachineID  string     `json:"machine_id,omitempty"`
	Status     string     `json:"status"`
}

type Summary struct {
	Agents      []AgentInfo `json:"agents"`
	EntityCount int         `json:"entity_count"`
	DataOrigin  string      `json:"data_origin"`
}
