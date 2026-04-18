# NetCatcher Desktop App Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Convert NetCatcher from a CLI daemon into a Wails v3 + Vue 3 desktop application with system tray, GUI config editing, real-time logs, and route connectivity testing.

**Architecture:** Existing Go code (~400 LOC) is refactored into a `Manager` service that the Wails `App` struct wraps as bindings. Vue 3 frontend communicates via Wails-generated JS bindings (request/response) and Wails Events (push notifications). System tray uses Wails v3 native systray API.

**Tech Stack:** Go 1.22+ / Wails v3 (alpha) / Vue 3 + Vite + Vue Router + Pinia / gen2brain/beeep (notifications)

**Important version note:** The design spec references Wails v2, but system tray is only natively supported in Wails v3. This plan uses Wails v3 alpha (v3.0.0-alpha.77+). The API is stable enough for production use per the Wails team. The `wails3` CLI is installed via `go install github.com/wailsapp/wails/v3/cmd/wails3@latest`.

---

## File Map

### New files

| File | Responsibility |
|------|---------------|
| `app.go` | Wails App struct — all frontend-facing bindings (config CRUD, monitor control, logs, settings) |
| `manager.go` | Manager wrapping multiple NetCatcher instances — lifecycle control (Start/Stop/Restart), status aggregation, event emission |
| `logbuffer.go` | Thread-safe ring buffer (1000 entries) for in-memory log storage + custom `log.Writer` |
| `autostart/autostart_darwin.go` | macOS LaunchAgent plist generation for auto-start |
| `autostart/autostart_windows.go` | Windows registry HKCU\...\Run for auto-start |
| `frontend/src/App.vue` | Root layout — sidebar nav + router-view content area + custom titlebar |
| `frontend/src/views/Dashboard.vue` | Interface status cards with expandable route details |
| `frontend/src/views/Routes.vue` | Config editor — add/remove interfaces and routes |
| `frontend/src/views/Logs.vue` | Real-time log viewer with level filter and search |
| `frontend/src/views/Settings.vue` | Auto-start toggle, notification toggle, about info |
| `frontend/src/stores/monitor.js` | Pinia store for interface/route status |
| `frontend/src/stores/logs.js` | Pinia store for log entries |
| `frontend/src/stores/config.js` | Pinia store for config data |
| `frontend/src/styles/theme.css` | GitHub Dark theme variables and base styles |
| `frontend/src/router.js` | Vue Router config (4 routes) |
| `build/appicon.png` | App icon for Wails build |

### Modified files

| File | Changes |
|------|---------|
| `main.go` | Replace CLI entry with Wails v3 application setup (window, systray, bindings) |
| `config/config.go` | Add `Load()`, `Save()`, `DefaultConfigPath()` functions; keep existing structs |
| `netcatcher/netcatcher.go` | Add `context.Context` cancellation to `Watch()`, add callback hooks for status changes and log events |
| `go.mod` | Bump to Go 1.22, add `github.com/wailsapp/wails/v3`, `github.com/gen2brain/beeep` |

### Deleted files

| File | Reason |
|------|--------|
| `install/darwin.sh` | Replaced by auto-start feature in app |

---

## Task 1: Initialize Wails v3 project scaffold

**Files:**
- Create: `wails3.yaml` (Wails v3 config, replaces `wails.json`)
- Create: `Taskfile.yml` (Wails v3 build system)
- Create: `build/appicon.png`
- Modify: `go.mod`

- [ ] **Step 1: Install Wails v3 CLI**

```bash
go install github.com/wailsapp/wails/v3/cmd/wails3@latest
```

Verify: `wails3 version` prints a version string.

- [ ] **Step 2: Initialize Wails v3 in the existing project**

Rather than using `wails3 init` (which creates a new directory), we manually add the required files. First, update `go.mod`:

```bash
cd /Users/attson/code/github.com:attson/netcatcher
go mod edit -go=1.22
go get github.com/wailsapp/wails/v3@latest
go mod tidy
```

- [ ] **Step 3: Create app icon placeholder**

```bash
mkdir -p build
```

Create a simple 512x512 PNG icon at `build/appicon.png`. For now, use a placeholder — any valid PNG will do. The Wails build system expects this file.

- [ ] **Step 4: Create minimal main.go with Wails v3 app**

Replace the contents of `main.go` with a minimal Wails v3 application that opens a window:

```go
package main

import (
	"embed"
	"log"

	"github.com/wailsapp/wails/v3/pkg/application"
)

//go:embed frontend/dist
var assets embed.FS

func main() {
	app := application.New(application.Options{
		Name:        "NetCatcher",
		Description: "Network route manager",
		Assets: application.AssetOptions{
			Handler: application.AssetFileServerFS(assets),
		},
	})

	app.NewWebviewWindowWithOptions(application.WebviewWindowOptions{
		Title:     "NetCatcher",
		Width:     900,
		Height:    600,
		MinWidth:  700,
		MinHeight: 450,
		Frameless: true,
		URL:       "/",
	})

	if err := app.Run(); err != nil {
		log.Fatal(err)
	}
}
```

- [ ] **Step 5: Create minimal Vue 3 frontend**

```bash
cd /Users/attson/code/github.com:attson/netcatcher
npm create vite@latest frontend -- --template vue
cd frontend
npm install
npm install vue-router@4 pinia
```

- [ ] **Step 6: Build frontend and verify Wails compiles**

```bash
cd /Users/attson/code/github.com:attson/netcatcher/frontend
npm run build
cd ..
go build -o netcatcher-app .
```

Expected: compiles without errors. The binary won't run correctly yet (no systray, no bindings), but it should compile.

- [ ] **Step 7: Commit**

```bash
git add main.go go.mod go.sum frontend/ build/
git commit -m "feat: initialize Wails v3 project scaffold with Vue 3 frontend"
```

---

## Task 2: Config module — platform-aware load/save

**Files:**
- Modify: `config/config.go`

- [ ] **Step 1: Write tests for config Load/Save/DefaultConfigPath**

Create `config/config_test.go`:

```go
package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadNonExistent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load non-existent should not error, got: %v", err)
	}
	if len(cfg.Interfaces) != 0 {
		t.Fatalf("expected empty interfaces, got %d", len(cfg.Interfaces))
	}
}

func TestSaveAndLoad(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sub", "config.json")
	cfg := Config{
		Interfaces: []Interface{
			{Name: "ppp0", Routes: []string{"github.com", "192.168.1.0/24"}},
		},
	}
	if err := Save(path, cfg); err != nil {
		t.Fatalf("Save failed: %v", err)
	}
	loaded, err := Load(path)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if len(loaded.Interfaces) != 1 {
		t.Fatalf("expected 1 interface, got %d", len(loaded.Interfaces))
	}
	if loaded.Interfaces[0].Name != "ppp0" {
		t.Fatalf("expected ppp0, got %s", loaded.Interfaces[0].Name)
	}
	if len(loaded.Interfaces[0].Routes) != 2 {
		t.Fatalf("expected 2 routes, got %d", len(loaded.Interfaces[0].Routes))
	}
}

func TestDefaultConfigPath(t *testing.T) {
	path := DefaultConfigPath()
	if path == "" {
		t.Fatal("DefaultConfigPath returned empty string")
	}
	dir := filepath.Dir(path)
	if dir == "" {
		t.Fatal("config directory is empty")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
cd /Users/attson/code/github.com:attson/netcatcher
go test ./config/ -v
```

Expected: FAIL — `Load`, `Save`, `DefaultConfigPath` are not defined.

- [ ] **Step 3: Implement Load, Save, DefaultConfigPath**

Update `config/config.go`:

```go
package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
)

type Interface struct {
	Name   string   `json:"name"`
	Routes []string `json:"routes"`
}

type Config struct {
	Interfaces []Interface `json:"interfaces"`
}

func DefaultConfigPath() string {
	var dir string
	switch runtime.GOOS {
	case "darwin":
		home, _ := os.UserHomeDir()
		dir = filepath.Join(home, "Library", "Application Support", "NetCatcher")
	case "windows":
		dir = filepath.Join(os.Getenv("APPDATA"), "NetCatcher")
	default:
		home, _ := os.UserHomeDir()
		dir = filepath.Join(home, ".config", "netcatcher")
	}
	return filepath.Join(dir, "config.json")
}

func Load(path string) (Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return Config{Interfaces: []Interface{}}, nil
		}
		return Config{}, err
	}
	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return Config{}, err
	}
	if cfg.Interfaces == nil {
		cfg.Interfaces = []Interface{}
	}
	return cfg, nil
}

func Save(path string, cfg Config) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}
```

- [ ] **Step 4: Run tests to verify they pass**

```bash
go test ./config/ -v
```

Expected: all 3 tests PASS.

- [ ] **Step 5: Commit**

```bash
git add config/
git commit -m "feat: add config Load/Save with platform-aware default paths"
```

---

## Task 3: Log buffer — thread-safe ring buffer with event hooks

**Files:**
- Create: `logbuffer/logbuffer.go`
- Create: `logbuffer/logbuffer_test.go`

