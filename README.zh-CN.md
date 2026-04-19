# NetCatcher

[English](README.md)

NetCatcher 是一个桌面应用，用于监控网络接口并在连接时自动添加静态路由。适用于多网卡或多 VPN 场景下，需要将特定流量路由到指定接口的情况。

## 功能

- 仪表盘展示活跃接口及路由状态；每个接口卡片有独立的 编辑 / 保存 / 取消 流程
- 按接口配置路由和 DNS 服务器 — 通过 GUI 管理
- 监测停止或接口断开时路由行显示为灰色 + 删除线，状态一目了然
- 实时日志查看器 — 实时查看路由添加/删除事件
- 路由连通性测试（Ping）
- **TUN 代理适配**（可选）— 本地 DNS 转发器 + `/etc/resolver/` 条目，让开启 TUN 模式代理（Clash / Mihomo / Surge）时域名路由也能解析到真实 IP
- 路由解析用的 DNS 查询绑定到监测接口（`IP_BOUND_IF` / `IP_UNICAST_IF`），绕开 TUN 的 DNS 劫持
- 接口连接/断开时系统通知
- 多语言支持（中文 / English）
- 设置面板（开机自启、通知、语言切换、TUN 代理适配）
- 系统托盘 — 关闭窗口时隐藏到托盘，通过托盘菜单退出

## 安装

