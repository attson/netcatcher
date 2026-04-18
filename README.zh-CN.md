# NetCatcher

[English](README.md)

NetCatcher 是一个桌面应用，用于监控网络接口并在连接时自动添加静态路由。适用于多网卡或多 VPN 场景下，需要将特定流量路由到指定接口的情况。

## 功能

- 仪表盘展示活跃接口及路由状态
- 路由配置编辑器 — 通过 GUI 管理接口和路由规则
- 实时日志查看器 — 实时查看路由添加/删除事件
- 路由连通性测试（Ping）
- 接口连接/断开时系统通知
- 多语言支持（中文 / English）
- 设置面板（开机自启、通知、语言切换）
- 系统托盘 — 关闭窗口时隐藏到托盘，通过托盘菜单退出

## 环境要求

- Go 1.25+
- Node.js 20+
- Wails v3 CLI

```bash
go install github.com/wailsapp/wails/v3/cmd/wails3@latest
```

## 从源码构建

```bash
# 克隆仓库
git clone https://github.com/attson/netcatcher.git
cd netcatcher

# 构建前端
cd frontend && npm ci && npx vite build && cd ..

# 构建 Go 二进制
go build -o build/bin/netcatcher-app .

# 或使用 Wails CLI（自动处理前端和后端）
wails3 build
```

## 运行

应用需要管理员/root 权限来修改路由表。

```bash
sudo ./build/bin/netcatcher-app
```

Windows 下以管理员身份运行可执行文件。

## 配置

配置文件存储在平台标准路径下，可通过 GUI 设置管理：

- macOS: `~/Library/Application Support/NetCatcher/config.json`
- Windows: `%APPDATA%\NetCatcher\config.json`

配置格式为 JSON。支持域名（连接时通过 DNS 解析）、IP 地址和 CIDR 地址段：

```json
{
  "interfaces": [
    {
      "name": "ppp0",
      "routes": [
        "github.com",
        "192.168.188.11",
        "192.168.188.0/24"
      ]
    }
  ]
}
```

`name` 字段必须与操作系统中的网络接口名称完全一致（如 VPN 适配器名称）。

## 平台说明

### macOS

应用需要 root 权限来调用 `route` 命令。使用 `sudo` 启动，或为生产构建授予相应权限。

### Windows

默认情况下，VPN 连接会将所有流量路由到隧道。要使用按路由配置：

1. 打开 VPN 适配器属性（网络设置 -> VPN 连接 -> 右键 -> 属性 -> 网络）。
2. 选择 Internet 协议版本 4 (TCP/IPv4) -> 属性 -> 高级。
3. 取消勾选"在远程网络上使用默认网关"。如有需要，对 IPv6 重复此操作。

这将阻止所有流量通过 VPN 发送，让 NetCatcher 的静态路由控制哪些目标使用隧道。

## 注意事项

- 如果本地 DNS 代理或全局代理处于活动状态，域名可能解析到错误的 IP 地址。如果依赖基于域名的路由，请在启动 NetCatcher 前禁用代理。
- 路由在每次接口连接时重新解析，因此接口重连时会自动获取 DNS 变更。
