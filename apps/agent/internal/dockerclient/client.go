package dockerclient

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"time"

	"brightnet/internal/contract"
	"brightnet/internal/ingest"
)

type Client struct {
	http   *http.Client
	stream *http.Client
}

func New(socketPath string) *Client {
	dial := func(ctx context.Context, _, _ string) (net.Conn, error) {
		var d net.Dialer
		return d.DialContext(ctx, "unix", socketPath)
	}
	return &Client{
		http:   &http.Client{Timeout: 20 * time.Second, Transport: &http.Transport{DialContext: dial}},
		stream: &http.Client{Transport: &http.Transport{DialContext: dial}},
	}
}

func (c *Client) Info(ctx context.Context) (contract.EngineSnapshot, error) {
	var raw struct {
		ID, Name, ServerVersion, OperatingSystem, OSType, Architecture string
		NCPU                                                           int
		MemTotal                                                       int64
	}
	if err := c.get(ctx, "/info", &raw); err != nil {
		return contract.EngineSnapshot{}, err
	}
	return contract.EngineSnapshot{
		ID: raw.ID, Name: raw.Name, Version: raw.ServerVersion, OS: raw.OSType,
		Arch: raw.Architecture, OperatingSystem: raw.OperatingSystem, CPUs: raw.NCPU, MemoryBytes: raw.MemTotal,
	}, nil
}

func (c *Client) Containers(ctx context.Context) ([]contract.ContainerSnapshot, error) {
	var raw []apiContainer
	if err := c.get(ctx, "/containers/json?all=1", &raw); err != nil {
		return nil, err
	}
	out := make([]contract.ContainerSnapshot, 0, len(raw))
	for _, item := range raw {
		out = append(out, convertContainer(item))
	}
	return out, nil
}

func (c *Client) Networks(ctx context.Context) ([]contract.NetworkSnapshot, error) {
	var raw []struct {
		ID, Name, Driver, Scope string
		IPAM                    struct {
			Config []struct{ Subnet string } `json:"Config"`
		} `json:"IPAM"`
	}
	if err := c.get(ctx, "/networks", &raw); err != nil {
		return nil, err
	}
	out := make([]contract.NetworkSnapshot, 0, len(raw))
	for _, item := range raw {
		network := contract.NetworkSnapshot{ID: item.ID, Name: item.Name, Driver: item.Driver, Scope: item.Scope, Subnets: []string{}}
		for _, cfg := range item.IPAM.Config {
			if cfg.Subnet != "" {
				network.Subnets = append(network.Subnets, cfg.Subnet)
			}
		}
		out = append(out, network)
	}
	return out, nil
}

func (c *Client) Volumes(ctx context.Context) ([]contract.VolumeSnapshot, error) {
	var raw struct {
		Volumes []struct{ Name, Driver, Scope string } `json:"Volumes"`
	}
	if err := c.get(ctx, "/volumes", &raw); err != nil {
		return nil, err
	}
	out := make([]contract.VolumeSnapshot, 0, len(raw.Volumes))
	for _, item := range raw.Volumes {
		out = append(out, contract.VolumeSnapshot{Name: item.Name, Driver: item.Driver, Scope: item.Scope})
	}
	return out, nil
}

func (c *Client) Events(ctx context.Context) (<-chan contract.DockerEvent, <-chan error) {
	out := make(chan contract.DockerEvent)
	errs := make(chan error, 1)
	go func() {
		defer close(out)
		defer close(errs)
		filters := url.QueryEscape(`{"type":["container","network","volume"]}`)
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://docker/events?filters="+filters, nil)
		if err != nil {
			errs <- err
			return
		}
		res, err := c.stream.Do(req)
		if err != nil {
			errs <- err
			return
		}
		defer res.Body.Close()
		if res.StatusCode != http.StatusOK {
			errs <- fmt.Errorf("événements Docker: %s", res.Status)
			return
		}
		scanner := bufio.NewScanner(res.Body)
		scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
		for scanner.Scan() {
			var raw apiEvent
			if err := json.Unmarshal(scanner.Bytes(), &raw); err != nil {
				continue
			}
			select {
			case <-ctx.Done():
				return
			case out <- raw.event():
			}
		}
		if err := scanner.Err(); err != nil && ctx.Err() == nil {
			errs <- err
		}
	}()
	return out, errs
}