- [ ] **Step 1: Write tests**

Create `logbuffer/logbuffer_test.go`:

```go
package logbuffer

import (
	"fmt"
	"testing"
	"time"
)

func TestWriteAndRecent(t *testing.T) {
	buf := New(5, nil)
	for i := 0; i < 3; i++ {
		buf.Add(Entry{
			Time:    time.Now(),
			Level:   "info",
			Message: fmt.Sprintf("msg %d", i),
		})
	}
	entries := buf.Recent(10)
	if len(entries) != 3 {
		t.Fatalf("expected 3, got %d", len(entries))
	}
	if entries[0].Message != "msg 0" {
		t.Fatalf("expected msg 0, got %s", entries[0].Message)
	}
}

func TestOverflow(t *testing.T) {
	buf := New(3, nil)
	for i := 0; i < 5; i++ {
		buf.Add(Entry{
			Time:    time.Now(),
			Level:   "info",
			Message: fmt.Sprintf("msg %d", i),
		})
	}
	entries := buf.Recent(10)
	if len(entries) != 3 {
		t.Fatalf("expected 3, got %d", len(entries))
	}
	if entries[0].Message != "msg 2" {
		t.Fatalf("expected msg 2 (oldest kept), got %s", entries[0].Message)
	}
	if entries[2].Message != "msg 4" {
		t.Fatalf("expected msg 4 (newest), got %s", entries[2].Message)
	}
}

func TestRecentLimit(t *testing.T) {
	buf := New(10, nil)
	for i := 0; i < 8; i++ {
		buf.Add(Entry{
			Time:    time.Now(),
			Level:   "info",
			Message: fmt.Sprintf("msg %d", i),
		})
	}
	entries := buf.Recent(3)
	if len(entries) != 3 {
		t.Fatalf("expected 3, got %d", len(entries))
	}
	if entries[0].Message != "msg 5" {
		t.Fatalf("expected msg 5, got %s", entries[0].Message)
	}
}

func TestCallback(t *testing.T) {
	var received []Entry
	cb := func(e Entry) {
		received = append(received, e)
	}
	buf := New(5, cb)
	buf.Add(Entry{Time: time.Now(), Level: "warn", Message: "test"})
	if len(received) != 1 {
		t.Fatalf("expected callback called once, got %d", len(received))
	}
	if received[0].Level != "warn" {
		t.Fatalf("expected warn, got %s", received[0].Level)
	}
}

func TestWriter(t *testing.T) {
	buf := New(10, nil)
	w := buf.Writer("info")
	_, err := w.Write([]byte("hello from writer\n"))
	if err != nil {
		t.Fatalf("Write failed: %v", err)
	}
	entries := buf.Recent(10)
	if len(entries) != 1 {
		t.Fatalf("expected 1, got %d", len(entries))
	}
	if entries[0].Message != "hello from writer" {
		t.Fatalf("expected 'hello from writer', got '%s'", entries[0].Message)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
go test ./logbuffer/ -v
```

Expected: FAIL — package does not exist.

- [ ] **Step 3: Implement logbuffer**

Create `logbuffer/logbuffer.go`:

```go
package logbuffer

import (
	"strings"
	"sync"
	"time"
)

type Entry struct {
	Time    time.Time `json:"time"`
	Level   string    `json:"level"`
	Message string    `json:"message"`
}

type OnNewEntry func(Entry)

type Buffer struct {
	mu       sync.Mutex
	entries  []Entry
	capacity int
	head     int
	count    int
	onNew    OnNewEntry
}

func New(capacity int, onNew OnNewEntry) *Buffer {
	return &Buffer{
		entries:  make([]Entry, capacity),
		capacity: capacity,
		onNew:    onNew,
	}
}

func (b *Buffer) Add(e Entry) {
	b.mu.Lock()
	b.entries[b.head] = e
	b.head = (b.head + 1) % b.capacity
	if b.count < b.capacity {
		b.count++
	}
	b.mu.Unlock()

	if b.onNew != nil {
		b.onNew(e)
	}
}

func (b *Buffer) Recent(n int) []Entry {
	b.mu.Lock()
	defer b.mu.Unlock()

	if n > b.count {
		n = b.count
	}
	result := make([]Entry, n)
	start := (b.head - n + b.capacity) % b.capacity
	for i := 0; i < n; i++ {
		result[i] = b.entries[(start+i)%b.capacity]
	}
	return result
}

type writer struct {
	buf   *Buffer
	level string
}

func (b *Buffer) Writer(level string) *writer {
	return &writer{buf: b, level: level}
}

func (w *writer) Write(p []byte) (n int, err error) {
	msg := strings.TrimRight(string(p), "\n")
	if msg == "" {
		return len(p), nil
	}
	w.buf.Add(Entry{
		Time:    time.Now(),
		Level:   w.level,
		Message: msg,
	})
	return len(p), nil
}
```

- [ ] **Step 4: Run tests to verify they pass**

```bash
go test ./logbuffer/ -v
```

Expected: all 5 tests PASS.

- [ ] **Step 5: Commit**

```bash
git add logbuffer/
git commit -m "feat: add thread-safe log ring buffer with callback and io.Writer"
```

---

## Task 4: Refactor netcatcher.go — add context cancellation and status callbacks

**Files:**
- Modify: `netcatcher/netcatcher.go`
- Create: `netcatcher/netcatcher_test.go`

- [ ] **Step 1: Write test for NetCatcher with context cancellation**

Create `netcatcher/netcatcher_test.go`:

```go
package netcatcher

import (
	"context"
	"testing"
	"time"

	"netcatcher/config"
)

func TestNewNetCatcher(t *testing.T) {
	cfg := config.Interface{Name: "lo0", Routes: []string{}}
	nc := NewNetCatcher(cfg, nil)
	if nc == nil {
		t.Fatal("expected non-nil NetCatcher")
	}
	status := nc.GetStatus()
	if status.InterfaceName != "lo0" {
		t.Fatalf("expected lo0, got %s", status.InterfaceName)
	}
	if status.Connected {
		t.Fatal("expected disconnected initially")
	}
}

func TestWatchCancellation(t *testing.T) {
	cfg := config.Interface{Name: "nonexistent_iface_xyz", Routes: []string{}}
	nc := NewNetCatcher(cfg, nil)
	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan struct{})
	go func() {
		nc.Watch(ctx)
		close(done)
	}()

	time.Sleep(100 * time.Millisecond)
	cancel()

	select {
	case <-done:
		// Watch returned after cancel — correct
	case <-time.After(2 * time.Second):
		t.Fatal("Watch did not return after context cancellation")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
go test ./netcatcher/ -v -count=1
```

Expected: FAIL — `GetStatus` is not defined, `Watch` signature doesn't match.

- [ ] **Step 3: Refactor netcatcher.go**

Replace `netcatcher/netcatcher.go` with the refactored version:

