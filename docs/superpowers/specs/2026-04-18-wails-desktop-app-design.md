# NetCatcher Desktop App Design

## Overview

将 NetCatcher 从 CLI 守护进程改造为 Wails + Vue 跨平台桌面应用。采用直接整合方案，现有 Go 代码重构为 Wails 应用的后端模块。

## Goals

- 安装即用的桌面应用，无需命令行操作
- 系统托盘常驻，关闭窗口最小化到托盘
- GUI 管理路由配置、查看接口状态、实时日志
- 先支持 macOS + Windows，Linux 后续补充

## Tech Stack

- **Backend**: Go + Wails v3 (alpha, required for native system tray support — v2 does not support systray)
- **Frontend**: Vue 3 (Composition API) + Vite + Vue Router + Pinia
- **UI Theme**: GitHub Dark 深色主题（深蓝黑底，蓝色强调色，手写样式，不引入重型 UI 库）
- **Build**: Wails CLI + GoReleaser + GitHub Actions

## Project Structure

```
netcatcher/
├── main.go                    # Wails 应用入口，初始化窗口、托盘、绑定服务
├── app.go                     # Wails App struct，暴露给前端的 binding 方法
├── config/
│   └── config.go              # 现有配置结构，扩展 CRUD 方法
├── netcatcher/
│   └── netcatcher.go          # 核心逻辑，重构为可启停的 Manager/Service
├── route/
│   ├── route_darwin.go        # macOS 路由（保持现有）
│   └── route_windows.go       # Windows 路由（保持现有）
├── tray/
│   └── tray.go                # 系统托盘（图标、右键菜单）
├── frontend/
│   ├── index.html
│   ├── package.json
│   ├── src/
│   │   ├── App.vue            # 主布局（侧边栏 + 内容区）
│   │   ├── views/
│   │   │   ├── Dashboard.vue  # 接口状态总览
│   │   │   ├── Routes.vue     # 路由配置编辑
│   │   │   ├── Logs.vue       # 实时日志查看
│   │   │   └── Settings.vue   # 开机自启、通知等设置
│   │   ├── components/        # 可复用组件
│   │   └── styles/            # GitHub Dark 主题样式
│   └── wailsjs/               # Wails 自动生成的 JS binding
├── build/                     # Wails 构建配置、图标
└── wails.json                 # Wails 项目配置
```

## Backend Architecture

### App struct (Binding Layer)

`app.go` 是前后端的唯一桥梁，暴露以下方法给前端：

```go
type App struct {
    ctx        context.Context
    manager    *netcatcher.Manager
    configPath string
}

// 配置管理
func (a *App) GetConfig() config.Config
func (a *App) SaveConfig(cfg config.Config) error
func (a *App) GetConfigPath() string

// 服务控制
func (a *App) StartMonitor() error
func (a *App) StopMonitor() error
func (a *App) GetStatus() MonitorStatus

// 路由测试
func (a *App) PingRoute(host string) PingResult

// 日志
func (a *App) GetRecentLogs(count int) []LogEntry

// 系统设置
func (a *App) GetAutoStart() bool
func (a *App) SetAutoStart(enabled bool) error
```

### Manager Refactor

现有 `netcatcher.go` 重构为 `Manager`：

- 管理所有 `NetCatcher` 实例的生命周期
- 支持 `Start()` / `Stop()` / `Restart()` 控制
- 状态变更时通过 `runtime.EventsEmit` 推送事件到前端
- 日志写入内存环形缓冲区（最近 1000 条），同时支持文件输出

### Events (Backend → Frontend)

- `interface:status-changed` — 接口连接/断开
- `log:new` — 新日志条目
- `monitor:started` / `monitor:stopped` — 监控服务状态变更

### Config File Location

App 化后，配置文件从当前目录改为平台标准路径：
- macOS: `~/Library/Application Support/NetCatcher/config.json`
- Windows: `%APPDATA%\NetCatcher\config.json`

首次启动时，如果标准路径下不存在配置文件，创建默认空配置。

### Auto Start

- macOS: `~/Library/LaunchAgents/com.attson.netcatcher.plist`（用户级）
- Windows: 注册表 `HKCU\Software\Microsoft\Windows\CurrentVersion\Run`

### Permission Handling

路由操作需要管理员权限。App 启动时检测权限，权限不足时提示用户：
- macOS: 弹出系统授权对话框
- Windows: UAC 提升

## Frontend Architecture

### Pages

**1. Dashboard**
- 接口状态卡片（名称、连接状态、网关 IP、路由数量）
- 点击卡片展开详情（该接口下所有路由及状态）
- 顶部汇总：活跃接口数 / 总路由数 / 运行时长

**2. Routes**
- 按接口分组展示路由规则
- 增删改：添加接口、添加/删除路由（域名、IP、CIDR）
- 输入校验：域名格式、IP 格式、CIDR 合法性
- 保存后自动重载配置，无需重启

**3. Logs**
- 实时滚动日志流（Wails Events 接收）
- 按级别筛选：info / warn / error / debug
- 搜索过滤
- 自动滚动 + 手动暂停

**4. Settings**
- 开机自启开关
- 通知开关
- 配置文件路径
- 关于信息（版本号、GitHub 链接）

### State Management

Pinia stores:
- `useMonitorStore` — 接口状态、路由状态
- `useLogStore` — 日志缓存
- `useConfigStore` — 配置数据

### Frontend-Backend Communication

- 前端调后端：`wailsjs/go/main/App.MethodName()` 返回 Promise
- 后端推前端：`runtime.EventsEmit` → 前端 `runtime.EventsOn` 监听

## System Tray

使用 Wails v2 内置的系统托盘支持（`menu.TrayMenu`）：

- 托盘图标：网络图标，连接状态切换颜色（绿色=活跃，灰色=无连接）
- 右键菜单：显示主窗口 / 启动·停止监控 / 分隔线 / 退出
- 关闭窗口 → 隐藏到托盘（`OnBeforeClose` 拦截，调用 `runtime.WindowHide`）
- 点击托盘菜单"显示主窗口" → `runtime.WindowShow`

## System Notifications

- 接口上线/掉线时发送系统原生通知
- macOS: `osascript` 或 `NSUserNotification`
- Windows: 托盘气泡通知
- 可在 Settings 中关闭

## Window Config

- 默认尺寸：900 x 600
- 最小尺寸：700 x 450
- 启动行为：开机自启时隐藏到托盘，手动启动显示窗口
- 无边框窗口 + 自定义标题栏（GitHub Dark 主题一致）

## Build & Distribution

- `wails build` 生成平台原生包
- macOS: `.app` bundle，可选 `.dmg`
- Windows: `.exe`，可选 NSIS 安装包
- GoReleaser 适配 Wails 构建产物
- GitHub Actions CI：macOS / Windows 分别构建

## Out of Scope (Future)

- Linux 平台支持（需新增 `route_linux.go` + Linux 打包）
- CLI headless 模式
- 多语言 i18n
