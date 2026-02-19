package docker

import (
	"context"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/client"
	"github.com/docker/go-connections/nat"
)

type Client struct {
	cli *client.Client
}

type ContainerConfig struct {
	Name        string
	Image       string
	Env         map[string]string
	Ports       []PortMapping
	Volumes     map[string]string
	MemoryLimit int64
	CPULimit    float64
}

type PortMapping struct {
	Host      string `json:"host"`
	Container string `json:"container"`
	Protocol  string `json:"protocol"`
}

func NewClient() (*Client, error) {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, fmt.Errorf("docker client: %w", err)
	}
	return &Client{cli: cli}, nil
}

func (c *Client) Close() error {
	return c.cli.Close()
}

func (c *Client) PullImage(ctx context.Context, ref string) error {
	reader, err := c.cli.ImagePull(ctx, ref, image.PullOptions{})
	if err != nil {
		return fmt.Errorf("pull image: %w", err)
	}
	defer reader.Close()
	_, err = io.Copy(io.Discard, reader)
	return err
}

func (c *Client) CreateContainer(ctx context.Context, cfg ContainerConfig) (string, error) {
	env := make([]string, 0, len(cfg.Env))
	for k, v := range cfg.Env {
		env = append(env, k+"="+v)
	}

	exposedPorts := nat.PortSet{}
	portBindings := nat.PortMap{}
	for _, p := range cfg.Ports {
		proto := p.Protocol
		if proto == "" {
			proto = "tcp"
		}
		containerPort := nat.Port(p.Container + "/" + proto)
		exposedPorts[containerPort] = struct{}{}
		portBindings[containerPort] = []nat.PortBinding{{HostPort: p.Host}}
	}

	mounts := make([]mount.Mount, 0, len(cfg.Volumes))
	for hostPath, containerPath := range cfg.Volumes {
		mounts = append(mounts, mount.Mount{
			Type:   mount.TypeBind,
			Source: hostPath,
			Target: containerPath,
		})
	}

	hostCfg := &container.HostConfig{
		PortBindings:  portBindings,
		Mounts:        mounts,
		RestartPolicy: container.RestartPolicy{Name: "unless-stopped"},
	}
	if cfg.MemoryLimit > 0 {
		hostCfg.Memory = cfg.MemoryLimit
	}
	if cfg.CPULimit > 0 {
		hostCfg.NanoCPUs = int64(cfg.CPULimit * 1e9)
	}

	resp, err := c.cli.ContainerCreate(ctx, &container.Config{
		Image:        cfg.Image,
		Env:          env,
		ExposedPorts: exposedPorts,
		Tty:          true,
		OpenStdin:    true,
		AttachStdin:  true,
	}, hostCfg, nil, nil, cfg.Name)
	if err != nil {
		return "", fmt.Errorf("create container: %w", err)
	}
	return resp.ID, nil
}

func (c *Client) StartContainer(ctx context.Context, id string) error {
	return c.cli.ContainerStart(ctx, id, container.StartOptions{})
}

func (c *Client) StopContainer(ctx context.Context, id string) error {
	timeout := 30
	return c.cli.ContainerStop(ctx, id, container.StopOptions{Timeout: &timeout})
}

func (c *Client) RestartContainer(ctx context.Context, id string) error {
	timeout := 30
	return c.cli.ContainerRestart(ctx, id, container.StopOptions{Timeout: &timeout})
}

func (c *Client) RemoveContainer(ctx context.Context, id string) error {
	return c.cli.ContainerRemove(ctx, id, container.RemoveOptions{Force: true})
}

func (c *Client) InspectContainer(ctx context.Context, id string) (*types.ContainerJSON, error) {
	resp, err := c.cli.ContainerInspect(ctx, id)
	if err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *Client) ContainerStatus(ctx context.Context, id string) (string, error) {
	resp, err := c.cli.ContainerInspect(ctx, id)
	if err != nil {
		return "unknown", err
	}
	return resp.State.Status, nil
}

func (c *Client) ContainerLogs(ctx context.Context, id string, tail string) (io.ReadCloser, error) {
	return c.cli.ContainerLogs(ctx, id, container.LogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Follow:     true,
		Tail:       tail,
	})
}

func (c *Client) ContainerStats(ctx context.Context, id string) (container.StatsResponseReader, error) {
	return c.cli.ContainerStats(ctx, id, true)
}

// ContainerStatsOnce returns a single stats snapshot (non-streaming).
func (c *Client) ContainerStatsOnce(ctx context.Context, id string) (container.StatsResponseReader, error) {
	return c.cli.ContainerStats(ctx, id, false)
}

func (c *Client) ContainerExecAttach(ctx context.Context, containerID string, cmd []string) (types_HijackedResponse, error) {
	execCfg := container.ExecOptions{
		Cmd:          cmd,
		AttachStdin:  true,
		AttachStdout: true,
		AttachStderr: true,
		Tty:          true,
	}
	exec, err := c.cli.ContainerExecCreate(ctx, containerID, execCfg)
	if err != nil {
		return types_HijackedResponse{}, err
	}
	return c.cli.ContainerExecAttach(ctx, exec.ID, container.ExecAttachOptions{Tty: true})
}

// Attach to the container's main process stdin/stdout
func (c *Client) ContainerAttach(ctx context.Context, id string) (types_HijackedResponse, error) {
	return c.cli.ContainerAttach(ctx, id, container.AttachOptions{
		Stream: true,
		Stdin:  true,
		Stdout: true,
		Stderr: true,
	})
}

type types_HijackedResponse = types.HijackedResponse

// ParsePortMappings parses port strings like "25565:25565/tcp"
func ParsePortMappings(ports []string) []PortMapping {
	var result []PortMapping
	for _, p := range ports {
		proto := "tcp"
		if idx := strings.Index(p, "/"); idx != -1 {
			proto = p[idx+1:]
			p = p[:idx]
		}
		parts := strings.SplitN(p, ":", 2)
		if len(parts) == 2 {
			result = append(result, PortMapping{Host: parts[0], Container: parts[1], Protocol: proto})
		}
	}
	return result
}

func FormatPortMappings(ports []PortMapping) []string {
	var result []string
	for _, p := range ports {
		proto := p.Protocol
		if proto == "" {
			proto = "tcp"
		}
		result = append(result, p.Host+":"+p.Container+"/"+proto)
	}
	return result
}

// ParseEnv converts map to key=value slice
func FormatEnv(env map[string]string) []string {
	result := make([]string, 0, len(env))
	for k, v := range env {
		result = append(result, k+"="+v)
	}
	return result
}

// ParseMemory parses a memory string like "2G" or "512M" to bytes
func ParseMemory(s string) int64 {
	s = strings.TrimSpace(strings.ToUpper(s))
	if s == "" || s == "0" {
		return 0
	}
	multiplier := int64(1)
	if strings.HasSuffix(s, "G") {
		multiplier = 1024 * 1024 * 1024
		s = s[:len(s)-1]
	} else if strings.HasSuffix(s, "M") {
		multiplier = 1024 * 1024
		s = s[:len(s)-1]
	}
	val, _ := strconv.ParseInt(s, 10, 64)
	return val * multiplier
}