```go
package netcatcher

import (
	"context"
	"fmt"
	"log"
	"net"
	"netcatcher/config"
	"netcatcher/route"
	"time"
)

type InterfaceStatus struct {
	InterfaceName string `json:"interfaceName"`
	Connected     bool   `json:"connected"`
	Gateway       string `json:"gateway"`
	Routes        []RouteStatus `json:"routes"`
}

type RouteStatus struct {
	For     string `json:"for"`
	Ip      string `json:"ip"`
	Gateway string `json:"gateway"`
	Active  bool   `json:"active"`
}

type StatusCallback func(status InterfaceStatus)

type status int

const (
	_ status = iota
	connected
	disconnected
)

type routeEntry struct {
	forAddr string
	ip      string
	gateway string
	mask    net.IPMask
}

func (r routeEntry) String() string {
	return fmt.Sprintf("%s -> %s @ %s", r.forAddr, r.ip, r.gateway)
}

type changeEvent struct {
	status status
	addr   net.Addr
}

type NetCatcher struct {
	config   config.Interface
	onChange chan changeEvent
	current  status
	routes   []routeEntry
	onStatus StatusCallback
}

func NewNetCatcher(cfg config.Interface, onStatus StatusCallback) *NetCatcher {
	return &NetCatcher{
		config:   cfg,
		onChange: make(chan changeEvent),
		onStatus: onStatus,
	}
}

func (n *NetCatcher) GetStatus() InterfaceStatus {
	s := InterfaceStatus{
		InterfaceName: n.config.Name,
		Connected:     n.current == connected,
		Routes:        make([]RouteStatus, len(n.routes)),
	}
	for i, r := range n.routes {
		s.Routes[i] = RouteStatus{
			For:     r.forAddr,
			Ip:      r.ip,
			Gateway: r.gateway,
			Active:  n.current == connected,
		}
	}
	if len(n.routes) > 0 {
		s.Gateway = n.routes[0].gateway
	}
	return s
}

func (n *NetCatcher) emitStatus() {
	if n.onStatus != nil {
		n.onStatus(n.GetStatus())
	}
}

func (n *NetCatcher) resolveRoutes(gateway string) {
	n.routes = []routeEntry{}
	for _, addr := range n.config.Routes {
		_, ipnet, err := net.ParseCIDR(addr)
		if err == nil {
			n.routes = append(n.routes, routeEntry{
				forAddr: addr, ip: addr, mask: ipnet.Mask, gateway: gateway,
			})
			continue
		}
		if net.ParseIP(addr) != nil {
			n.routes = append(n.routes, routeEntry{
				forAddr: addr, ip: addr, mask: nil, gateway: gateway,
			})
			continue
		}
		ips, err := net.LookupIP(addr)
		if err != nil {
			log.Printf("%s: [warn] lookup %s fail %v\n", n.config.Name, addr, err)
		}
		for _, ip := range ips {
			n.routes = append(n.routes, routeEntry{
				forAddr: addr, ip: ip.String(), gateway: gateway,
			})
		}
	}
}

func (n *NetCatcher) addRoutesTo(addr net.Addr) {
	ip, _, err := net.ParseCIDR(addr.String())
	if err != nil {
		log.Printf("%s: [error] parse %s CIDR fail %v", n.config.Name, addr.String(), err)
		return
	}
	n.resolveRoutes(ip.String())
	for _, r := range n.routes {
		err := route.AddRoute(r.ip, r.gateway, r.mask)
		if err != nil {
			log.Printf("%s: [warn] add route fail %s %v", n.config.Name, r, err)
		} else {
			log.Printf("%s: [debug] add route %s", n.config.Name, r)
		}
	}
}

func (n *NetCatcher) clearRoutes() {
	for _, r := range n.routes {
		err := route.DeleteRoute(r.ip, r.gateway, r.mask)
		if err != nil {
			log.Printf("%s: [warn] delete route fail %s %v", n.config.Name, r, err)
		} else {
			log.Printf("%s: [debug] delete route %s", n.config.Name, r)
		}
	}
}

func (n *NetCatcher) Watch(ctx context.Context) {
	poll := make(chan changeEvent)

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			default:
			}
			i, err := net.InterfaceByName(n.config.Name)
			if err != nil {
				if opErr, ok := err.(*net.OpError); ok {
					if opErr.Unwrap().Error() == "no such network interface" {
						poll <- changeEvent{status: disconnected}
						time.Sleep(time.Second)
						continue
					}
				}
				log.Printf("%s: [warn] get interface fail %v\n", n.config.Name, err)
			} else {
				addrs, err := i.Addrs()
				if err != nil || len(addrs) == 0 {
					log.Printf("%s: [warn] get interface addr fail %v\n", n.config.Name, err)
				} else {
					poll <- changeEvent{status: connected, addr: addrs[0]}
				}
			}
			time.Sleep(time.Second)
		}
	}()

	for {
		select {
		case <-ctx.Done():
			n.clearRoutes()
			return
		case event := <-poll:
			if n.current == event.status {
				break
			}
			log.Printf("%s: [info] interface status changed to %v\n", n.config.Name, event.status == connected)
			n.current = event.status
			if event.status == connected {
				n.addRoutesTo(event.addr)
			}
			n.emitStatus()
		}
	}
}

func (n *NetCatcher) Stop() {
	n.clearRoutes()
}
```

- [ ] **Step 4: Run tests to verify they pass**

```bash
go test ./netcatcher/ -v -count=1
```

Expected: both tests PASS.

- [ ] **Step 5: Commit**

```bash
git add netcatcher/
git commit -m "refactor: add context cancellation and status callbacks to NetCatcher"
```

---

## Task 5: Manager — lifecycle control for all NetCatcher instances

**Files:**
- Create: `manager.go`
- Create: `manager_test.go`

- [ ] **Step 1: Write tests for Manager**

Create `manager_test.go`:

```go
package main

import (
	"netcatcher/config"
	"testing"
	"time"
)

func TestManagerStartStop(t *testing.T) {
	cfg := config.Config{
		Interfaces: []config.Interface{
			{Name: "nonexistent_iface_test", Routes: []string{}},
		},
	}
	m := NewManager(cfg, nil)

	if m.IsRunning() {
		t.Fatal("expected not running initially")
	}

	m.Start()
	time.Sleep(100 * time.Millisecond)

	if !m.IsRunning() {
		t.Fatal("expected running after Start")
	}

	statuses := m.GetAllStatus()
	if len(statuses) != 1 {
		t.Fatalf("expected 1 status, got %d", len(statuses))
	}
	if statuses[0].InterfaceName != "nonexistent_iface_test" {
		t.Fatalf("expected nonexistent_iface_test, got %s", statuses[0].InterfaceName)
	}

	m.Stop()
	time.Sleep(100 * time.Millisecond)

	if m.IsRunning() {
		t.Fatal("expected not running after Stop")
	}
}

func TestManagerDoubleStart(t *testing.T) {
	cfg := config.Config{
		Interfaces: []config.Interface{},
	}
	m := NewManager(cfg, nil)
	m.Start()
	m.Start() // should not panic
	m.Stop()
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
go test -run TestManager -v -count=1
```

Expected: FAIL — `NewManager` not defined.

- [ ] **Step 3: Implement Manager**

Create `manager.go`:

```go
package main

import (
	"context"
	"sync"
	"time"

	"netcatcher/config"
	nc "netcatcher/netcatcher"
)

type MonitorStatus struct {
	Running    bool                  `json:"running"`
	Interfaces []nc.InterfaceStatus  `json:"interfaces"`
	StartedAt  *time.Time            `json:"startedAt"`
}

type StatusChangeCallback func(status nc.InterfaceStatus)

type Manager struct {
	mu        sync.Mutex
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
		go catcher.Watch(ctx)
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
```

- [ ] **Step 4: Run tests to verify they pass**

```bash
go test -run TestManager -v -count=1
```

Expected: both tests PASS.

- [ ] **Step 5: Commit**

```bash
git add manager.go manager_test.go
git commit -m "feat: add Manager for NetCatcher lifecycle control"
```

---

## Task 6: App struct — Wails bindings layer

**Files:**
- Create: `app.go`

- [ ] **Step 1: Create App struct with all binding methods**

Create `app.go`:

```go
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
			a.app.EmitEvent("log:new", e)
		}
	})

	log.SetOutput(a.logBuf.Writer("info"))

	cfg, err := config.Load(configPath)
	if err != nil {
		log.Printf("[error] load config: %v", err)
		cfg = config.Config{Interfaces: []config.Interface{}}
	}

	a.manager = NewManager(cfg, func(status nc.InterfaceStatus) {
		if a.app != nil {
			a.app.EmitEvent("interface:status-changed", status)
		}
	})

	return a
}

func (a *App) OnStartup(ctx context.Context) {
	a.ctx = ctx
	a.manager.Start()
	if a.app != nil {
		a.app.EmitEvent("monitor:started", nil)
	}
}

func (a *App) OnShutdown() {
	a.manager.Stop()
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
		a.app.EmitEvent("monitor:started", nil)
	}
	return nil
}

func (a *App) StopMonitor() error {
	a.manager.Stop()
	if a.app != nil {
		a.app.EmitEvent("monitor:stopped", nil)
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
```

- [ ] **Step 2: Commit**

```bash
git add app.go
git commit -m "feat: add App struct with Wails binding methods"
```

---

## Task 7: Auto-start — platform-specific implementations

**Files:**
- Create: `autostart_darwin.go`
- Create: `autostart_windows.go`

- [ ] **Step 1: Implement macOS auto-start**

Create `autostart_darwin.go`:

```go
//go:build darwin

package main

import (
	"fmt"
	"os"
	"path/filepath"
)

const plistTemplate = `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>Label</key>
	<string>com.attson.netcatcher</string>
	<key>ProgramArguments</key>
	<array>
		<string>%s</string>
	</array>
	<key>RunAtLoad</key>
	<true/>
	<key>KeepAlive</key>
	<false/>
</dict>
</plist>`

func plistPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, "Library", "LaunchAgents", "com.attson.netcatcher.plist")
}

func checkAutoStart() bool {
	_, err := os.Stat(plistPath())
	return err == nil
}

func setAutoStart(enabled bool) error {
	path := plistPath()
	if !enabled {
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return err
		}
		return nil
	}

	execPath, err := os.Executable()
	if err != nil {
		return err
	}

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	content := fmt.Sprintf(plistTemplate, execPath)
	return os.WriteFile(path, []byte(content), 0644)
}
```

- [ ] **Step 2: Implement Windows auto-start**

Create `autostart_windows.go`:

```go
//go:build windows

package main

import (
	"os"

	"golang.org/x/sys/windows/registry"
)

const regKey = `Software\Microsoft\Windows\CurrentVersion\Run`
const regValue = "NetCatcher"

func checkAutoStart() bool {
	k, err := registry.OpenKey(registry.CURRENT_USER, regKey, registry.QUERY_VALUE)
	if err != nil {
		return false
	}
	defer k.Close()
	_, _, err = k.GetStringValue(regValue)
	return err == nil
}

func setAutoStart(enabled bool) error {
	if !enabled {
		k, err := registry.OpenKey(registry.CURRENT_USER, regKey, registry.SET_VALUE)
		if err != nil {
			return nil
		}
		defer k.Close()
		_ = k.DeleteValue(regValue)
		return nil
	}

	execPath, err := os.Executable()
	if err != nil {
		return err
	}

	k, err := registry.OpenKey(registry.CURRENT_USER, regKey, registry.SET_VALUE)
	if err != nil {
		return err
	}
	defer k.Close()
	return k.SetStringValue(regValue, execPath)
}
```

