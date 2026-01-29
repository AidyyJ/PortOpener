package metrics

import (
	"sync"
	"time"
)

type Counters struct {
	Requests int64
	BytesIn  int64
	BytesOut int64
}

type Collector struct {
	mu       sync.Mutex
	byTunnel map[string]Counters
}

func New() *Collector {
	return &Collector{byTunnel: make(map[string]Counters)}
}

func (c *Collector) Add(tunnelID string, reqs, bytesIn, bytesOut int64) {
	if tunnelID == "" {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	entry := c.byTunnel[tunnelID]
	entry.Requests += reqs
	entry.BytesIn += bytesIn
	entry.BytesOut += bytesOut
	c.byTunnel[tunnelID] = entry
}

func (c *Collector) Snapshot() map[string]Counters {
	c.mu.Lock()
	defer c.mu.Unlock()
	snap := make(map[string]Counters, len(c.byTunnel))
	for k, v := range c.byTunnel {
		snap[k] = v
	}
	return snap
}

type LogEntry struct {
	TunnelID   string
	Timestamp  time.Time
	RemoteAddr string
	Method     string
	Path       string
	Status     int
	BytesIn    int64
	BytesOut   int64
}

type Logger struct {
	mu   sync.Mutex
	logs []LogEntry
	max  int
}

func NewLogger(max int) *Logger {
	if max <= 0 {
		max = 1000
	}
	return &Logger{max: max}
}

func (l *Logger) Add(entry LogEntry) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.logs = append(l.logs, entry)
	if len(l.logs) > l.max {
		offset := len(l.logs) - l.max
		l.logs = append([]LogEntry(nil), l.logs[offset:]...)
	}
}

func (l *Logger) Snapshot() []LogEntry {
	l.mu.Lock()
	defer l.mu.Unlock()
	copyLogs := make([]LogEntry, len(l.logs))
	copy(copyLogs, l.logs)
	return copyLogs
}
