package stats

import (
	"context"
	"database/sql"
	"encoding/json"
	"io"
	"log"
	"sync"
	"time"

	"github.com/reedfamily/reedout/internal/docker"
)

type Stats struct {
	ID          int64   `json:"id"`
	ServerID    string  `json:"server_id"`
	CPUPercent  float64 `json:"cpu_percent"`
	MemoryBytes int64   `json:"memory_bytes"`
	MemoryLimit int64   `json:"memory_limit"`
	DiskBytes   int64   `json:"disk_bytes"`
	NetworkRx   int64   `json:"network_rx"`
	NetworkTx   int64   `json:"network_tx"`
	RecordedAt  string  `json:"recorded_at"`
}

type Collector struct {
	db     *sql.DB
	docker *docker.Client

	mu        sync.RWMutex
	latest    map[string]*Stats // server_id -> latest stats
	listeners map[string][]chan *Stats

	cancel context.CancelFunc
}

func NewCollector(db *sql.DB, dockerClient *docker.Client) *Collector {
	return &Collector{
		db:        db,
		docker:    dockerClient,
		latest:    make(map[string]*Stats),
		listeners: make(map[string][]chan *Stats),
	}
}

func (c *Collector) Start() {
	ctx, cancel := context.WithCancel(context.Background())
	c.cancel = cancel

	go func() {
		ticker := time.NewTicker(10 * time.Second)
		defer ticker.Stop()

		// Run immediately on start
		c.collect(ctx)

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				c.collect(ctx)
			}
		}
	}()

	log.Println("Stats collector started (10s interval)")
}

func (c *Collector) Stop() {
	if c.cancel != nil {
		c.cancel()
	}
}

func (c *Collector) collect(ctx context.Context) {
	// Get all running servers
	rows, err := c.db.Query("SELECT id, container_id FROM servers WHERE status = 'running' AND container_id != ''")
	if err != nil {
		log.Printf("stats: query servers: %v", err)
		return
	}
	defer rows.Close()

	type serverInfo struct {
		id          string
		containerID string
	}
	var servers []serverInfo
	for rows.Next() {
		var s serverInfo
		if err := rows.Scan(&s.id, &s.containerID); err != nil {
			continue
		}
		servers = append(servers, s)
	}

	for _, srv := range servers {
		stats, err := c.fetchStats(ctx, srv.id, srv.containerID)
		if err != nil {
			log.Printf("stats: fetch %s: %v", srv.id, err)
			continue
		}

		// Write to DB
		_, err = c.db.Exec(
			`INSERT INTO stats (server_id, cpu_percent, memory_bytes, memory_limit, disk_bytes, network_rx, network_tx) VALUES (?, ?, ?, ?, ?, ?, ?)`,
			stats.ServerID, stats.CPUPercent, stats.MemoryBytes, stats.MemoryLimit, stats.DiskBytes, stats.NetworkRx, stats.NetworkTx,
		)
		if err != nil {
			log.Printf("stats: insert %s: %v", srv.id, err)
		}

		// Update latest cache and notify listeners
		c.mu.Lock()
		c.latest[srv.id] = stats
		listeners := c.listeners[srv.id]
		c.mu.Unlock()

		for _, ch := range listeners {
			select {
			case ch <- stats:
			default:
				// Drop if listener is slow
			}
		}
	}

	// Cleanup old stats (older than 24 hours)
	_, err = c.db.Exec("DELETE FROM stats WHERE recorded_at < datetime('now', '-24 hours')")
	if err != nil {
		log.Printf("stats: cleanup: %v", err)
	}
}

func (c *Collector) fetchStats(ctx context.Context, serverID, containerID string) (*Stats, error) {
	resp, err := c.docker.ContainerStatsOnce(ctx, containerID)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var dockerStats dockerStatsJSON
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(data, &dockerStats); err != nil {
		return nil, err
	}

	cpuPercent := calculateCPUPercent(dockerStats)
	memoryBytes := int64(dockerStats.MemoryStats.Usage)
	memoryLimit := int64(dockerStats.MemoryStats.Limit)

	var networkRx, networkTx int64
	for _, netStats := range dockerStats.Networks {
		networkRx += int64(netStats.RxBytes)
		networkTx += int64(netStats.TxBytes)
	}

	return &Stats{
		ServerID:    serverID,
		CPUPercent:  cpuPercent,
		MemoryBytes: memoryBytes,
		MemoryLimit: memoryLimit,
		NetworkRx:   networkRx,
		NetworkTx:   networkTx,
		RecordedAt:  time.Now().UTC().Format(time.RFC3339),
	}, nil
}

func (c *Collector) Latest(serverID string) *Stats {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.latest[serverID]
}

func (c *Collector) Subscribe(serverID string) chan *Stats {
	ch := make(chan *Stats, 1)
	c.mu.Lock()
	c.listeners[serverID] = append(c.listeners[serverID], ch)
	c.mu.Unlock()
	return ch
}

func (c *Collector) Unsubscribe(serverID string, ch chan *Stats) {
	c.mu.Lock()
	defer c.mu.Unlock()
	listeners := c.listeners[serverID]
	for i, l := range listeners {
		if l == ch {
			c.listeners[serverID] = append(listeners[:i], listeners[i+1:]...)
			close(ch)
			return
		}
	}
}

// Docker stats JSON structures
type dockerStatsJSON struct {
	CPUStats    cpuStats               `json:"cpu_stats"`
	PreCPUStats cpuStats               `json:"precpu_stats"`
	MemoryStats memoryStats            `json:"memory_stats"`
	Networks    map[string]networkStats `json:"networks"`
}

type cpuStats struct {
	CPUUsage struct {
		TotalUsage uint64 `json:"total_usage"`
	} `json:"cpu_usage"`
	SystemCPUUsage uint64 `json:"system_cpu_usage"`
	OnlineCPUs     uint64 `json:"online_cpus"`
}

type memoryStats struct {
	Usage uint64 `json:"usage"`
	Limit uint64 `json:"limit"`
}

type networkStats struct {
	RxBytes uint64 `json:"rx_bytes"`
	TxBytes uint64 `json:"tx_bytes"`
}

func calculateCPUPercent(stats dockerStatsJSON) float64 {
	cpuDelta := float64(stats.CPUStats.CPUUsage.TotalUsage - stats.PreCPUStats.CPUUsage.TotalUsage)
	systemDelta := float64(stats.CPUStats.SystemCPUUsage - stats.PreCPUStats.SystemCPUUsage)

	if systemDelta <= 0 || cpuDelta <= 0 {
		return 0
	}

	cpus := float64(stats.CPUStats.OnlineCPUs)
	if cpus == 0 {
		cpus = 1
	}

	return (cpuDelta / systemDelta) * cpus * 100.0
}