- [ ] **Step 3: Add Windows registry dependency**

```bash
go get golang.org/x/sys
go mod tidy
```

- [ ] **Step 4: Verify compilation on current platform**

```bash
go build -o /dev/null .
```

Expected: compiles on macOS. Windows file is excluded by build tag.

- [ ] **Step 5: Commit**

```bash
git add autostart_darwin.go autostart_windows.go go.mod go.sum
git commit -m "feat: add platform-specific auto-start (macOS plist, Windows registry)"
```

---

## Task 8: Main entry point — Wails v3 app with systray

**Files:**
- Modify: `main.go`

- [ ] **Step 1: Rewrite main.go with full Wails v3 setup**

```go
package main

import (
	"embed"
	_ "embed"
	"log"

	"netcatcher/config"

	"github.com/wailsapp/wails/v3/pkg/application"
)

//go:embed frontend/dist
var assets embed.FS

//go:embed build/appicon.png
var appIcon []byte

func main() {
	configPath := config.DefaultConfigPath()

	wailsApp := application.New(application.Options{
		Name:        "NetCatcher",
		Description: "Network route manager",
		Assets: application.AssetOptions{
			Handler: application.AssetFileServerFS(assets),
		},
	})

	app := NewApp(configPath, wailsApp)

	wailsApp.OnEvent("startup", func(event *application.CustomEvent) {
		app.OnStartup(nil)
	})
	wailsApp.OnEvent("shutdown", func(event *application.CustomEvent) {
		app.OnShutdown()
	})

	wailsApp.NewService(app)

	mainWindow := wailsApp.NewWebviewWindowWithOptions(application.WebviewWindowOptions{
		Title:            "NetCatcher",
		Width:            900,
		Height:           600,
		MinWidth:         700,
		MinHeight:        450,
		Frameless:        true,
		Hidden:           false,
		URL:              "/",
		BackgroundColour: application.NewRGBA(13, 17, 23, 255),
	})

	trayMenu := wailsApp.NewMenu()
	trayMenu.Add("Show Window").OnClick(func(ctx *application.Context) {
		mainWindow.Show()
		mainWindow.Focus()
	})
	trayMenu.AddSeparator()
	trayMenu.Add("Quit").OnClick(func(ctx *application.Context) {
		app.OnShutdown()
		wailsApp.Quit()
	})

	systray := wailsApp.NewSystemTray()
	systray.SetIcon(appIcon)
	systray.SetMenu(trayMenu)

	mainWindow.OnWindowEvent(application.WindowEventClosing, func(event *application.WindowEvent) {
		mainWindow.Hide()
		event.Cancel()
	})

	if err := wailsApp.Run(); err != nil {
		log.Fatal(err)
	}
}
```

- [ ] **Step 2: Verify compilation**

```bash
go build -o /dev/null .
```

Expected: compiles without errors (frontend/dist must exist from Task 1).

- [ ] **Step 3: Commit**

```bash
git add main.go
git commit -m "feat: wire up Wails v3 app with systray, window hide-on-close"
```

---

## Task 9: Frontend — GitHub Dark theme and layout shell

**Files:**
- Create: `frontend/src/styles/theme.css`
- Modify: `frontend/src/App.vue`
- Create: `frontend/src/router.js`
- Modify: `frontend/src/main.js`

- [ ] **Step 1: Create GitHub Dark theme CSS**

Create `frontend/src/styles/theme.css`:

```css
:root {
  --bg-primary: #0d1117;
  --bg-secondary: #161b22;
  --bg-tertiary: #21262d;
  --bg-hover: #30363d;
  --border-color: #30363d;
  --text-primary: #c9d1d9;
  --text-secondary: #8b949e;
  --text-link: #58a6ff;
  --accent-blue: #1f6feb;
  --accent-blue-bg: rgba(31, 111, 235, 0.15);
  --success: #3fb950;
  --success-bg: rgba(63, 185, 80, 0.15);
  --warning: #d29922;
  --warning-bg: rgba(210, 153, 34, 0.15);
  --error: #f85149;
  --error-bg: rgba(248, 81, 73, 0.15);
  --font-mono: 'SF Mono', 'Cascadia Code', 'Consolas', monospace;
  --font-sans: -apple-system, BlinkMacSystemFont, 'Segoe UI', Helvetica, Arial, sans-serif;
  --radius: 6px;
  --sidebar-width: 200px;
}

* {
  margin: 0;
  padding: 0;
  box-sizing: border-box;
}

body {
  font-family: var(--font-sans);
  background: var(--bg-primary);
  color: var(--text-primary);
  font-size: 14px;
  line-height: 1.5;
  overflow: hidden;
  height: 100vh;
}

#app {
  display: flex;
  flex-direction: column;
  height: 100vh;
}

.titlebar {
  display: flex;
  align-items: center;
  height: 38px;
  background: var(--bg-secondary);
  border-bottom: 1px solid var(--border-color);
  padding: 0 12px;
  -webkit-app-region: drag;
  user-select: none;
}

.titlebar-title {
  font-size: 13px;
  color: var(--text-secondary);
  font-weight: 500;
}

.layout {
  display: flex;
  flex: 1;
  overflow: hidden;
}

.sidebar {
  width: var(--sidebar-width);
  background: var(--bg-secondary);
  border-right: 1px solid var(--border-color);
  padding: 12px 0;
  display: flex;
  flex-direction: column;
}

.sidebar-nav {
  list-style: none;
}

.sidebar-nav a {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 8px 16px;
  color: var(--text-secondary);
  text-decoration: none;
  font-size: 14px;
  transition: background 0.15s, color 0.15s;
}

.sidebar-nav a:hover {
  background: var(--bg-tertiary);
  color: var(--text-primary);
}

.sidebar-nav a.router-link-active {
  background: var(--accent-blue-bg);
  color: var(--text-link);
  border-left: 2px solid var(--accent-blue);
}

.content {
  flex: 1;
  overflow-y: auto;
  padding: 24px;
}

.card {
  background: var(--bg-secondary);
  border: 1px solid var(--border-color);
  border-radius: var(--radius);
  padding: 16px;
  margin-bottom: 12px;
}

.badge {
  display: inline-block;
  padding: 2px 8px;
  border-radius: 10px;
  font-size: 12px;
  font-weight: 500;
}

.badge-success {
  background: var(--success-bg);
  color: var(--success);
}

.badge-error {
  background: var(--error-bg);
  color: var(--error);
}

.btn {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  padding: 6px 14px;
  border-radius: var(--radius);
  border: 1px solid var(--border-color);
  background: var(--bg-tertiary);
  color: var(--text-primary);
  font-size: 13px;
  cursor: pointer;
  transition: background 0.15s;
}

.btn:hover {
  background: var(--bg-hover);
}

.btn-primary {
  background: var(--accent-blue);
  border-color: var(--accent-blue);
  color: #fff;
}

.btn-primary:hover {
  background: #388bfd;
}

.btn-danger {
  color: var(--error);
  border-color: var(--error);
}

.btn-danger:hover {
  background: var(--error-bg);
}

input, select {
  background: var(--bg-primary);
  border: 1px solid var(--border-color);
  border-radius: var(--radius);
  padding: 6px 10px;
  color: var(--text-primary);
  font-size: 14px;
  outline: none;
}

input:focus, select:focus {
  border-color: var(--accent-blue);
  box-shadow: 0 0 0 2px var(--accent-blue-bg);
}

.toggle {
  position: relative;
  width: 40px;
  height: 22px;
  background: var(--bg-tertiary);
  border-radius: 11px;
  cursor: pointer;
  transition: background 0.2s;
  border: 1px solid var(--border-color);
}

.toggle.active {
  background: var(--accent-blue);
  border-color: var(--accent-blue);
}

.toggle::after {
  content: '';
  position: absolute;
  width: 16px;
  height: 16px;
  border-radius: 50%;
  background: #fff;
  top: 2px;
  left: 2px;
  transition: transform 0.2s;
}

.toggle.active::after {
  transform: translateX(18px);
}

h1 { font-size: 20px; font-weight: 600; margin-bottom: 16px; }
h2 { font-size: 16px; font-weight: 600; margin-bottom: 12px; }
h3 { font-size: 14px; font-weight: 600; margin-bottom: 8px; }

::-webkit-scrollbar {
  width: 8px;
}

::-webkit-scrollbar-track {
  background: var(--bg-primary);
}

::-webkit-scrollbar-thumb {
  background: var(--bg-tertiary);
  border-radius: 4px;
}

::-webkit-scrollbar-thumb:hover {
  background: var(--bg-hover);
}
```

