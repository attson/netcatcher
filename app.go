package main

import (
	"context"
	"log"
	"net"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"netcatcher/config"
	"netcatcher/logbuffer"
	nc "netcatcher/netcatcher"
	"netcatcher/route"

	"github.com/wailsapp/wails/v3/pkg/application"
)

type PingResult struct {
	Host      string `json:"host"`
	Reachable bool   `json:"reachable"`
	Latency   string `json:"latency"`
	Error     string `json:"error"`
}

type App struct {
	ctx        context.Context
	manager    *Manager
	notifier   *Notifier
	configPath string
	logBuf     *logbuffer.Buffer
	app        *application.App
}

func NewApp(configPath string, wailsApp *application.App) *App {
	a := &App{
		configPath: configPath,
		app:        wailsApp,
	}

	a.logBuf = logbuffer.New(1000, func(e logbuffer.Entry) {
		if a.app != nil {
			a.app.Event.Emit("log:new", e)
		}
	})

	log.SetOutput(a.logBuf.Writer("info"))

	cfg, err := config.Load(configPath)
	if err != nil {
		log.Printf("[error] load config: %v", err)
		cfg = config.Config{Interfaces: []config.Interface{}}
	}

	a.notifier = NewNotifier()

	a.manager = NewManager(cfg, func(status nc.InterfaceStatus) {
		if a.app != nil {
			a.app.Event.Emit("interface:status-changed", status)
		}
		a.notifier.OnStatusChange(status)
	})

	return a
}

func (a *App) OnStartup(ctx context.Context) {
	a.ctx = ctx
	a.manager.Start()
	if a.app != nil {
		a.app.Event.Emit("monitor:started", nil)
	}
}

func (a *App) OnShutdown() {
	a.manager.Stop()
	route.Cleanup()
}

func (a *App) GetConfig() config.Config {
	cfg, err := config.Load(a.configPath)
	if err != nil {
		log.Printf("[error] load config: %v", err)
		return config.Config{Interfaces: []config.Interface{}}
	}
	return cfg
}

func (a *App) SaveConfig(cfg config.Config) error {
	if err := config.Save(a.configPath, cfg); err != nil {
		return err
	}
	a.manager.UpdateConfig(cfg)
	return nil
}

func (a *App) GetConfigPath() string {
	return a.configPath
}

func (a *App) StartMonitor() error {
	a.manager.Start()
	if a.app != nil {
		a.app.Event.Emit("monitor:started", nil)
	}
	return nil
}

func (a *App) StopMonitor() error {
	a.manager.Stop()
	if a.app != nil {
		a.app.Event.Emit("monitor:stopped", nil)
	}
	return nil
}

func (a *App) GetStatus() MonitorStatus {
	return a.manager.GetMonitorStatus()
}

func (a *App) PingRoute(host string) PingResult {
	start := time.Now()
	result := PingResult{Host: host}

	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.Command("ping", "-n", "1", "-w", "3000", host)
	} else {
		cmd = exec.Command("ping", "-c", "1", "-W", "3", host)
	}

	output, err := cmd.CombinedOutput()
	if err != nil {
		result.Reachable = false
		result.Error = err.Error()
		return result
	}

	latency := time.Since(start)
	result.Reachable = true
	result.Latency = latency.Round(time.Millisecond).String()
	_ = output
	return result
}

func (a *App) GetRecentLogs(count int) []logbuffer.Entry {
	return a.logBuf.Recent(count)
}

func (a *App) GetAutoStart() bool {
	return checkAutoStart()
}

func (a *App) SetAutoStart(enabled bool) error {
	return setAutoStart(enabled)
}

func (a *App) SetNotifications(enabled bool) {
	a.notifier.SetEnabled(enabled)
}

func (a *App) GetNotifications() bool {
	return a.notifier.IsEnabled()
}

func (a *App) GetNetworkInterfaces() []string {
	ifaces, err := net.Interfaces()
	if err != nil {
		return []string{}
	}
	var names []string
	for _, i := range ifaces {
		if strings.HasPrefix(i.Name, "lo") {
			continue
		}
		names = append(names, i.Name)
	}
	return names
}
