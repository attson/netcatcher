package main

import (
	"context"
	"fmt"
	"net"
	"sync"
	"time"

	"netcatcher/config"
	"netcatcher/llog"
	nc "netcatcher/netcatcher"
	"netcatcher/route"
)

func isDomainRoute(addr string) bool {
	if _, _, err := net.ParseCIDR(addr); err == nil {
		return false
	}
	if net.ParseIP(addr) != nil {
		return false
	}
	return true
}

type MonitorStatus struct {
	Running    bool                 `json:"running"`
	Interfaces []nc.InterfaceStatus `json:"interfaces"`
	StartedAt  *time.Time           `json:"startedAt"`
}

type StatusChangeCallback = nc.StatusCallback

type Manager struct {
	mu              sync.Mutex
	wg              sync.WaitGroup
	cfg             config.Config
	catchers        []*nc.NetCatcher
	cancel          context.CancelFunc
	running         bool
	startedAt       *time.Time
	onStatus        StatusChangeCallback
	dnsForwarder    *nc.DNSForwarder
	installedDomains []string
}

func NewManager(cfg config.Config, onStatus StatusChangeCallback) *Manager {
	return &Manager{cfg: cfg, onStatus: onStatus, dnsForwarder: nc.NewDNSForwarder()}
}

func (m *Manager) Start() {
	m.mu.Lock()
	if m.running {
		m.mu.Unlock()
		return
	}
	cfg := m.cfg
	m.mu.Unlock()

	var (
		installedDomains []string
		forwarderStarted bool
	)
	if cfg.TunMode {
		domains := collectDomains(cfg)
		if len(domains) > 0 {
			if err := m.dnsForwarder.Start(); err != nil {
				llog.Warnf("manager", "dns forwarder start failed: %v", err)
			} else {
				forwarderStarted = true
				registerForwarderRoutes(m.dnsForwarder, cfg)
				if err := route.InstallResolverEntries(domains, nc.ForwarderPort); err != nil {
					llog.Warnf("manager", "install /etc/resolver entries failed: %v", err)
				} else {
					installedDomains = append([]string(nil), domains...)
				}
			}
		}
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	if m.running {
		// Raced with another Start; roll back what we just did.
		if forwarderStarted {
			m.dnsForwarder.Stop()
		}
		if len(installedDomains) > 0 {
			go func(d []string) {
				if err := route.RemoveResolverEntries(d); err != nil {
					llog.Warnf("manager", "rollback /etc/resolver: %v", err)
				}
			}(installedDomains)
		}
		return
	}

	ctx, cancel := context.WithCancel(context.Background())
	m.cancel = cancel
	m.catchers = nil
	m.installedDomains = installedDomains

	for _, iface := range cfg.Interfaces {
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

func collectDomains(cfg config.Config) []string {
	seen := map[string]struct{}{}
	var out []string
	for _, iface := range cfg.Interfaces {
		for _, r := range iface.Routes {
			if !isDomainRoute(r) {
				continue
			}
			if _, ok := seen[r]; ok {
				continue
			}
			seen[r] = struct{}{}
			out = append(out, r)
		}
	}
	return out
}

func registerForwarderRoutes(fwd *nc.DNSForwarder, cfg config.Config) {
	for _, iface := range cfg.Interfaces {
		netIface, err := net.InterfaceByName(iface.Name)
		if err != nil {
			llog.Warnf("manager", "resolve interface %s for dns forwarder: %v", iface.Name, err)
			continue
		}
		for _, r := range iface.Routes {
			if !isDomainRoute(r) {
				continue
			}
			fwd.SetRoute(r, netIface, iface.DNS)
		}
	}
}

func (m *Manager) Stop() {
	m.mu.Lock()
	if !m.running {
		m.mu.Unlock()
		return
	}
	m.cancel()
	installedDomains := m.installedDomains
	m.installedDomains = nil
	m.mu.Unlock()

	m.wg.Wait()

	m.dnsForwarder.Stop()
	if len(installedDomains) > 0 {
		if err := route.RemoveResolverEntries(installedDomains); err != nil {
			llog.Warnf("manager", "remove /etc/resolver entries failed: %v", err)
		}
	}

	m.mu.Lock()
	m.running = false
	m.startedAt = nil
	m.mu.Unlock()
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