- [ ] **Step 2: Create Vue Router config**

Create `frontend/src/router.js`:

```js
import { createRouter, createWebHashHistory } from 'vue-router'
import Dashboard from './views/Dashboard.vue'
import Routes from './views/Routes.vue'
import Logs from './views/Logs.vue'
import Settings from './views/Settings.vue'

const routes = [
  { path: '/', name: 'dashboard', component: Dashboard },
  { path: '/routes', name: 'routes', component: Routes },
  { path: '/logs', name: 'logs', component: Logs },
  { path: '/settings', name: 'settings', component: Settings },
]

export default createRouter({
  history: createWebHashHistory(),
  routes,
})
```

- [ ] **Step 3: Create placeholder views**

Create `frontend/src/views/Dashboard.vue`:

```vue
<script setup>
</script>

<template>
  <div>
    <h1>Dashboard</h1>
    <p style="color: var(--text-secondary)">Interface status overview</p>
  </div>
</template>
```

Create `frontend/src/views/Routes.vue`:

```vue
<script setup>
</script>

<template>
  <div>
    <h1>Routes</h1>
    <p style="color: var(--text-secondary)">Route configuration</p>
  </div>
</template>
```

Create `frontend/src/views/Logs.vue`:

```vue
<script setup>
</script>

<template>
  <div>
    <h1>Logs</h1>
    <p style="color: var(--text-secondary)">Real-time log viewer</p>
  </div>
</template>
```

Create `frontend/src/views/Settings.vue`:

```vue
<script setup>
</script>

<template>
  <div>
    <h1>Settings</h1>
    <p style="color: var(--text-secondary)">Application settings</p>
  </div>
</template>
```

- [ ] **Step 4: Update App.vue with layout shell**

Replace `frontend/src/App.vue`:

```vue
<script setup>
</script>

<template>
  <div class="titlebar">
    <span class="titlebar-title">NetCatcher</span>
  </div>
  <div class="layout">
    <nav class="sidebar">
      <ul class="sidebar-nav">
        <li><router-link to="/">Dashboard</router-link></li>
        <li><router-link to="/routes">Routes</router-link></li>
        <li><router-link to="/logs">Logs</router-link></li>
        <li><router-link to="/settings">Settings</router-link></li>
      </ul>
    </nav>
    <main class="content">
      <router-view />
    </main>
  </div>
</template>
```

- [ ] **Step 5: Update main.js with router and pinia**

Replace `frontend/src/main.js`:

```js
import { createApp } from 'vue'
import { createPinia } from 'pinia'
import App from './App.vue'
import router from './router'
import './styles/theme.css'

const app = createApp(App)
app.use(createPinia())
app.use(router)
app.mount('#app')
```

- [ ] **Step 6: Build frontend and verify**

```bash
cd frontend && npm run build && cd ..
go build -o /dev/null .
```

Expected: compiles. The app should open a window with sidebar navigation and GitHub Dark theme.

- [ ] **Step 7: Commit**

```bash
git add frontend/src/
git commit -m "feat: add GitHub Dark theme, layout shell, and Vue Router setup"
```

---

## Task 10: Pinia stores — monitor, logs, config

**Files:**
- Create: `frontend/src/stores/monitor.js`
- Create: `frontend/src/stores/logs.js`
- Create: `frontend/src/stores/config.js`

- [ ] **Step 1: Create monitor store**

Create `frontend/src/stores/monitor.js`:

```js
import { defineStore } from 'pinia'
import { ref, computed } from 'vue'

export const useMonitorStore = defineStore('monitor', () => {
  const status = ref({
    running: false,
    interfaces: [],
    startedAt: null,
  })

  const activeCount = computed(() =>
    status.value.interfaces.filter(i => i.connected).length
  )

  const totalRoutes = computed(() =>
    status.value.interfaces.reduce((sum, i) => sum + (i.routes?.length || 0), 0)
  )

  async function fetchStatus() {
    try {
      const result = await window.wails.Call({ packageName: 'main', structName: 'App', methodName: 'GetStatus' })
      status.value = result
    } catch (e) {
      console.error('fetchStatus failed:', e)
    }
  }

  async function startMonitor() {
    await window.wails.Call({ packageName: 'main', structName: 'App', methodName: 'StartMonitor' })
    await fetchStatus()
  }

  async function stopMonitor() {
    await window.wails.Call({ packageName: 'main', structName: 'App', methodName: 'StopMonitor' })
    await fetchStatus()
  }

  function updateInterfaceStatus(ifaceStatus) {
    const idx = status.value.interfaces.findIndex(
      i => i.interfaceName === ifaceStatus.interfaceName
    )
    if (idx >= 0) {
      status.value.interfaces[idx] = ifaceStatus
    }
  }

  return {
    status,
    activeCount,
    totalRoutes,
    fetchStatus,
    startMonitor,
    stopMonitor,
    updateInterfaceStatus,
  }
})
```

- [ ] **Step 2: Create logs store**

Create `frontend/src/stores/logs.js`:

```js
import { defineStore } from 'pinia'
import { ref, computed } from 'vue'

export const useLogStore = defineStore('logs', () => {
  const entries = ref([])
  const levelFilter = ref('all')
  const searchQuery = ref('')
  const maxEntries = 1000

  const filtered = computed(() => {
    return entries.value.filter(e => {
      if (levelFilter.value !== 'all' && e.level !== levelFilter.value) {
        return false
      }
      if (searchQuery.value && !e.message.toLowerCase().includes(searchQuery.value.toLowerCase())) {
        return false
      }
      return true
    })
  })

  async function fetchRecent() {
    try {
      const result = await window.wails.Call({ packageName: 'main', structName: 'App', methodName: 'GetRecentLogs', args: [200] })
      entries.value = result || []
    } catch (e) {
      console.error('fetchRecent failed:', e)
    }
  }

  function addEntry(entry) {
    entries.value.push(entry)
    if (entries.value.length > maxEntries) {
      entries.value = entries.value.slice(-maxEntries)
    }
  }

  function clear() {
    entries.value = []
  }

  return {
    entries,
    levelFilter,
    searchQuery,
    filtered,
    fetchRecent,
    addEntry,
    clear,
  }
})
```

- [ ] **Step 3: Create config store**

Create `frontend/src/stores/config.js`:

```js
import { defineStore } from 'pinia'
import { ref } from 'vue'

export const useConfigStore = defineStore('config', () => {
  const config = ref({ interfaces: [] })
  const configPath = ref('')
  const saving = ref(false)

  async function fetchConfig() {
    try {
      const result = await window.wails.Call({ packageName: 'main', structName: 'App', methodName: 'GetConfig' })
      config.value = result
    } catch (e) {
      console.error('fetchConfig failed:', e)
    }
  }

  async function fetchConfigPath() {
    try {
      const result = await window.wails.Call({ packageName: 'main', structName: 'App', methodName: 'GetConfigPath' })
      configPath.value = result
    } catch (e) {
      console.error('fetchConfigPath failed:', e)
    }
  }

  async function saveConfig() {
    saving.value = true
    try {
      await window.wails.Call({ packageName: 'main', structName: 'App', methodName: 'SaveConfig', args: [config.value] })
    } catch (e) {
      console.error('saveConfig failed:', e)
      throw e
    } finally {
      saving.value = false
    }
  }

  function addInterface(name) {
    config.value.interfaces.push({ name, routes: [] })
  }

  function removeInterface(index) {
    config.value.interfaces.splice(index, 1)
  }

  function addRoute(ifaceIndex, route) {
    config.value.interfaces[ifaceIndex].routes.push(route)
  }

  function removeRoute(ifaceIndex, routeIndex) {
    config.value.interfaces[ifaceIndex].routes.splice(routeIndex, 1)
  }

  return {
    config,
    configPath,
    saving,
    fetchConfig,
    fetchConfigPath,
    saveConfig,
    addInterface,
    removeInterface,
    addRoute,
    removeRoute,
  }
})
```

- [ ] **Step 4: Commit**

```bash
git add frontend/src/stores/
git commit -m "feat: add Pinia stores for monitor, logs, and config state"
```

---

## Task 11: Dashboard view — interface status cards

**Files:**
- Modify: `frontend/src/views/Dashboard.vue`

- [ ] **Step 1: Implement Dashboard view**

Replace `frontend/src/views/Dashboard.vue`:

