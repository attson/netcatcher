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
	"netcatcher/llog"
	"netcatcher/logbuffer"
	nc "netcatcher/netcatcher"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/services/notifications"
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
	notifSvc   *notifications.NotificationService
	configPath string
	logBuf     *logbuffer.Buffer
	app        *application.App
}

func NewApp(configPath string, wailsApp *application.App, notifSvc *notifications.NotificationService) *App {
	a := &App{
		configPath: configPath,
		app:        wailsApp,
		notifSvc:   notifSvc,
	}

	a.logBuf = logbuffer.New(1000, func(e logbuffer.Entry) {
		if a.app != nil {
			a.app.Event.Emit("log:new", e)
		}
	})

	log.SetFlags(0) // Entry.Time carries the timestamp; avoid a duplicate prefix.
	log.SetOutput(a.logBuf.Writer("info"))

	cfg, err := config.Load(configPath)
	if err != nil {
		llog.Errorf("config", "load failed: %v", err)
		cfg = config.Config{Interfaces: []config.Interface{}}
	}

	a.notifier = NewNotifier(notifSvc)

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
	if a.notifSvc != nil {
		go func() {
			granted, err := a.notifSvc.RequestNotificationAuthorization()
			if err != nil {
				llog.Warnf("notify", "authorization request failed: %v", err)
				return
			}
			if !granted {
				llog.Infof("notify", "authorization denied by user")
			}
		}()
	}
}

func (a *App) OnShutdown() {
	a.manager.Stop()
}

func (a *App) GetConfig() config.Config {
	cfg, err := config.Load(a.configPath)
	if err != nil {
		llog.Errorf("config", "load failed: %v", err)
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

func (a *App) PingRoute(ifaceName string, host string) PingResult {
	if ifaceName != "" {
		if err := a.manager.RefreshRoute(ifaceName, host); err != nil {
			llog.Warnf("app", "refresh route %s on %s failed: %v", host, ifaceName, err)
		}
	}

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

type NetworkInterface struct {
	Name  string `json:"name"`
	Label string `json:"label"`
}

func (a *App) GetNetworkInterfaces() []NetworkInterface {
	ifaces, err := net.Interfaces()
	if err != nil {
		return []NetworkInterface{}
	}
	vpnNames := getVPNServiceNames()
	var result []NetworkInterface
	for _, i := range ifaces {
		if strings.HasPrefix(i.Name, "lo") {
			continue
		}
		label := i.Name
		if name, ok := vpnNames[i.Name]; ok {
			label = i.Name + " (" + name + ")"
		}
		result = append(result, NetworkInterface{Name: i.Name, Label: label})
	}
	return result
}
