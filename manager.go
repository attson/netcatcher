package main

import (
	"context"
	"fmt"
	"sync"
	"time"

	"netcatcher/config"
	nc "netcatcher/netcatcher"
)

type MonitorStatus struct {
	Running    bool                 `json:"running"`
	Interfaces []nc.InterfaceStatus `json:"interfaces"`
	StartedAt  *time.Time           `json:"startedAt"`
}

type StatusChangeCallback = nc.StatusCallback

type Manager struct {
	mu        sync.Mutex
	wg        sync.WaitGroup
	cfg       config.Config
	catchers  []*nc.NetCatcher
	cancel    context.CancelFunc
	running   bool
	startedAt *time.Time
	onStatus  StatusChangeCallback
}

func NewManager(cfg config.Config, onStatus StatusChangeCallback) *Manager {
	return &Manager{cfg: cfg, onStatus: onStatus}
}

func (m *Manager) Start() {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.running {
		return
	}

	ctx, cancel := context.WithCancel(context.Background())
	m.cancel = cancel
	m.catchers = nil

	for _, iface := range m.cfg.Interfaces {
		catcher := nc.NewNetCatcher(iface, m.onStatus)
		m.catchers = append(m.catchers, catcher)
		m.wg.Add(1)
		go func() {
			defer m.wg.Done()
			catcher.Watch(ctx)
		}()
	}

	now := time.Now()
	m.startedAt = &now
	m.running = true
}

func (m *Manager) Stop() {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.running {
		return
	}

	m.cancel()
	m.mu.Unlock()
	m.wg.Wait()
	m.mu.Lock()
	m.running = false
	m.startedAt = nil
}

func (m *Manager) IsRunning() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.running
}

func (m *Manager) GetAllStatus() []nc.InterfaceStatus {
	m.mu.Lock()
	defer m.mu.Unlock()

	statuses := make([]nc.InterfaceStatus, len(m.catchers))
	for i, c := range m.catchers {
		statuses[i] = c.GetStatus()
	}
	return statuses
}

func (m *Manager) GetMonitorStatus() MonitorStatus {
	m.mu.Lock()
	defer m.mu.Unlock()

	statuses := make([]nc.InterfaceStatus, len(m.catchers))
	for i, c := range m.catchers {
		statuses[i] = c.GetStatus()
	}
	return MonitorStatus{
		Running:    m.running,
		Interfaces: statuses,
		StartedAt:  m.startedAt,
	}
}

func (m *Manager) RefreshRoute(ifaceName, forAddr string) error {
	m.mu.Lock()
	var target *nc.NetCatcher
	for _, c := range m.catchers {
		if c.Name() == ifaceName {
			target = c
			break
		}
	}
	m.mu.Unlock()
	if target == nil {
		return fmt.Errorf("interface not found: %s", ifaceName)
	}
	return target.RefreshRoute(forAddr)
}

func (m *Manager) UpdateConfig(cfg config.Config) {
	wasRunning := m.IsRunning()
	if wasRunning {
		m.Stop()
	}
	m.mu.Lock()
	m.cfg = cfg
	m.mu.Unlock()
	if wasRunning {
		m.Start()
	}
}