```vue
<script setup>
import { onMounted, onUnmounted, ref, computed } from 'vue'
import { useMonitorStore } from '../stores/monitor'

const monitor = useMonitorStore()
const expanded = ref({})

onMounted(async () => {
  await monitor.fetchStatus()
  window.wails.Events.On('interface:status-changed', (event) => {
    monitor.updateInterfaceStatus(event.data)
  })
})

const uptime = computed(() => {
  if (!monitor.status.startedAt) return '—'
  const ms = Date.now() - new Date(monitor.status.startedAt).getTime()
  const s = Math.floor(ms / 1000)
  const h = Math.floor(s / 3600)
  const m = Math.floor((s % 3600) / 60)
  if (h > 0) return `${h}h ${m}m`
  return `${m}m ${s % 60}s`
})

function toggle(name) {
  expanded.value[name] = !expanded.value[name]
}
</script>

<template>
  <div>
    <h1>Dashboard</h1>

    <div style="display: flex; gap: 16px; margin-bottom: 20px;">
      <div class="card" style="flex: 1; text-align: center;">
        <div style="color: var(--text-secondary); font-size: 12px; margin-bottom: 4px;">Status</div>
        <span class="badge" :class="monitor.status.running ? 'badge-success' : 'badge-error'">
          {{ monitor.status.running ? 'Running' : 'Stopped' }}
        </span>
      </div>
      <div class="card" style="flex: 1; text-align: center;">
        <div style="color: var(--text-secondary); font-size: 12px; margin-bottom: 4px;">Active</div>
        <div style="font-size: 20px; font-weight: 600;">{{ monitor.activeCount }} / {{ monitor.status.interfaces.length }}</div>
      </div>
      <div class="card" style="flex: 1; text-align: center;">
        <div style="color: var(--text-secondary); font-size: 12px; margin-bottom: 4px;">Routes</div>
        <div style="font-size: 20px; font-weight: 600;">{{ monitor.totalRoutes }}</div>
      </div>
      <div class="card" style="flex: 1; text-align: center;">
        <div style="color: var(--text-secondary); font-size: 12px; margin-bottom: 4px;">Uptime</div>
        <div style="font-size: 20px; font-weight: 600;">{{ uptime }}</div>
      </div>
    </div>

    <div style="display: flex; justify-content: space-between; align-items: center; margin-bottom: 12px;">
      <h2 style="margin-bottom: 0;">Interfaces</h2>
      <div style="display: flex; gap: 8px;">
        <button class="btn btn-primary" v-if="!monitor.status.running" @click="monitor.startMonitor()">Start</button>
        <button class="btn btn-danger" v-if="monitor.status.running" @click="monitor.stopMonitor()">Stop</button>
      </div>
    </div>

    <div v-if="monitor.status.interfaces.length === 0" class="card">
      <p style="color: var(--text-secondary); text-align: center; padding: 20px;">
        No interfaces configured. Go to Routes to add interfaces.
      </p>
    </div>

    <div v-for="iface in monitor.status.interfaces" :key="iface.interfaceName" class="card" style="cursor: pointer;" @click="toggle(iface.interfaceName)">
      <div style="display: flex; align-items: center; justify-content: space-between;">
        <div style="display: flex; align-items: center; gap: 10px;">
          <span style="font-size: 10px;" :style="{ color: iface.connected ? 'var(--success)' : 'var(--error)' }">●</span>
          <span style="font-weight: 600;">{{ iface.interfaceName }}</span>
          <span class="badge" :class="iface.connected ? 'badge-success' : 'badge-error'">
            {{ iface.connected ? 'Connected' : 'Disconnected' }}
          </span>
        </div>
        <div style="display: flex; align-items: center; gap: 12px;">
          <span style="color: var(--text-secondary); font-size: 13px;">{{ iface.routes?.length || 0 }} routes</span>
          <span style="color: var(--text-secondary);">{{ expanded[iface.interfaceName] ? '▼' : '▶' }}</span>
        </div>
      </div>

      <div v-if="expanded[iface.interfaceName]" style="margin-top: 12px; border-top: 1px solid var(--border-color); padding-top: 12px;">
        <div v-if="iface.gateway" style="color: var(--text-secondary); font-size: 13px; margin-bottom: 8px;">
          Gateway: <span style="color: var(--text-primary);">{{ iface.gateway }}</span>
        </div>
        <div v-for="route in iface.routes" :key="route.ip" style="display: flex; align-items: center; gap: 8px; padding: 4px 0; font-size: 13px;">
          <span style="font-size: 8px;" :style="{ color: route.active ? 'var(--success)' : 'var(--text-secondary)' }">●</span>
          <span style="color: var(--text-link); font-family: var(--font-mono);">{{ route.for }}</span>
          <span v-if="route.for !== route.ip" style="color: var(--text-secondary);">→ {{ route.ip }}</span>
        </div>
        <div v-if="!iface.routes?.length" style="color: var(--text-secondary); font-size: 13px;">
          No routes configured
        </div>
      </div>
    </div>
  </div>
</template>
```

- [ ] **Step 2: Build and verify**

```bash
cd frontend && npm run build && cd ..
```

Expected: builds without errors.

- [ ] **Step 3: Commit**

```bash
git add frontend/src/views/Dashboard.vue
git commit -m "feat: implement Dashboard view with interface status cards"
```

---

## Task 12: Routes view — config editor

**Files:**
- Modify: `frontend/src/views/Routes.vue`

- [ ] **Step 1: Implement Routes view**

Replace `frontend/src/views/Routes.vue`:

```vue
<script setup>
import { onMounted, ref } from 'vue'
import { useConfigStore } from '../stores/config'

const configStore = useConfigStore()
const newIfaceName = ref('')
const newRoutes = ref({})
const saveMessage = ref('')

onMounted(async () => {
  await configStore.fetchConfig()
})

function addInterface() {
  const name = newIfaceName.value.trim()
  if (!name) return
  configStore.addInterface(name)
  newIfaceName.value = ''
}

function addRoute(ifaceIndex) {
  const route = (newRoutes.value[ifaceIndex] || '').trim()
  if (!route) return
  if (!validateRoute(route)) return
  configStore.addRoute(ifaceIndex, route)
  newRoutes.value[ifaceIndex] = ''
}

function validateRoute(route) {
  const ipv4 = /^\d{1,3}\.\d{1,3}\.\d{1,3}\.\d{1,3}$/
  const cidr = /^\d{1,3}\.\d{1,3}\.\d{1,3}\.\d{1,3}\/\d{1,2}$/
  const domain = /^[a-zA-Z0-9]([a-zA-Z0-9-]*[a-zA-Z0-9])?(\.[a-zA-Z0-9]([a-zA-Z0-9-]*[a-zA-Z0-9])?)*\.[a-zA-Z]{2,}$/
  return ipv4.test(route) || cidr.test(route) || domain.test(route)
}

async function save() {
  try {
    await configStore.saveConfig()
    saveMessage.value = 'Saved successfully'
    setTimeout(() => { saveMessage.value = '' }, 2000)
  } catch (e) {
    saveMessage.value = 'Save failed: ' + e
  }
}
</script>

<template>
  <div>
    <div style="display: flex; justify-content: space-between; align-items: center; margin-bottom: 20px;">
      <h1 style="margin-bottom: 0;">Route Configuration</h1>
      <div style="display: flex; align-items: center; gap: 12px;">
        <span v-if="saveMessage" style="color: var(--success); font-size: 13px;">{{ saveMessage }}</span>
        <button class="btn btn-primary" @click="save" :disabled="configStore.saving">
          {{ configStore.saving ? 'Saving...' : 'Save & Apply' }}
        </button>
      </div>
    </div>

    <div style="display: flex; gap: 8px; margin-bottom: 20px;">
      <input v-model="newIfaceName" placeholder="Interface name (e.g. ppp0)" @keyup.enter="addInterface" style="flex: 1;" />
      <button class="btn" @click="addInterface">Add Interface</button>
    </div>

    <div v-if="configStore.config.interfaces.length === 0" class="card">
      <p style="color: var(--text-secondary); text-align: center; padding: 20px;">
        No interfaces configured. Add one above.
      </p>
    </div>

    <div v-for="(iface, ifaceIdx) in configStore.config.interfaces" :key="ifaceIdx" class="card">
      <div style="display: flex; justify-content: space-between; align-items: center; margin-bottom: 12px;">
        <h2 style="margin-bottom: 0;">{{ iface.name }}</h2>
        <button class="btn btn-danger" @click="configStore.removeInterface(ifaceIdx)" style="font-size: 12px;">Remove</button>
      </div>

      <div v-for="(route, routeIdx) in iface.routes" :key="routeIdx"
           style="display: flex; align-items: center; justify-content: space-between; padding: 6px 10px; background: var(--bg-primary); border-radius: var(--radius); margin-bottom: 4px;">
        <span style="font-family: var(--font-mono); font-size: 13px; color: var(--text-link);">{{ route }}</span>
        <button @click="configStore.removeRoute(ifaceIdx, routeIdx)"
                style="background: none; border: none; color: var(--text-secondary); cursor: pointer; font-size: 16px; padding: 0 4px;"
                title="Remove route">×</button>
      </div>

      <div style="display: flex; gap: 8px; margin-top: 8px;">
        <input v-model="newRoutes[ifaceIdx]"
               placeholder="Domain, IP, or CIDR (e.g. github.com, 192.168.1.0/24)"
               @keyup.enter="addRoute(ifaceIdx)"
               style="flex: 1; font-size: 13px;" />
        <button class="btn" @click="addRoute(ifaceIdx)" style="font-size: 13px;">Add</button>
      </div>
    </div>
  </div>
</template>
```

- [ ] **Step 2: Build and verify**

```bash
cd frontend && npm run build && cd ..
```