从 [Releases](https://github.com/attson/netcatcher/releases) 页面下载最新版本：

| 平台 | 文件 |
|------|------|
| macOS (Apple Silicon) | `NetCatcher-arm64.dmg` |
| macOS (Intel) | `NetCatcher-amd64.dmg` |
| Windows | `NetCatcher-amd64.exe` |

**macOS：** 打开 `.dmg` 文件，将 NetCatcher 拖入"应用程序"文件夹。首次添加路由时会弹出密码输入框请求授权，之后无需重复输入。

> **提示 "NetCatcher 已损坏，无法打开"？** 应用只做了 ad-hoc 签名，没有经过 Apple Developer ID 公证，所以下载 DMG 时 macOS 加上的 quarantine 属性会被 Gatekeeper 拦截。执行一次以下命令即可：
>
> ```bash
> sudo xattr -cr /Applications/NetCatcher.app
> ```
>
> 然后正常打开。每次更新后重复一次即可。

**Windows：** 以管理员身份运行 `NetCatcher-amd64.exe`。

## 从源码构建

### 环境要求

- Go 1.25+
- Node.js 20+

```bash
# 克隆仓库
git clone https://github.com/attson/netcatcher.git
cd netcatcher

# 构建前端
cd frontend && npm ci && npx vite build && cd ..

# 构建 Go 二进制
go build -o build/bin/netcatcher-app .

# 运行
./build/bin/netcatcher-app
```

## 使用说明

### 仪表盘

主界面集成了状态监控和按接口的路由配置。系统网络接口以下拉列表展示，VPN 接口会显示服务名称（如 `ppp0 (我的VPN)`）方便识别。

- 从下拉列表选择接口，点击 **添加接口**，新卡片自动进入编辑态
- 在现有卡片上点击 **编辑** 进入编辑态；可增删路由（域名 / IP / CIDR）和 DNS 服务器（以 chip 形式显示在网关旁）
- 点击 **保存并应用** 提交当前卡片的修改，或 **取消** 放弃修改
- 使用 **Ping** 测试路由连通性；延迟显示在路由旁
- 使用 **启动** / **停止** 控制监控；未激活的路由会灰色 + 删除线，一眼看出哪些在生效
- 域名路由会在名字旁显示解析到的 IP 地址

![仪表盘](doc/screenshots/dashboard.png)

### 日志

实时日志查看器，显示接口上线/下线、路由添加/删除、错误等事件。可按日志级别筛选或关键词搜索。向上滚动会暂停自动滚动，滚动到底部恢复。

![日志](doc/screenshots/logs.png)

### 设置

- **开机启动** — 注册/取消注册开机自启。
- **通知** — 开启/关闭接口连接和断开时的系统通知。
- **语言** — 在中文和 English 之间切换。偏好设置会保存，重启后保持。
- **TUN 代理适配** — 本机在跑 TUN 模式代理（Clash / Mihomo / Surge）时打开。开启后 NetCatcher 会启动本地 DNS 转发器，并为每个配置的域名路由写一个 `/etc/resolver/<domain>` 条目；macOS 随后把这些域名的查询发给转发器（绑到 VPN 接口），拿到真实 IP 而不是 TUN 的 fake IP。普通环境下保持关闭即可。

![设置](doc/screenshots/settings.png)

### 系统托盘

应用运行在系统托盘中。关闭窗口会隐藏到托盘 — 应用继续在后台运行并监控。右键点击托盘图标可显示窗口或退出。

## 配置

配置文件存储在平台标准路径下，可通过 GUI 管理：

- macOS: `~/Library/Application Support/NetCatcher/config.json`
- Windows: `%APPDATA%\NetCatcher\config.json`

配置格式为 JSON。支持域名（连接时通过 DNS 解析）、IP 地址和 CIDR 地址段：

```json
{
  "tunMode": false,
  "interfaces": [
    {
      "name": "ppp0",
      "dns": ["114.114.114.114"],
      "routes": [
        "github.com",
        "192.168.188.11",
        "192.168.188.0/24"
      ]
    }
  ]
}
```

- `name` 字段必须与操作系统中的网络接口名称完全一致（如 VPN 适配器名称）。
- `dns`（可选）— 解析该接口域名路由时使用的 DNS 服务器，查询会绑定到该接口发出。开启 `tunMode` 时，本地 DNS 转发器也会用这个列表。
- `tunMode`（可选，默认 `false`）— 开启 TUN 代理适配流程，等同于设置页的开关。

## 平台说明

### macOS

首次需要权限时弹出一次密码输入框。该 osascript 会：

1. 在 `/usr/local/sbin/netcatcher-resolver-helper` 安装一个受限 helper（root 所有，0755）。该 helper 仅接受 `install <port> <domain>...` 和 `remove <domain>...` 两种命令，并对参数做严格校验。
2. 写入 `/etc/sudoers.d/netcatcher`，授予 `sudo /sbin/route` 和 `sudo -n netcatcher-resolver-helper` 的 NOPASSWD 规则。

之后每次路由变更和 `/etc/resolver/` 更新都通过 `sudo -n` 静默执行，不再弹授权。监测停止时会自动清理路由和 `/etc/resolver/` 条目。

### Windows

默认情况下，VPN 连接会将所有流量路由到隧道。要使用按路由配置：

1. 打开 VPN 适配器属性（网络设置 -> VPN 连接 -> 右键 -> 属性 -> 网络）。
2. 选择 Internet 协议版本 4 (TCP/IPv4) -> 属性 -> 高级。
3. 取消勾选"在远程网络上使用默认网关"。如有需要，对 IPv6 重复此操作。

这将阻止所有流量通过 VPN 发送，让 NetCatcher 的静态路由控制哪些目标使用隧道。

![vpn-info.png](doc/vpn-info.png)
![vpn-net.png](doc/vpn-net.png)
![modify- default.png](doc/modify-%20default.png)

## 注意事项

- 路由解析的 DNS 查询总是绑定到监测接口（`IP_BOUND_IF` / `IP_UNICAST_IF`），TUN 模式代理无法用 fake IP 劫持。绑接口查询失败时（DNS 不可达、未配 DNS 等）会回退到系统 resolver。
- 如果本机跑了 TUN 模式代理（Clash / Mihomo / Surge），且**应用层也要拿到真实 IP**（不仅仅是 NetCatcher 自己的路由解析），请在设置里打开 **TUN 代理适配**。不开的话，浏览器/终端查配置里的那些域名仍会拿到 fake IP，因为系统 DNS 路径在 utun 层被劫持了。
- 路由和 DNS 解析在每次接口连接时重新执行，因此接口重连会自动获取 DNS 变更。