func (c *Client) get(ctx context.Context, path string, dest any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://docker"+path, nil)
	if err != nil {
		return err
	}
	res, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("docker %s: %s", path, res.Status)
	}
	return json.NewDecoder(res.Body).Decode(dest)
}

type apiContainer struct {
	ID      string            `json:"Id"`
	Names   []string          `json:"Names"`
	Image   string            `json:"Image"`
	ImageID string            `json:"ImageID"`
	State   string            `json:"State"`
	Status  string            `json:"Status"`
	Labels  map[string]string `json:"Labels"`
	Ports   []struct {
		IP          string `json:"IP"`
		PrivatePort int    `json:"PrivatePort"`
		PublicPort  int    `json:"PublicPort"`
		Type        string `json:"Type"`
	} `json:"Ports"`
	NetworkSettings struct {
		Networks map[string]struct {
			NetworkID         string `json:"NetworkID"`
			IPAddress         string `json:"IPAddress"`
			GlobalIPv6Address string `json:"GlobalIPv6Address"`
			MacAddress        string `json:"MacAddress"`
		} `json:"Networks"`
	} `json:"NetworkSettings"`
	Mounts []struct {
		Type        string `json:"Type"`
		Name        string `json:"Name"`
		Destination string `json:"Destination"`
		RW          bool   `json:"RW"`
	} `json:"Mounts"`
}

func convertContainer(raw apiContainer) contract.ContainerSnapshot {
	name := ""
	if len(raw.Names) > 0 {
		name = raw.Names[0]
	}
	item := contract.ContainerSnapshot{
		ID: raw.ID, Name: name, Image: raw.Image, ImageID: raw.ImageID,
		State: raw.State, StatusText: raw.Status, Labels: ingest.FilterLabels(raw.Labels),
	}
	for _, port := range raw.Ports {
		item.Ports = append(item.Ports, contract.PortSnapshot{
			IP: port.IP, PrivatePort: port.PrivatePort, PublicPort: port.PublicPort, Type: port.Type,
		})
	}
	for networkName, network := range raw.NetworkSettings.Networks {
		item.Networks = append(item.Networks, contract.ContainerNet{
			Name: networkName, NetworkID: network.NetworkID, IPAddress: network.IPAddress,
			IPv6: network.GlobalIPv6Address, MacAddress: network.MacAddress,
		})
	}
	for _, mount := range raw.Mounts {
		item.Mounts = append(item.Mounts, contract.MountSnapshot{
			Type: mount.Type, Name: mount.Name, Destination: mount.Destination, RW: mount.RW,
		})
	}
	return item
}

type apiEvent struct {
	Type   string `json:"Type"`
	Action string `json:"Action"`
	Actor  struct {
		ID         string            `json:"ID"`
		Attributes map[string]string `json:"Attributes"`
	} `json:"Actor"`
	Time int64 `json:"time"`
}

func (raw apiEvent) event() contract.DockerEvent {
	name := raw.Actor.Attributes["name"]
	attrs := map[string]string{}
	for _, key := range []string{"image", "name"} {
		if value := raw.Actor.Attributes[key]; value != "" {
			attrs[key] = value
		}
	}
	when := time.Unix(raw.Time, 0).UTC()
	if raw.Time == 0 {
		when = time.Now().UTC()
	}
	return contract.DockerEvent{Type: raw.Type, Action: raw.Action, ActorID: raw.Actor.ID, ActorName: name, Time: when, Attributes: attrs}
}