Expected: builds without errors.

- [ ] **Step 3: Commit**

```bash
git add frontend/src/views/Routes.vue
git commit -m "feat: implement Routes view with config editor and validation"
```

---

## Task 13: Logs view — real-time log viewer

**Files:**
- Modify: `frontend/src/views/Logs.vue`

- [ ] **Step 1: Implement Logs view**

Replace `frontend/src/views/Logs.vue`:

```vue
<script setup>
import { onMounted, onUnmounted, ref, watch, nextTick } from 'vue'
import { useLogStore } from '../stores/logs'

const logStore = useLogStore()
const logContainer = ref(null)
const autoScroll = ref(true)

onMounted(async () => {
  await logStore.fetchRecent()
  window.wails.Events.On('log:new', (event) => {
    logStore.addEntry(event.data)
  })
  scrollToBottom()
})

watch(() => logStore.filtered.length, () => {
  if (autoScroll.value) {
    nextTick(scrollToBottom)
  }
})

function scrollToBottom() {
  if (logContainer.value) {
    logContainer.value.scrollTop = logContainer.value.scrollHeight
  }
}

function onScroll() {
  if (!logContainer.value) return
  const el = logContainer.value
  const atBottom = el.scrollHeight - el.scrollTop - el.clientHeight < 30
  autoScroll.value = atBottom
}

function formatTime(t) {
  const d = new Date(t)
  return d.toLocaleTimeString('en-US', { hour12: false }) + '.' + String(d.getMilliseconds()).padStart(3, '0')
}

function levelColor(level) {
  switch (level) {
    case 'error': return 'var(--error)'
    case 'warn': return 'var(--warning)'
    case 'debug': return 'var(--text-secondary)'
    default: return 'var(--text-primary)'
  }
}
</script>

<template>
  <div style="display: flex; flex-direction: column; height: 100%;">
    <div style="display: flex; justify-content: space-between; align-items: center; margin-bottom: 12px;">
      <h1 style="margin-bottom: 0;">Logs</h1>
      <div style="display: flex; align-items: center; gap: 8px;">
        <input v-model="logStore.searchQuery" placeholder="Search..." style="width: 200px; font-size: 13px;" />
        <select v-model="logStore.levelFilter" style="font-size: 13px;">
          <option value="all">All levels</option>
          <option value="error">Error</option>
          <option value="warn">Warning</option>
          <option value="info">Info</option>
          <option value="debug">Debug</option>
        </select>
        <button class="btn" @click="logStore.clear()" style="font-size: 12px;">Clear</button>
      </div>
    </div>

    <div ref="logContainer" @scroll="onScroll"
         style="flex: 1; overflow-y: auto; background: var(--bg-secondary); border: 1px solid var(--border-color); border-radius: var(--radius); padding: 8px; font-family: var(--font-mono); font-size: 12px; line-height: 1.7;">
      <div v-if="logStore.filtered.length === 0" style="color: var(--text-secondary); text-align: center; padding: 40px;">
        No log entries
      </div>
      <div v-for="(entry, idx) in logStore.filtered" :key="idx" style="display: flex; gap: 8px; white-space: nowrap;">
        <span style="color: var(--text-secondary); flex-shrink: 0;">{{ formatTime(entry.time) }}</span>
        <span :style="{ color: levelColor(entry.level), flexShrink: 0, width: '44px', textAlign: 'right' }">[{{ entry.level }}]</span>
        <span style="white-space: pre-wrap; word-break: break-all;">{{ entry.message }}</span>
      </div>
    </div>

    <div style="display: flex; justify-content: space-between; align-items: center; margin-top: 8px; font-size: 12px; color: var(--text-secondary);">
      <span>{{ logStore.filtered.length }} entries{{ logStore.levelFilter !== 'all' ? ' (filtered)' : '' }}</span>
      <span :style="{ color: autoScroll ? 'var(--success)' : 'var(--text-secondary)' }">
        {{ autoScroll ? 'Auto-scroll ON' : 'Auto-scroll paused — scroll to bottom to resume' }}
      </span>
    </div>
  </div>
</template>
```

- [ ] **Step 2: Build and verify**

```bash
cd frontend && npm run build && cd ..
```

Expected: builds without errors.

- [ ] **Step 3: Commit**

```bash
git add frontend/src/views/Logs.vue
git commit -m "feat: implement Logs view with real-time streaming and filters"
```

---

## Task 14: Settings view — auto-start, notifications, about

**Files:**
- Modify: `frontend/src/views/Settings.vue`

- [ ] **Step 1: Implement Settings view**

Replace `frontend/src/views/Settings.vue`:

```vue
<script setup>
import { onMounted, ref } from 'vue'
import { useConfigStore } from '../stores/config'

const configStore = useConfigStore()
const autoStart = ref(false)
const notifications = ref(true)
const loading = ref(true)

onMounted(async () => {
  await configStore.fetchConfigPath()
  try {
    const result = await window.wails.Call({ packageName: 'main', structName: 'App', methodName: 'GetAutoStart' })
    autoStart.value = result
  } catch (e) {
    console.error('GetAutoStart failed:', e)
  }
  loading.value = false
})

async function toggleAutoStart() {
  autoStart.value = !autoStart.value
  try {
    await window.wails.Call({ packageName: 'main', structName: 'App', methodName: 'SetAutoStart', args: [autoStart.value] })
  } catch (e) {
    console.error('SetAutoStart failed:', e)
    autoStart.value = !autoStart.value
  }
}

function toggleNotifications() {
  notifications.value = !notifications.value
}
</script>

<template>
  <div>
    <h1>Settings</h1>

    <div class="card">
      <h2>General</h2>

      <div style="display: flex; justify-content: space-between; align-items: center; padding: 12px 0; border-bottom: 1px solid var(--border-color);">
        <div>
          <div style="font-weight: 500;">Launch at startup</div>
          <div style="color: var(--text-secondary); font-size: 13px;">Automatically start NetCatcher when you log in</div>
        </div>
        <div class="toggle" :class="{ active: autoStart }" @click="toggleAutoStart"></div>
      </div>

      <div style="display: flex; justify-content: space-between; align-items: center; padding: 12px 0;">
        <div>
          <div style="font-weight: 500;">Notifications</div>
          <div style="color: var(--text-secondary); font-size: 13px;">Show system notifications when interfaces connect or disconnect</div>
        </div>
        <div class="toggle" :class="{ active: notifications }" @click="toggleNotifications"></div>
      </div>
    </div>

    <div class="card" style="margin-top: 16px;">
      <h2>Configuration</h2>
      <div style="padding: 8px 0;">
        <div style="color: var(--text-secondary); font-size: 13px; margin-bottom: 4px;">Config file location</div>
        <div style="font-family: var(--font-mono); font-size: 13px; color: var(--text-link); background: var(--bg-primary); padding: 8px 12px; border-radius: var(--radius);">
          {{ configStore.configPath }}
        </div>
      </div>
    </div>

    <div class="card" style="margin-top: 16px;">
      <h2>About</h2>
      <div style="padding: 8px 0; color: var(--text-secondary); font-size: 13px;">
        <div style="margin-bottom: 4px;"><span style="color: var(--text-primary);">NetCatcher</span> — Network route manager</div>
        <div style="margin-bottom: 4px;">Version: 1.0.0</div>
        <div>
          <a href="https://github.com/attson/netcatcher" target="_blank" style="color: var(--text-link); text-decoration: none;">GitHub Repository</a>
        </div>
      </div>
    </div>
  </div>
</template>
```

- [ ] **Step 2: Build and verify**

```bash
cd frontend && npm run build && cd ..
```

Expected: builds without errors.

- [ ] **Step 3: Commit**

```bash
git add frontend/src/views/Settings.vue
git commit -m "feat: implement Settings view with auto-start and notifications"
```

---

## Task 15: System notifications — beeep integration

**Files:**
- Create: `notify.go`

- [ ] **Step 1: Add beeep dependency**

```bash
go get github.com/gen2brain/beeep
go mod tidy
```

- [ ] **Step 2: Create notify.go**

Create `notify.go`:

```go
package main

import (
	"fmt"

	nc "netcatcher/netcatcher"

	"github.com/gen2brain/beeep"
)

type Notifier struct {
	enabled bool
}

func NewNotifier() *Notifier {
	return &Notifier{enabled: true}
}

func (n *Notifier) SetEnabled(enabled bool) {
	n.enabled = enabled
}

func (n *Notifier) OnStatusChange(status nc.InterfaceStatus) {
	if !n.enabled {
		return
	}

	var title, message string
	if status.Connected {
		title = "Interface Connected"
		message = fmt.Sprintf("%s is now online (gateway: %s)", status.InterfaceName, status.Gateway)
	} else {
		title = "Interface Disconnected"
		message = fmt.Sprintf("%s is now offline", status.InterfaceName)
	}

	_ = beeep.Notify(title, message, "")
}
```

- [ ] **Step 3: Wire notifier into App**

Update `app.go` — add a `notifier` field to the `App` struct and call it from the status callback. Add the field:

In `app.go`, add `notifier *Notifier` to the `App` struct and update `NewApp`:

```go
// In the App struct, add:
notifier *Notifier

// In NewApp, after creating the manager:
a.notifier = NewNotifier()

// Update the manager's StatusChangeCallback to also notify:
a.manager = NewManager(cfg, func(status nc.InterfaceStatus) {
    if a.app != nil {
        a.app.EmitEvent("interface:status-changed", status)
    }
    a.notifier.OnStatusChange(status)
})
```

Also add a binding method to control notifications:

```go
func (a *App) SetNotifications(enabled bool) {
    a.notifier.SetEnabled(enabled)
}

func (a *App) GetNotifications() bool {
    return a.notifier.enabled
}
```

- [ ] **Step 4: Verify compilation**

```bash
go build -o /dev/null .
```

Expected: compiles.

- [ ] **Step 5: Commit**

```bash
git add notify.go app.go go.mod go.sum
git commit -m "feat: add system notifications via beeep for interface status changes"
```

---

## Task 16: Route connectivity test — ping integration

**Files:**
- Modify: `frontend/src/views/Dashboard.vue`

- [ ] **Step 1: Add ping button to Dashboard route details**

In `frontend/src/views/Dashboard.vue`, add a ping function and button to each route in the expanded section. Add to the `<script setup>`:

```js
const pingResults = ref({})

async function pingRoute(host) {
  pingResults.value[host] = { loading: true }
  try {
    const result = await window.wails.Call({ packageName: 'main', structName: 'App', methodName: 'PingRoute', args: [host] })
    pingResults.value[host] = result
  } catch (e) {
    pingResults.value[host] = { reachable: false, error: e.toString() }
  }
}
```

Update the route list item in the template to include a ping button and result display. Replace the route `v-for` div:

```html
<div v-for="route in iface.routes" :key="route.ip"
     style="display: flex; align-items: center; gap: 8px; padding: 4px 0; font-size: 13px;">
  <span style="font-size: 8px;" :style="{ color: route.active ? 'var(--success)' : 'var(--text-secondary)' }">●</span>
  <span style="color: var(--text-link); font-family: var(--font-mono);">{{ route.for }}</span>
  <span v-if="route.for !== route.ip" style="color: var(--text-secondary);">→ {{ route.ip }}</span>
  <button class="btn" style="font-size: 11px; padding: 2px 8px; margin-left: auto;" @click.stop="pingRoute(route.for)">
    {{ pingResults[route.for]?.loading ? '...' : 'Ping' }}
  </button>
  <span v-if="pingResults[route.for] && !pingResults[route.for].loading" style="font-size: 12px;"
        :style="{ color: pingResults[route.for].reachable ? 'var(--success)' : 'var(--error)' }">
    {{ pingResults[route.for].reachable ? pingResults[route.for].latency : 'Unreachable' }}
  </span>
</div>
```

- [ ] **Step 2: Build and verify**

```bash
cd frontend && npm run build && cd ..
```

Expected: builds without errors.

- [ ] **Step 3: Commit**

```bash
git add frontend/src/views/Dashboard.vue
git commit -m "feat: add route ping connectivity test to Dashboard"
```

---

## Task 17: Event wiring — connect all Wails Events in App.vue

**Files:**
- Modify: `frontend/src/App.vue`

- [ ] **Step 1: Add global event listeners in App.vue**

Update `frontend/src/App.vue` to set up global event listeners on mount:

```vue
<script setup>
import { onMounted } from 'vue'
import { useMonitorStore } from './stores/monitor'
import { useLogStore } from './stores/logs'

const monitor = useMonitorStore()
const logStore = useLogStore()

onMounted(async () => {
  await monitor.fetchStatus()
  await logStore.fetchRecent()

  window.wails.Events.On('monitor:started', () => {
    monitor.fetchStatus()
  })
  window.wails.Events.On('monitor:stopped', () => {
    monitor.fetchStatus()
  })
})
</script>

<template>
  <div class="titlebar">
    <span class="titlebar-title">NetCatcher</span>
  </div>
  <div class="layout">
    <nav class="sidebar">
      <ul class="sidebar-nav">
        <li><router-link to="/">Dashboard</router-link></li>
        <li><router-link to="/routes">Routes</router-link></li>
        <li><router-link to="/logs">Logs</router-link></li>
        <li><router-link to="/settings">Settings</router-link></li>
      </ul>
      <div style="margin-top: auto; padding: 12px 16px; border-top: 1px solid var(--border-color);">
        <span class="badge" :class="monitor.status.running ? 'badge-success' : 'badge-error'" style="font-size: 11px;">
          {{ monitor.status.running ? 'Monitoring' : 'Stopped' }}
        </span>
      </div>
    </nav>
    <main class="content">
      <router-view />
    </main>
  </div>
</template>
```

- [ ] **Step 2: Build and verify**

```bash
cd frontend && npm run build && cd ..
```

Expected: builds without errors.

- [ ] **Step 3: Commit**

```bash
git add frontend/src/App.vue
git commit -m "feat: wire global Wails event listeners in App.vue"
```

---

## Task 18: Update .gitignore and clean up old files

**Files:**
- Modify: `.gitignore`
- Delete: `install/darwin.sh` (replaced by in-app auto-start)

- [ ] **Step 1: Update .gitignore**

Add these entries to `.gitignore`:

```
# Wails
frontend/dist/
frontend/node_modules/
wailsjs/

# Build artifacts
build/bin/

# Superpowers
.superpowers/
```

- [ ] **Step 2: Remove install/darwin.sh**

```bash
rm install/darwin.sh
rmdir install/ 2>/dev/null || true
```

- [ ] **Step 3: Commit**

```bash
git add .gitignore
git rm install/darwin.sh
git commit -m "chore: update .gitignore for Wails, remove old install script"
```

---

## Task 19: Update CI — GitHub Actions for Wails build

**Files:**
- Modify: `.github/workflows/release.yaml`
- Delete (or archive): `.goreleaser.yaml`

- [ ] **Step 1: Replace release.yaml with Wails build workflow**

Replace `.github/workflows/release.yaml`:

```yaml
name: Release

on:
  push:
    tags:
      - "v*"

jobs:
  build:
    strategy:
      matrix:
        include:
          - os: macos-latest
            platform: darwin
            arch: universal
          - os: windows-latest
            platform: windows
            arch: amd64
    runs-on: ${{ matrix.os }}
    steps:
      - uses: actions/checkout@v4

      - name: Set up Go
        uses: actions/setup-go@v5
        with:
          go-version: '1.22'

      - name: Set up Node.js
        uses: actions/setup-node@v4
        with:
          node-version: '20'

      - name: Install Wails v3
        run: go install github.com/wailsapp/wails/v3/cmd/wails3@latest

      - name: Install frontend dependencies
        run: cd frontend && npm ci

      - name: Build
        run: wails3 build

      - name: Upload artifacts
        uses: actions/upload-artifact@v4
        with:
          name: netcatcher-${{ matrix.platform }}-${{ matrix.arch }}
          path: build/bin/*

  release:
    needs: build
    runs-on: ubuntu-latest
    permissions:
      contents: write
    steps:
      - name: Download all artifacts
        uses: actions/download-artifact@v4

      - name: Create Release
        uses: softprops/action-gh-release@v1
        with:
          files: netcatcher-*/*
          generate_release_notes: true
```

- [ ] **Step 2: Remove .goreleaser.yaml**

```bash
rm .goreleaser.yaml
```

- [ ] **Step 3: Commit**

```bash
git add .github/workflows/release.yaml
git rm .goreleaser.yaml
git commit -m "chore: replace GoReleaser with Wails build in GitHub Actions"
```

---

## Task 20: End-to-end smoke test

- [ ] **Step 1: Build the full application**

```bash
cd /Users/attson/code/github.com:attson/netcatcher
cd frontend && npm ci && npm run build && cd ..
go build -o build/bin/netcatcher-app .
```

Expected: compiles successfully.

- [ ] **Step 2: Run the application**

```bash
./build/bin/netcatcher-app
```

Expected:
- Window opens with GitHub Dark theme
- Sidebar shows Dashboard / Routes / Logs / Settings
- System tray icon appears
- Dashboard shows "Stopped" status (or "Running" if monitor auto-starts)
- Navigate to each page — no crashes

- [ ] **Step 3: Test core flows**

1. Go to Routes → add an interface → add a route → click "Save & Apply"
2. Go to Dashboard → verify interface card appears → click Start
3. Go to Logs → verify log entries appear
4. Go to Settings → toggle auto-start → verify plist/registry created
5. Close window → verify app minimizes to tray (not quit)
6. Click tray → "Show Window" → verify window reappears

- [ ] **Step 4: Final commit**

```bash
git add -A
git commit -m "chore: end-to-end verification complete"
```

---

## Post-Implementation

After all tasks are complete:
- Update `CLAUDE.md` with new build/run instructions for Wails
- Update `README.md` with new installation and usage instructions
- Tag the release: `git tag v1.0.0`
