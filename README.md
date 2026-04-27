# Node Agent — VPN 节点控制系统技术文档

> 版本：1.5.0  
> 最后更新：2026-04-28

---

## 目录

- [1. 项目概述](#1-项目概述)
- [2. 系统架构](#2-系统架构)
  - [2.1 整体架构](#21-整体架构)
  - [2.2 Node Agent 在架构中的角色](#22-node-agent-在架构中的角色)
  - [2.3 数据流](#23-数据流)
- [3. 项目结构](#3-项目结构)
- [4. 核心模块详解](#4-核心模块详解)
  - [4.1 配置管理模块 (config)](#41-配置管理模块-config)
  - [4.2 sing-box 进程管理模块 (core/singbox)](#42-sing-box-进程管理模块-coresingbox)
  - [4.3 配置动态生成模块 (core/configgen)](#43-配置动态生成模块-coreconfiggen)
  - [4.4 流量统计模块 (core/stats)](#44-流量统计模块-corestats)
  - [4.5 设备限制模块 (core/device)](#45-设备限制模块-coredevice)
  - [4.6 心跳上报模块 (core/heartbeat)](#46-心跳上报模块-coreheartbeat)
  - [4.7 系统信息采集模块 (utils)](#47-系统信息采集模块-utils)
- [5. API 接口文档](#5-api-接口文档)
  - [5.1 认证方式](#51-认证方式)
  - [5.2 节点部署接口](#52-节点部署接口)
  - [5.3 节点状态接口](#53-节点状态接口)
  - [5.4 流量统计接口](#54-流量统计接口)
  - [5.5 按用户流量统计接口](#55-按用户流量统计接口)
  - [5.6 心跳数据接口](#56-心跳数据接口)
  - [5.7 进程控制接口](#57-进程控制接口)
  - [5.8 设备管理接口](#58-设备管理接口)
  - [5.9 日志查询接口](#59-日志查询接口)
  - [5.10 健康检查接口](#510-健康检查接口)
- [6. 安全机制](#6-安全机制)
  - [6.1 Token 认证](#61-token-认证)
  - [6.2 IP 白名单](#62-ip-白名单)
  - [6.3 Deploy 签名验证](#63-deploy-签名验证)
  - [6.4 中间件执行链](#64-中间件执行链)
- [7. 配置参考](#7-配置参考)
  - [7.1 完整配置文件](#71-完整配置文件)
  - [7.2 配置项说明](#72-配置项说明)
- [8. 部署指南](#8-部署指南)
  - [8.1 编译](#81-编译)
  - [8.2 一键安装](#82-一键安装)
  - [8.3 手动安装](#83-手动安装)
  - [8.4 systemd 管理](#84-systemd-管理)
- [9. 运维手册](#9-运维手册)
  - [9.1 日常运维命令](#91-日常运维命令)
  - [9.2 日志查看](#92-日志查看)
  - [9.3 故障排查](#93-故障排查)
  - [9.4 sing-box 配置模板](#94-sing-box-配置模板)
- [10. 扩展指南](#10-扩展指南)
  - [10.1 新增协议支持](#101-新增协议支持)
  - [10.2 自定义统计后端](#102-自定义统计后端)
  - [10.3 对接 Laravel 控制面](#103-对接-laravel-控制面)
  - [10.4 多节点负载均衡](#104-多节点负载均衡)
- [11. 设计决策与权衡](#11-设计决策与权衡)

---

## 1. 项目概述

Node Agent 是一个运行在每台 VPS 上的 VPN 节点控制守护进程，是整个 VPN 平台的**核心执行层**。它负责：

- **sing-box 生命周期管理**：启动、停止、重启、崩溃自动恢复
- **动态配置下发**：接收控制面指令，生成 sing-box 配置并热更新
- **多协议支持**：Hysteria2 / VLESS / Reality，可扩展
- **流量统计**：通过 Clash API 采集全局实时流量数据，通过 V2Ray API 采集按用户流量数据
- **设备限制**：用户级设备绑定与并发会话控制
- **心跳上报**：定时向控制面汇报节点状态、流量、在线用户数
- **安全防护**：Token 认证、IP 白名单、CORS 跨域、HMAC 签名验证
- **日志捕获**：sing-box 进程日志实时捕获与查询，崩溃原因追踪

技术栈：Go 1.21+，标准库 + gopsutil + gRPC，最小化外部依赖。

---

## 2. 系统架构

### 2.1 整体架构

```
┌─────────────────────────────────────────────────────────┐
│                    Laravel Control Plane                  │
│          (用户管理 / 套餐 / 计费 / 节点分配)              │
└──────────┬──────────────────────┬────────────────────────┘
           │                      │
     HTTP API 调用          心跳上报接收
           │                      │
           ▼                      │
┌──────────────────────────────────┤
│         Node Agent (本项目)       │
│  ┌──────────┐  ┌──────────────┐ │
│  │ HTTP API  │  │  Heartbeat   │ │
│  │  Server   │  │  Reporter    │ │
│  └────┬─────┘  └──────────────┘ │
│       │                         │
│  ┌────▼─────────────────────┐   │
│  │    Core Services         │   │
│  │  ┌─────────┐ ┌────────┐  │   │
│  │  │ singbox │ │config  │  │   │
│  │  │ Manager │ │Generator│  │   │
│  │  └─────────┘ └────────┘  │   │
│  │  ┌─────────┐ ┌────────┐  │   │
│  │  │  Stats  │ │ Device │  │   │
│  │  │Collector│ │Limiter │  │   │
│  │  └─────────┘ └────────┘  │   │
│  │  ┌──────────────────────┐│   │
│  │  │ V2Ray Stats Collector││   │
│  │  │ (按用户流量统计)      ││   │
│  │  └──────────────────────┘│   │
│  └──────────────────────────┘   │
│       │                         │
│       ▼                         │
│  ┌──────────────┐               │
│  │   sing-box    │               │
│  │  (子进程)     │               │
│  │ ┌───────────┐│               │
│  │ │Clash API  ││──全局流量/连接 │
│  │ │ :9090     ││               │
│  │ └───────────┘│               │
│  │ ┌───────────┐│               │
│  │ │V2Ray API  ││──按用户流量   │
│  │ │ :10001    ││               │
│  │ └───────────┘│               │
│  └──────────────┘               │
└──────────────────────────────────┘
```

### 2.2 Node Agent 在架构中的角色

| 层级 | 组件 | 职责 |
|------|------|------|
| 业务层 | Laravel Control Plane | 用户注册、套餐购买、节点分配、计费结算 |
| 执行层 | **Node Agent** | 接收指令、管理 sing-box、采集数据、上报状态 |
| 网络层 | sing-box | 实际处理 VPN 流量（代理、路由、加密） |

Node Agent 是业务层与网络层之间的**桥梁**，将高层业务指令翻译为底层 sing-box 配置，并将底层运行状态汇总上报给业务层。

### 2.3 数据流

```
Laravel ──POST /deploy──▶ Node Agent ──生成 config.json──▶ sing-box restart
Laravel ──GET /status──▶ Node Agent ──采集系统信息──▶ 返回 JSON
Laravel ──GET /traffic/user──▶ Node Agent ──V2Ray API gRPC──▶ 按用户流量数据
Node Agent ──POST /heartbeat──▶ Laravel ──存储/展示──▶ 管理后台
sing-box ──Clash API──▶ Node Agent StatsCollector ──累计流量──▶ 上报/查询
sing-box ──V2Ray API──▶ Node Agent V2RayStatsCollector ──按用户流量──▶ 计费/统计
```

---

## 3. 项目结构

```
node-agent/
├── main.go                              # 程序入口：组装模块、启动 HTTP 服务、信号处理
├── go.mod                               # Go 模块定义与依赖管理
├── config/
│   └── config.go                        # 配置加载、保存、默认值、CORS 配置
├── controller/
│   └── handler.go                       # REST API 请求处理器（所有业务逻辑入口）
├── middleware/
│   └── auth.go                          # HTTP 中间件：Token 认证、IP 白名单、CORS、签名验证、日志、Panic 恢复
├── core/
│   ├── singbox/
│   │   ├── manager.go                   # sing-box 进程生命周期管理（状态机、启停、监控、日志捕获、崩溃追踪）
│   │   ├── process_unix.go              # Unix 平台进程信号处理
│   │   └── process_windows.go           # Windows 平台进程信号处理
│   ├── configgen/
│   │   └── generator.go                 # sing-box config.json 动态生成（多协议、多入站、V2Ray API 统计）
│   ├── stats/
│   │   ├── collector.go                 # 流量统计抽象层（Clash API 采集 + Fallback + Multi 降级）
│   │   └── v2ray_stats.go               # 按用户流量统计（V2Ray API gRPC 客户端）
│   ├── device/
│   │   └── limiter.go                   # 设备绑定与并发会话限制
│   └── heartbeat/
│       └── reporter.go                  # 心跳上报至 Laravel 控制面
├── utils/
│   └── system.go                        # 系统信息采集（CPU / 内存 / 磁盘 / 负载 / Uptime）
├── deploy/
│   ├── node-agent.service               # systemd 服务单元文件
│   └── install.sh                       # 一键部署脚本（含源码编译 with_v2ray_api）
├── .github/
│   └── workflows/
│       ├── ci.yml                       # CI 测试工作流
│       └── release.yml                  # 自动编译发布工作流
└── test.php                             # Web 测试页面（API 测试 + Token 生成）
```

**分层设计原则**：

| 层级 | 目录 | 职责 | 依赖方向 |
|------|------|------|----------|
| 入口层 | `main.go` | 组装所有模块，启动服务 | 依赖所有层 |
| 控制层 | `controller/` | 处理 HTTP 请求，编排业务逻辑 | 依赖 core 层 |
| 中间件层 | `middleware/` | 请求拦截、认证、日志 | 依赖 config 层 |
| 核心层 | `core/` | 业务核心逻辑 | 依赖 config、utils 层 |
| 基础层 | `config/`、`utils/` | 配置管理、工具函数 | 无外部依赖 |

---

## 4. 核心模块详解

### 4.1 配置管理模块 (config)

**文件**：[config/config.go](file:///Volumes/koeyx/box/node-agent/config/config.go)

#### 功能

- 从 JSON 文件加载配置
- 配置文件不存在时自动生成默认配置
- 全局配置单例访问
- IP 白名单判断
- 线程安全的配置读写

#### 配置结构体

```go
type Config struct {
    NodeID           string              // 节点唯一标识
    APIPort          int                 // HTTP API 监听端口
    APIToken         string              // API 认证 Token
    LogLevel         string              // 日志级别
    DataDir          string              // 数据存储目录
    SingBox          SingBoxConfig       // sing-box 相关配置
    ControlPlane     ControlPlaneConfig  // 控制面连接配置
    DeviceLimit      DeviceLimitConfig   // 设备限制配置
    IPWhitelist      []string            // IP 白名单
    HeartbeatInterval int                // 心跳上报间隔（秒）
    WatchdogInterval  int                // Watchdog 检测间隔（秒）
}
```

#### 关键方法

| 方法 | 说明 |
|------|------|
| `Load(path)` | 从文件加载配置，不存在则创建默认配置 |
| `Get()` | 获取全局配置单例 |
| `Save(path)` | 保存配置到文件 |
| `GetAPIAddr()` | 返回 API 监听地址（`:port` 格式） |
| `IsIPWhitelisted(ip)` | 判断 IP 是否在白名单中（白名单为空则全部放行） |

#### 设计要点

- 配置文件首次加载时若不存在，自动写入默认配置，方便运维人员修改
- `filepath.Dir()` 计算父目录，避免硬编码路径后缀
- `sync.RWMutex` 保护并发读写

---

### 4.2 sing-box 进程管理模块 (core/singbox)

**文件**：[core/singbox/manager.go](file:///Volumes/koeyx/box/node-agent/core/singbox/manager.go)

#### 状态机

```
                    Start()
  Stopped ──────────────────▶ Starting
     ▲                          │
     │                          │ cmd.Start() 成功
     │                          ▼
     │                      Running
     │                       │  │
     │          Stop()       │  │ 进程异常退出
     │           │           │  ▼
     │           ▼           │  Crashed
     │       Stopping ◀──────┘   │
     │           │               │ Watchdog 自动重启
     └───────────┘               │
             Stopped ◀───────────┘
```

| 状态 | 说明 |
|------|------|
| `StateStopped` | sing-box 未运行 |
| `StateStarting` | 正在启动中 |
| `StateRunning` | 正常运行中 |
| `StateStopping` | 正在停止中 |
| `StateCrashed` | 异常退出，等待 Watchdog 恢复 |

#### 进程管理策略

**启动流程**：

1. 检查当前状态，防止重复启动
2. 校验 sing-box 二进制文件和配置文件是否存在
3. 使用 `exec.CommandContext` 创建子进程
4. 设置 `Setpgid: true`，将 sing-box 放入独立进程组
5. 启动监控 goroutine，等待进程退出

**停止流程**：

1. 设置状态为 `StateStopping`
2. 调用 `context.Cancel()`
3. 向进程组发送 `SIGTERM`（`kill(-pgid, SIGTERM)`）
4. 等待最多 10 秒
5. 超时后发送 `SIGKILL` 强制终止

**为什么使用进程组**：sing-box 可能产生子进程，使用进程组确保 `SIGTERM`/`SIGKILL` 发送给整个进程组，避免僵尸进程。

#### 关键方法

| 方法 | 说明 |
|------|------|
| `Start() error` | 启动 sing-box |
| `Stop() error` | 停止 sing-box（优雅停止 + 强制终止） |
| `Restart() error` | 重启 sing-box（停止 → 500ms 等待 → 启动） |
| `IsRunning() bool` | 检查是否运行中 |
| `GetState() ProcessState` | 获取当前状态 |
| `GetUptime() time.Duration` | 获取运行时长 |
| `GetPID() int` | 获取进程 PID |
| `CrashChannel() <-chan struct{}` | 获取崩溃通知 channel |

#### Watchdog 自动恢复

在 [main.go](file:///Volumes/koeyx/box/node-agent/main.go) 中启动独立 goroutine 运行 Watchdog：

```go
func startWatchdog(mgr *singbox.Manager, cfg *config.Config) {
    ticker := time.NewTicker(interval)
    for range ticker.C {
        if !mgr.IsRunning() && mgr.GetState() == singbox.StateCrashed {
            mgr.Start()  // 自动重启
        }
    }
}
```

Watchdog 以可配置间隔（默认 5 秒）检测 sing-box 状态，发现崩溃后自动拉起。

---

### 4.3 配置动态生成模块 (core/configgen)

**文件**：[core/configgen/generator.go](file:///Volumes/koeyx/box/node-agent/core/configgen/generator.go)

#### 功能

- 根据 DeployRequest 动态生成完整的 sing-box config.json
- 支持 Hysteria2 / VLESS / Reality 三种协议
- 支持 tun / mixed 两种入站模式
- 原子写入配置文件（先写 `.tmp` 再 `rename`）
- 管理多用户部署记录

#### DeployRequest 结构

```go
type DeployRequest struct {
    UserID           string `json:"user_id"`              // 用户 ID（必填）
    NodeID           string `json:"node_id"`              // 节点 ID（必填）
    Protocol         string `json:"protocol"`             // 协议：hysteria2 / vless / reality（必填）
    Server           string `json:"server"`               // 服务器地址（必填）
    Port             int    `json:"port"`                 // 服务器端口（必填）
    Password         string `json:"password"`             // 密码 / Token（必填）
    UUID             string `json:"uuid,omitempty"`       // VLESS/Reality UUID（可选，默认使用 Password）
    SNI              string `json:"sni,omitempty"`        // TLS SNI（可选）
    RealityPubKey    string `json:"reality_public_key,omitempty"`  // Reality 公钥
    RealityShortID   string `json:"reality_short_id,omitempty"`    // Reality Short ID
    InboundType      string `json:"inbound_type,omitempty"`        // 入站类型：tun / mixed
}
```

#### 协议配置映射

**Hysteria2**：

```json
{
  "type": "hysteria2",
  "tag": "proxy",
  "server": "xxx.com",
  "server_port": 443,
  "password": "user_token",
  "tls": {
    "enabled": true,
    "server_name": "sni_value",
    "insecure": false
  }
}
```

**VLESS**：

```json
{
  "type": "vless",
  "tag": "proxy",
  "server": "xxx.com",
  "server_port": 443,
  "uuid": "user_uuid",
  "flow": "xtls-rprx-vision",
  "tls": {
    "enabled": true,
    "server_name": "sni_value",
    "insecure": false
  }
}
```

**Reality**（基于 VLESS + Reality TLS）：

```json
{
  "type": "vless",
  "tag": "proxy",
  "server": "xxx.com",
  "server_port": 443,
  "uuid": "user_uuid",
  "flow": "xtls-rprx-vision",
  "tls": {
    "enabled": true,
    "server_name": "sni_value",
    "reality": {
      "enabled": true,
      "public_key": "reality_pub_key",
      "short_id": "reality_short_id"
    }
  }
}
```

#### 生成的完整配置结构

每次 Deploy 会生成包含以下部分的完整 sing-box 配置：

| 部分 | 内容 |
|------|------|
| `log` | 日志级别 info，启用时间戳 |
| `dns` | Google DNS (tls, 8.8.8.8) + 阿里 DNS (udp, 223.5.5.5)，新格式（type + server） |
| `inbounds` | tun（默认）或 mixed 入站 |
| `outbounds` | 代理出站 + direct |
| `route` | sniff + hijack-dns 规则动作，default_domain_resolver，final 走 proxy |
| `experimental` | Clash API (0.0.0.0:9090) + V2Ray API (127.0.0.1:10001, stats.enabled) |

#### 原子写入机制

```
写入 config.json.tmp → rename config.json.tmp → config.json
```

`os.Rename` 在同一文件系统上是原子操作，确保配置文件不会出现半写状态。

---

### 4.4 流量统计模块 (core/stats)

**文件**：[core/stats/collector.go](file:///Volumes/koeyx/box/node-agent/core/stats/collector.go)

#### 架构设计

```
              ┌──────────────────────┐
              │   Collector 接口      │
              │  GetTraffic()         │
              │  GetConnections()     │
              │  GetStats()           │
              └──────────┬───────────┘
                         │
           ┌─────────────┼─────────────┐
           │             │             │
           ▼             ▼             ▼
  ┌─────────────┐ ┌───────────┐ ┌──────────────┐
  │ SingBox     │ │ Fallback  │ │ Multi        │
  │ Stats       │ │ Collector │ │ Collector    │
  │ Collector   │ │           │ │              │
  └─────────────┘ └───────────┘ └──────────────┘
  通过 Clash API    系统层统计     自动降级


  ┌──────────────────────────────────────────────┐
  │         V2RayStatsCollector                   │
  │  ┌────────────────────────────────────────┐  │
  │  │ gRPC Client → V2Ray API (127.0.0.1:10001) │ │
  │  │ QueryStats() → 按用户/出站流量统计        │  │
  │  └────────────────────────────────────────┘  │
  │  GetUserTraffic(userID) → 单用户流量          │
  │  GetAllUserTraffic()    → 所有用户流量        │
  └──────────────────────────────────────────────┘
```

**双 API 统计架构**：

| API | 地址 | 协议 | 用途 | 粒度 |
|-----|------|------|------|------|
| Clash API | `0.0.0.0:9090` | HTTP RESTful | 全局流量、连接数、实时速率 | 节点级 |
| V2Ray API | `127.0.0.1:10001` | gRPC | 按用户/出站流量统计 | 用户级 |

#### Collector 接口

```go
type Collector interface {
    GetTraffic() (*TrafficData, error)
    GetConnections() (*ConnectionData, error)
    GetStats() (*StatsResult, error)
}
```

#### SingBoxStatsCollector

通过 sing-box 内置的 Clash API 采集流量数据：

| API 端点 | 返回数据 |
|----------|----------|
| `GET /traffic` | `{"up": bytes, "down": bytes}` — 当前速率和累计值 |
| `GET /connections` | `{"total": count}` — 活跃连接数 |

**流量累计逻辑**：

Clash API 的 `/traffic` 返回的是 sing-box 启动后的累计值。当 sing-box 重启后，计数器会重置为 0。为避免负数，采用 delta 差值计算：

```go
deltaUp := traffic.Up - s.lastTraffic.Upload
if deltaUp > 0 {
    s.totalTraffic.Upload += deltaUp
    s.lastTraffic.Upload = traffic.Up
}
```

当 `deltaUp <= 0` 时（说明 sing-box 重启了），跳过本次更新，避免累计值出现负数。

#### FallbackCollector

当 Clash API 不可用时的备用统计器，提供手动写入接口：

| 方法 | 说明 |
|------|------|
| `AddUpload(bytes)` | 增加上传流量 |
| `AddDownload(bytes)` | 增加下载流量 |
| `SetConnections(count)` | 设置连接数 |

可用于对接 iptables / tc / conntrack 等系统层统计工具。

#### MultiCollector

自动降级策略：优先使用 primary（SingBoxStatsCollector），失败时自动切换到 fallback（FallbackCollector）。

#### V2RayStatsCollector

**文件**：[core/stats/v2ray_stats.go](file:///Volumes/koeyx/box/node-agent/core/stats/v2ray_stats.go)

通过 V2Ray API 的 gRPC 接口采集按用户粒度的流量统计数据。**前提条件**：sing-box 必须使用 `-tags with_v2ray_api` 编译。

**工作原理**：

1. 通过 gRPC 连接 sing-box 的 V2Ray API（`127.0.0.1:10001`）
2. 调用 `QueryStats` RPC 获取所有统计项
3. 解析统计项名称格式 `user>>>{user_id}>>>traffic>>>{uplink|downlink}`
4. 汇总为按用户的上传/下载流量数据

**关键方法**：

| 方法 | 说明 |
|------|------|
| `IsEnabled() bool` | 检查 V2Ray API 是否可用 |
| `GetUserTraffic(userID) (*UserTraffic, error)` | 查询单个用户流量 |
| `GetAllUserTraffic() ([]*UserTraffic, error)` | 查询所有用户流量 |
| `Close()` | 关闭 gRPC 连接 |

**自动重连机制**：

- 首次启动时尝试连接 V2Ray API
- 连接失败时 `enabled = false`，不阻塞服务启动
- 每次查询时若 `enabled = false`，自动尝试重连
- 查询失败时标记 `enabled = false`，下次查询时重试

**V2Ray API 配置**（在 sing-box config.json 中）：

```json
{
  "experimental": {
    "v2ray_api": {
      "listen": "127.0.0.1:10001",
      "stats": {
        "enabled": true,
        "outbounds": ["proxy", "direct"]
      }
    }
  }
}
```

**统计项名称解析**：

| 名称格式 | 解析结果 |
|----------|----------|
| `user>>>user-001>>>traffic>>>uplink` | 用户 `user-001` 的上传流量 |
| `user>>>user-001>>>traffic>>>downlink` | 用户 `user-001` 的下载流量 |
| `outbound>>>proxy>>>traffic>>>uplink` | 出站 `proxy` 的上传流量 |

---

### 4.5 设备限制模块 (core/device)

**文件**：[core/device/limiter.go](file:///Volumes/koeyx/box/node-agent/core/device/limiter.go)

#### 数据结构

```
devices: map[userID]map[deviceID]*DeviceInfo
sessions: map[userID]int (当前并发会话数)
```

#### 设备注册流程

```
RegisterDevice(userID, deviceID, ip)
  │
  ├── 设备已存在？── 是 ──▶ 更新 LastSeen，返回 Allowed
  │
  └── 设备不存在
       │
       ├── 设备数 >= MaxDevices？── 是 ──▶ 返回 DeviceLimitExceeded
       │
       └── 设备数 < MaxDevices ──▶ 注册新设备，返回 Allowed
```

#### 会话控制流程

```
AcquireSession(userID)
  │
  ├── 当前会话数 >= MaxConcurrent？── 是 ──▶ 返回 ConcurrentLimitExceeded
  │
  └── 当前会话数 < MaxConcurrent ──▶ 会话数 +1，返回 Allowed

ReleaseSession(userID)
  │
  └── 会话数 -1，若为 0 则删除记录
```

#### 注册结果枚举

| 值 | 说明 |
|----|------|
| `RegisterAllowed` | 允许注册 |
| `RegisterDeviceLimitExceeded` | 设备数超限 |
| `RegisterConcurrentLimitExceeded` | 并发会话数超限 |

#### 过期清理

`CleanupStale(timeout)` 方法清理超过指定时间未活跃的设备记录。在 main.go 中每 5 分钟执行一次，清理 30 分钟未活跃的设备。

#### 配置热更新

`UpdateConfig(cfg)` 方法支持运行时更新设备限制配置，无需重启 Node Agent。

---

### 4.6 心跳上报模块 (core/heartbeat)

**文件**：[core/heartbeat/reporter.go](file:///Volumes/koeyx/box/node-agent/core/heartbeat/reporter.go)

#### 上报内容

每次心跳上报包含以下四类数据：

**节点状态**：

| 字段 | 类型 | 说明 |
|------|------|------|
| `singbox_running` | bool | sing-box 是否运行中 |
| `singbox_state` | string | 进程状态（running/stopped/crashed） |
| `uptime_seconds` | int64 | 运行时长（秒） |
| `pid` | int | sing-box 进程 PID |

**流量数据**：

| 字段 | 类型 | 说明 |
|------|------|------|
| `upload` | int64 | 累计上传字节数 |
| `download` | int64 | 累计下载字节数 |

**在线信息**：

| 字段 | 类型 | 说明 |
|------|------|------|
| `user_count` | int | 在线用户数 |
| `device_count` | int | 总设备数 |
| `active_sessions` | int | 活跃会话数 |

**系统信息**：

| 字段 | 类型 | 说明 |
|------|------|------|
| `cpu_percent` | float64 | CPU 使用率 |
| `mem_percent` | float64 | 内存使用率 |
| `mem_used_mb` | uint64 | 已用内存（MB） |
| `mem_total_mb` | uint64 | 总内存（MB） |
| `disk_used_gb` | uint64 | 已用磁盘（GB） |
| `disk_total_gb` | uint64 | 总磁盘（GB） |
| `load_1/5/15` | float64 | 系统负载 |

#### 上报机制

- 以可配置间隔（默认 10 秒）向 Laravel 控制面发送 HTTP POST 请求
- 请求路径：`{ControlPlane.URL}/api/node/heartbeat`
- 认证方式：`X-Node-Token` + `X-Node-ID` Header
- 启动时立即发送一次心跳，之后定时发送
- 支持优雅停止（通过 `stopCh` channel）

---

### 4.7 系统信息采集模块 (utils)

**文件**：[utils/system.go](file:///Volumes/koeyx/box/node-agent/utils/system.go)

基于 [gopsutil](https://github.com/shirou/gopsutil) 库实现跨平台系统信息采集：

| 函数 | 返回值 | 说明 |
|------|--------|------|
| `GetCPUUsage()` | `(float64, error)` | CPU 使用率百分比 |
| `GetMemoryUsage()` | `(percent, usedMB, totalMB)` | 内存使用率、已用、总量 |
| `GetDiskUsage()` | `(usedGB, totalGB)` | 磁盘使用量、总量 |
| `GetLoadAvg()` | `(load1, load5, load15)` | 系统负载（仅 Linux） |
| `GetUptime()` | `uint64` | 系统运行时长（秒） |

**注意**：`GetLoadAvg()` 在非 Linux 系统上返回 `(0, 0, 0)`。`GetCPUUsage()` 采样间隔为 1 秒。

---

## 5. API 接口文档

### 5.1 认证方式

所有 API 请求（除 `/health`）均需携带认证 Token：

**方式一：HTTP Header（推荐）**

```
X-Node-Token: your-secure-token
```

**方式二：Query Parameter**

```
?token=your-secure-token
```

未认证请求返回：

```json
{
  "error": "unauthorized: invalid token"
}
```

HTTP 状态码：`401 Unauthorized`

---

### 5.2 节点部署接口

#### `POST /deploy`

下发用户节点配置，生成 sing-box config.json 并重启 sing-box。

**请求体**：

```json
{
  "user_id": "123",
  "node_id": "n1",
  "protocol": "hysteria2",
  "server": "hk1.xxx.com",
  "port": 443,
  "password": "token_xxx",
  "sni": "hk1.xxx.com",
  "inbound_type": "tun"
}
```

**字段说明**：

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `user_id` | string | 是 | 用户 ID |
| `node_id` | string | 是 | 节点 ID |
| `protocol` | string | 是 | 协议类型：`hysteria2` / `vless` / `reality` |
| `server` | string | 是 | 服务器地址 |
| `port` | int | 是 | 服务器端口 |
| `password` | string | 是 | 认证密码/Token |
| `uuid` | string | 否 | VLESS/Reality 的 UUID（默认使用 password） |
| `sni` | string | 否 | TLS SNI |
| `reality_public_key` | string | 否 | Reality 公钥（protocol=reality 时使用） |
| `reality_short_id` | string | 否 | Reality Short ID |
| `inbound_type` | string | 否 | 入站类型：`tun`（默认）/ `mixed` |

**成功响应** (`200 OK`)：

```json
{
  "success": true,
  "message": "deployed hysteria2 config for user 123",
  "node_id": "n1"
}
```

**错误响应**：

| 状态码 | 场景 |
|--------|------|
| 400 | 缺少必填字段 |
| 500 | 配置生成失败 / sing-box 启动失败 |

---

### 5.3 节点状态接口

#### `GET /status`

返回节点完整状态信息。

**响应** (`200 OK`)：

```json
{
  "node": {
    "singbox_running": true,
    "singbox_state": "running",
    "uptime_seconds": 3600,
    "pid": 12345
  },
  "system": {
    "cpu_percent": 15.3,
    "mem_percent": 45.2,
    "mem_used_mb": 1808,
    "mem_total_mb": 4096,
    "disk_used_gb": 25,
    "disk_total_gb": 100,
    "load_1": 0.5,
    "load_5": 0.4,
    "load_15": 0.3
  },
  "connections": {
    "active": 42
  },
  "devices": {
    "online_users": 15,
    "total_devices": 38
  }
}
```

---

### 5.4 流量统计接口

#### `GET /stats`

返回 sing-box 流量统计数据。

**响应** (`200 OK`)：

```json
{
  "traffic": {
    "upload": 1073741824,
    "download": 2147483648
  },
  "connections": {
    "active": 42
  }
}
```

**字段说明**：

| 字段 | 类型 | 说明 |
|------|------|------|
| `traffic.upload` | int64 | 累计上传字节数 |
| `traffic.download` | int64 | 累计下载字节数 |
| `connections.active` | int | 当前活跃连接数 |

---

### 5.5 按用户流量统计接口

#### `GET /traffic/user`

通过 V2Ray API 查询按用户粒度的流量统计数据。

**前提条件**：sing-box 必须使用 `-tags with_v2ray_api` 编译，且配置中启用了 `v2ray_api.stats`。

**查询参数**：

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `user_id` | string | 否 | 指定用户 ID，不传则返回所有用户 |

**查询单个用户** (`GET /traffic/user?user_id=user-001`)：

**响应** (`200 OK`)：

```json
{
  "user_id": "user-001",
  "upload": 5242880,
  "download": 31457280
}
```

**查询所有用户** (`GET /traffic/user`)：

**响应** (`200 OK`)：

```json
{
  "users": [
    {
      "user_id": "user-001",
      "upload": 5242880,
      "download": 31457280
    },
    {
      "user_id": "user-002",
      "upload": 10485760,
      "download": 52428800
    },
    {
      "user_id": "outbound:proxy",
      "upload": 15728640,
      "download": 83886080
    }
  ],
  "count": 3
}
```

**错误响应**：

| 状态码 | 场景 |
|--------|------|
| 503 | V2Ray API 不可用（sing-box 未使用 with_v2ray_api 编译） |
| 500 | gRPC 查询失败 |

**与 /stats 接口的区别**：

| 接口 | 粒度 | 数据来源 | 用途 |
|------|------|----------|------|
| `GET /stats` | 节点级（全局总量） | Clash API | 监控节点整体负载 |
| `GET /traffic/user` | 用户级（按用户统计） | V2Ray API | 计费、用户流量配额管理 |

---

### 5.6 心跳数据接口

#### `GET /heartbeat`

返回与上报给控制面相同的心跳数据（可用于本地调试）。

**响应** (`200 OK`)：

```json
{
  "node_id": "node-001",
  "timestamp": 1714200000,
  "status": {
    "singbox_running": true,
    "singbox_state": "running",
    "uptime_seconds": 3600,
    "pid": 12345
  },
  "traffic": {
    "upload": 1073741824,
    "download": 2147483648
  },
  "online": {
    "user_count": 15,
    "device_count": 38,
    "active_sessions": 15
  },
  "system": {
    "cpu_percent": 15.3,
    "mem_percent": 45.2,
    "mem_used_mb": 1808,
    "mem_total_mb": 4096,
    "disk_used_gb": 25,
    "disk_total_gb": 100,
    "load_1": 0.5,
    "load_5": 0.4,
    "load_15": 0.3
  }
}
```

---

### 5.7 进程控制接口

#### `POST /start`

启动 sing-box 进程。

**响应** (`200 OK`)：

```json
{
  "success": true,
  "message": "sing-box started"
}
```

#### `POST /stop`

停止 sing-box 进程。

**响应** (`200 OK`)：

```json
{
  "success": true,
  "message": "sing-box stopped"
}
```

#### `POST /restart`

重启 sing-box 进程。

**响应** (`200 OK`)：

```json
{
  "success": true,
  "message": "sing-box restarted"
}
```

---

### 5.8 设备管理接口

#### `POST /device/register`

注册设备并检查设备数限制。

**请求体**：

```json
{
  "user_id": "123",
  "device_id": "dev-abc-001",
  "ip": "1.2.3.4"
}
```

**成功响应** (`200 OK`)：

```json
{
  "allowed": true,
  "reason": "allowed",
  "device_count": 2
}
```

**设备超限响应** (`403 Forbidden`)：

```json
{
  "allowed": false,
  "reason": "device_limit_exceeded",
  "error": "user 123 has 3 devices, limit is 3"
}
```

#### `POST /session/acquire`

获取会话并检查并发限制。

**请求体**：

```json
{
  "user_id": "123"
}
```

**成功响应** (`200 OK`)：

```json
{
  "allowed": true,
  "reason": "allowed",
  "concurrent_count": 3
}
```

**并发超限响应** (`403 Forbidden`)：

```json
{
  "allowed": false,
  "reason": "concurrent_limit_exceeded",
  "error": "user 123 has 5 concurrent sessions, limit is 5"
}
```

#### `POST /session/release`

释放会话。

**请求体**：

```json
{
  "user_id": "123"
}
```

**响应** (`200 OK`)：

```json
{
  "success": true
}
```

---

### 5.9 日志查询接口

#### `GET /logs`

查询 sing-box 进程的最近日志输出，包含崩溃原因和错误信息。

**查询参数**：

| 参数 | 类型 | 必填 | 默认值 | 说明 |
|------|------|------|--------|------|
| `lines` | int | 否 | 50 | 返回最近 N 行日志（1-200） |

**响应** (`200 OK`)：

```json
{
  "lines": [
    "[singbox:err] \u001b[36mINFO\u001b[0m[0000] network: updated default interface eth0, index 2",
    "[singbox:err] \u001b[31mFATAL\u001b[0m[0000] start service: create v2ray-server: v2ray api is not included in this build"
  ],
  "count": 2
}
```

**日志来源**：sing-box 的 stderr 输出，由 Manager 通过 RingBuffer 实时捕获。

---

### 5.10 健康检查接口

#### `GET /health`

无需认证，返回纯文本 `ok`，用于负载均衡器健康检查。

**响应** (`200 OK`)：

```
ok
```

---

## 6. 安全机制

### 6.1 Token 认证

所有 API 请求（`/health` 除外）必须携带 `X-Node-Token` Header 或 `token` Query 参数。Token 在配置文件中设置：

```json
{
  "api_token": "your-secure-random-token"
}
```

**生产环境建议**：

- Token 长度不少于 32 字符
- 使用加密安全的随机生成器
- 定期轮换 Token

### 6.2 IP 白名单

可选功能，限制只有指定 IP 可以访问 API：

```json
{
  "ip_whitelist": ["10.0.0.1", "192.168.1.0/24"]
}
```

- 白名单为空数组时，所有 IP 均可访问
- 支持 `0.0.0.0/0` 表示允许所有
- 优先读取 `X-Forwarded-For` 和 `X-Real-IP` Header

### 6.3 Deploy 签名验证

为防止 Deploy 请求被伪造，提供 HMAC-SHA256 签名验证中间件。

**签名算法**：

```
message = timestamp + "." + request_body
signature = HMAC-SHA256(signing_key, message)
```

**请求 Header**：

| Header | 说明 |
|--------|------|
| `X-Signature` | HMAC-SHA256 签名（十六进制） |
| `X-Timestamp` | 请求时间戳（RFC3339 格式） |

**时效验证**：签名超过 5 分钟自动失效。

**使用方式**（在 main.go 中添加）：

```go
// 在 deploy 路由上单独应用签名验证
deployHandler := middleware.DeploySignatureVerify("your-signing-key")(
    http.HandlerFunc(handler.Deploy),
)
mux.Handle("/deploy", withMethods(deployHandler.ServeHTTP, http.MethodPost))
```

### 6.4 中间件执行链

请求经过以下中间件链（按执行顺序）：

```
Request → Recovery → Logging → CORS → IPWhitelist → TokenAuth → Handler
```

| 顺序 | 中间件 | 功能 |
|------|--------|------|
| 1 | Recovery | 捕获 panic，返回 500 |
| 2 | Logging | 记录请求方法和耗时 |
| 3 | CORS | 处理跨域请求，添加 CORS 响应头 |
| 4 | IPWhitelist | IP 白名单过滤 |
| 5 | TokenAuth | Token 认证（`/health` 除外） |

### 6.5 CORS 跨域支持

Node Agent 内置 CORS 中间件，允许从 Web 页面（如 test.php 测试页）跨域访问 API。

**配置方式**：

```json
{
  "cors": {
    "enabled": true,
    "allowed_origins": ["https://admin.example.com"],
    "allowed_methods": ["GET", "POST"],
    "allowed_headers": ["Content-Type", "X-Node-Token"]
  }
}
```

**安全建议**：

- 生产环境应将 `allowed_origins` 设为具体域名，避免使用 `*`
- 仅开放必要的 HTTP 方法
- `/health` 端点始终允许跨域访问

---

## 7. 配置参考

### 7.1 完整配置文件

配置文件路径：`/etc/node-agent/config.json`

```json
{
  "node_id": "node-001",
  "api_port": 8080,
  "api_token": "CHANGE-ME-TO-A-SECURE-TOKEN",
  "log_level": "info",
  "data_dir": "/var/lib/node-agent",
  "cors": {
    "enabled": true,
    "allowed_origins": ["*"],
    "allowed_methods": ["GET", "POST", "PUT", "DELETE", "OPTIONS"],
    "allowed_headers": ["Content-Type", "X-Node-Token", "Authorization"]
  },
  "singbox": {
    "binary_path": "/usr/local/bin/sing-box",
    "config_path": "/etc/sing-box/config.json",
    "work_dir": "/etc/sing-box"
  },
  "control_plane": {
    "url": "http://YOUR-LARAVEL-SERVER:8000",
    "token": "YOUR-CONTROL-PLANE-TOKEN",
    "node_id": "node-001",
    "timeout": 10
  },
  "device_limit": {
    "max_devices": 3,
    "max_concurrent": 5
  },
  "ip_whitelist": [],
  "heartbeat_interval": 10,
  "watchdog_interval": 5
}
```

### 7.2 配置项说明

#### 顶层配置

| 字段 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `node_id` | string | `node-001` | 节点唯一标识，需与控制面一致 |
| `api_port` | int | `8080` | Node Agent HTTP API 监听端口 |
| `api_token` | string | `change-me-in-production` | API 认证 Token，**必须修改** |
| `log_level` | string | `info` | 日志级别 |
| `data_dir` | string | `/var/lib/node-agent` | 数据存储目录 |
| `heartbeat_interval` | int | `10` | 心跳上报间隔（秒） |
| `watchdog_interval` | int | `5` | Watchdog 检测间隔（秒） |

#### singbox 配置

| 字段 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `binary_path` | string | `/usr/local/bin/sing-box` | sing-box 二进制文件路径 |
| `config_path` | string | `/etc/sing-box/config.json` | sing-box 配置文件路径 |
| `work_dir` | string | `/etc/sing-box` | sing-box 工作目录 |

#### control_plane 配置

| 字段 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `url` | string | `http://127.0.0.1:8000` | Laravel 控制面地址 |
| `token` | string | 空 | 控制面认证 Token |
| `node_id` | string | `node-001` | 在控制面注册的节点 ID |
| `timeout` | int | `10` | HTTP 请求超时（秒） |

#### device_limit 配置

| 字段 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `max_devices` | int | `3` | 每用户最大设备数（0 = 不限制） |
| `max_concurrent` | int | `5` | 每用户最大并发会话数（0 = 不限制） |

#### ip_whitelist 配置

| 字段 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `ip_whitelist` | []string | `[]` | IP 白名单，为空则允许所有 |

#### cors 配置

| 字段 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `cors.enabled` | bool | `true` | 是否启用 CORS |
| `cors.allowed_origins` | []string | `["*"]` | 允许的来源域名 |
| `cors.allowed_methods` | []string | `["GET","POST","PUT","DELETE","OPTIONS"]` | 允许的 HTTP 方法 |
| `cors.allowed_headers` | []string | `["Content-Type","X-Node-Token","Authorization"]` | 允许的请求头 |

---

## 8. 部署指南

### 8.1 编译

**在开发机或 CI 环境编译**：

```bash
cd node-agent

# 下载依赖
go mod tidy

# 编译 Linux amd64 二进制
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o node-agent .

# 编译 Linux arm64 二进制（ARM 服务器）
CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -ldflags="-s -w" -o node-agent-arm64 .
```

编译参数说明：

| 参数 | 说明 |
|------|------|
| `CGO_ENABLED=0` | 禁用 CGO，生成纯静态二进制 |
| `-s -w` | 去除调试信息，减小二进制体积 |
| `GOOS=linux` | 目标操作系统 |
| `GOARCH=amd64` | 目标架构 |

### 8.2 一键安装

将编译好的二进制和 deploy 目录上传到 VPS 后执行：

```bash
bash deploy/install.sh
```

安装脚本执行以下步骤：

1. 检测系统架构（amd64/arm64/armv7）
2. 安装 Go 编译环境（如不存在）
3. 从源码编译 sing-box（含 `-tags with_v2ray_api`，支持按用户流量统计）
4. 若源码编译失败，回退到下载预编译二进制（不含按用户流量统计）
5. 编译 Node Agent 二进制文件
6. 安装到 `/usr/local/bin/`
7. 创建所需目录
8. 生成默认配置文件（如不存在）
9. 创建 sing-box 占位配置文件
10. 安装 systemd 服务
11. 配置防火墙规则
12. 启用开机自启

**验证安装**：

```bash
# 检查 sing-box 是否包含 v2ray_api
sing-box version
# 输出应包含 "with_v2ray_api"

# 检查 Node Agent 状态
systemctl status node-agent
```

### 8.3 手动安装

```bash
# 1. 安装 Node Agent 二进制
cp node-agent /usr/local/bin/node-agent
chmod +x /usr/local/bin/node-agent

# 2. 创建目录
mkdir -p /etc/node-agent
mkdir -p /etc/sing-box
mkdir -p /var/lib/node-agent
mkdir -p /var/log/node-agent

# 3. 安装 sing-box（从源码编译，含 with_v2ray_api）
# 3a. 安装 Go
wget https://go.dev/dl/go1.23.4.linux-amd64.tar.gz
tar -C /usr/local -xzf go1.23.4.linux-amd64.tar.gz
export PATH=$PATH:/usr/local/go/bin

# 3b. 编译 sing-box
go install -tags "with_v2ray_api" github.com/sagernet/sing-box/cmd/sing-box@latest
cp $(go env GOPATH)/bin/sing-box /usr/local/bin/
chmod +x /usr/local/bin/sing-box

# 3c. 验证
sing-box version  # 应显示 with_v2ray_api

# 4. 创建配置文件
cat > /etc/node-agent/config.json << 'EOF'
{
  "node_id": "node-hk-001",
  "api_port": 8080,
  "api_token": "your-secure-random-token-here",
  "log_level": "info",
  "data_dir": "/var/lib/node-agent",
  "cors": {
    "enabled": true,
    "allowed_origins": ["*"],
    "allowed_methods": ["GET", "POST", "PUT", "DELETE", "OPTIONS"],
    "allowed_headers": ["Content-Type", "X-Node-Token", "Authorization"]
  },
  "singbox": {
    "binary_path": "/usr/local/bin/sing-box",
    "config_path": "/etc/sing-box/config.json",
    "work_dir": "/etc/sing-box"
  },
  "control_plane": {
    "url": "https://your-laravel-server.com",
    "token": "your-control-plane-token",
    "node_id": "node-hk-001",
    "timeout": 10
  },
  "device_limit": {
    "max_devices": 3,
    "max_concurrent": 5
  },
  "ip_whitelist": [],
  "heartbeat_interval": 10,
  "watchdog_interval": 5
}
EOF

# 5. 创建 sing-box 占位配置
cat > /etc/sing-box/config.json << 'EOF'
{
  "log": {"level": "info"},
  "dns": {
    "servers": [
      {"tag": "google", "type": "tls", "server": "8.8.8.8"},
      {"tag": "local", "type": "udp", "server": "223.5.5.5"}
    ]
  },
  "inbounds": [],
  "outbounds": [{"type": "direct", "tag": "direct"}],
  "route": {
    "rules": [{"action": "sniff"}, {"protocol": "dns", "action": "hijack-dns"}],
    "default_domain_resolver": "google",
    "final": "direct"
  }
}
EOF

# 6. 安装 systemd 服务
cp deploy/node-agent.service /etc/systemd/system/
systemctl daemon-reload
systemctl enable node-agent
```

### 8.4 systemd 管理

```bash
# 启动服务
systemctl start node-agent

# 停止服务
systemctl stop node-agent

# 重启服务
systemctl restart node-agent

# 查看状态
systemctl status node-agent

# 查看日志
journalctl -u node-agent -f

# 查看最近 100 行日志
journalctl -u node-agent -n 100
```

---

## 9. 运维手册

### 9.1 日常运维命令

```bash
# 检查节点状态
curl -s http://localhost:8080/status -H "X-Node-Token: your-token" | jq .

# 查看流量统计（全局）
curl -s http://localhost:8080/stats -H "X-Node-Token: your-token" | jq .

# 查看按用户流量统计（所有用户）
curl -s http://localhost:8080/traffic/user -H "X-Node-Token: your-token" | jq .

# 查看按用户流量统计（指定用户）
curl -s "http://localhost:8080/traffic/user?user_id=user-001" -H "X-Node-Token: your-token" | jq .

# 查看 sing-box 日志
curl -s "http://localhost:8080/logs?lines=20" -H "X-Node-Token: your-token" | jq .

# 重启 sing-box
curl -X POST http://localhost:8080/restart -H "X-Node-Token: your-token"

# 部署新配置
curl -X POST http://localhost:8080/deploy \
  -H "Content-Type: application/json" \
  -H "X-Node-Token: your-token" \
  -d '{
    "user_id": "123",
    "node_id": "node-hk-001",
    "protocol": "hysteria2",
    "server": "hk1.xxx.com",
    "port": 443,
    "password": "token_xxx"
  }'
```

### 9.2 日志查看

Node Agent 日志通过 systemd journal 输出：

```bash
# 实时跟踪日志
journalctl -u node-agent -f

# 查看错误日志
journalctl -u node-agent -p err

# 查看最近 1 小时日志
journalctl -u node-agent --since "1 hour ago"

# 搜索特定关键词
journalctl -u node-agent | grep "watchdog"
journalctl -u node-agent | grep "crashed"
```

日志前缀说明：

| 前缀 | 来源 | 说明 |
|------|------|------|
| `[main]` | main.go | 程序启动/停止 |
| `[http]` | middleware | HTTP 请求日志 |
| `[watchdog]` | Watchdog | sing-box 崩溃恢复 |
| `[heartbeat]` | heartbeat | 心跳上报状态 |
| `[singbox]` | singbox manager | 进程管理 |
| `[panic]` | middleware | panic 恢复 |

### 9.3 故障排查

#### sing-box 无法启动

```bash
# 1. 检查 sing-box 二进制是否存在
ls -la /usr/local/bin/sing-box

# 2. 手动测试 sing-box 配置
sing-box check -c /etc/sing-box/config.json

# 3. 查看 sing-box 错误输出
journalctl -u node-agent | grep "sing-box"

# 4. 检查配置文件内容
cat /etc/sing-box/config.json | jq .
```

#### 心跳上报失败

```bash
# 1. 检查控制面连通性
curl -I https://your-laravel-server.com

# 2. 检查心跳日志
journalctl -u node-agent | grep "heartbeat"

# 3. 手动测试心跳
curl -X POST https://your-laravel-server.com/api/node/heartbeat \
  -H "Content-Type: application/json" \
  -H "X-Node-Token: your-token" \
  -H "X-Node-ID: node-hk-001" \
  -d '{"node_id":"node-hk-001","timestamp":0}'
```

#### 流量统计为 0

```bash
# 1. 检查 Clash API 是否可访问（全局流量）
curl http://localhost:9090/traffic -H "Authorization: Bearer node-agent-stats"

# 2. 检查 sing-box 配置中是否启用了 Clash API
cat /etc/sing-box/config.json | jq '.experimental'

# 3. 检查连接数
curl http://localhost:9090/connections -H "Authorization: Bearer node-agent-stats"
```

#### 按用户流量统计不可用

```bash
# 1. 检查 sing-box 是否包含 with_v2ray_api
sing-box version
# 输出应包含 "with_v2ray_api"

# 2. 检查 V2Ray API 配置
cat /etc/sing-box/config.json | jq '.experimental.v2ray_api'

# 3. 测试 /traffic/user 接口
curl -s http://localhost:8080/traffic/user -H "X-Node-Token: your-token"
# 若返回 503，说明 sing-box 未使用 with_v2ray_api 编译

# 4. 重新编译 sing-box
go install -tags "with_v2ray_api" github.com/sagernet/sing-box/cmd/sing-box@latest
sudo cp $(go env GOPATH)/bin/sing-box /usr/local/bin/sing-box
sudo systemctl restart node-agent
```

#### sing-box 启动报废弃错误

```bash
# 查看 sing-box 日志
curl -s "http://localhost:8080/logs?lines=10" -H "X-Node-Token: your-token" | jq .

# 常见废弃错误及解决方案：
# - "legacy DNS servers is deprecated" → 更新 DNS 格式为 type + server
# - "missing route.default_domain_resolver" → 添加 default_domain_resolver 字段
# - "dns outbound is deprecated" → 使用 route action: hijack-dns
# - "clash_api.listen: unknown field" → 改用 external_controller
```

#### API 返回 401

```bash
# 检查配置文件中的 Token
cat /etc/node-agent/config.json | jq '.api_token'

# 使用正确 Token 测试
curl http://localhost:8080/status -H "X-Node-Token: correct-token"
```

### 9.4 sing-box 配置模板

> **重要**：sing-box 1.12+ 使用新 DNS 格式和路由规则动作，以下模板已更新为兼容格式。

**Hysteria2**：

```bash
curl -X POST http://localhost:8080/deploy \
  -H "Content-Type: application/json" \
  -H "X-Node-Token: your-token" \
  -d '{
    "user_id": "user-001",
    "node_id": "node-hk-001",
    "protocol": "hysteria2",
    "server": "hk1.example.com",
    "port": 443,
    "password": "hy2-password-here",
    "sni": "hk1.example.com"
  }'
```

**VLESS**：

```bash
curl -X POST http://localhost:8080/deploy \
  -H "Content-Type: application/json" \
  -H "X-Node-Token: your-token" \
  -d '{
    "user_id": "user-002",
    "node_id": "node-jp-001",
    "protocol": "vless",
    "server": "jp1.example.com",
    "port": 443,
    "password": "vless-password",
    "uuid": "a3482e88-686a-4a58-8126-99c9df64b7bf",
    "sni": "jp1.example.com"
  }'
```

**Reality**：

```bash
curl -X POST http://localhost:8080/deploy \
  -H "Content-Type: application/json" \
  -H "X-Node-Token: your-token" \
  -d '{
    "user_id": "user-003",
    "node_id": "node-us-001",
    "protocol": "reality",
    "server": "us1.example.com",
    "port": 443,
    "password": "reality-password",
    "uuid": "a3482e88-686a-4a58-8126-99c9df64b7bf",
    "sni": "www.microsoft.com",
    "reality_public_key": "xWjB2CjO7ZGm3YqCjL7mGqL9mWqB2CjO7ZGm3YqCjL4",
    "reality_short_id": "6ba85179930d344f"
  }'
```

**生成的 sing-box 配置示例**（Hysteria2）：

```json
{
  "log": {
    "level": "info",
    "timestamp": true
  },
  "dns": {
    "servers": [
      {
        "tag": "google",
        "type": "tls",
        "server": "8.8.8.8"
      },
      {
        "tag": "local",
        "type": "udp",
        "server": "223.5.5.5"
      }
    ]
  },
  "inbounds": [
    {
      "type": "tun",
      "tag": "tun-in",
      "address": ["10.0.0.1/24"]
    }
  ],
  "outbounds": [
    {
      "type": "hysteria2",
      "tag": "proxy",
      "server": "hk1.example.com",
      "server_port": 443,
      "password": "hy2-password-here",
      "tls": {
        "enabled": true,
        "server_name": "hk1.example.com"
      }
    },
    {
      "type": "direct",
      "tag": "direct"
    }
  ],
  "route": {
    "rules": [
      { "action": "sniff" },
      { "protocol": "dns", "action": "hijack-dns" }
    ],
    "default_domain_resolver": "google",
    "final": "proxy"
  },
  "experimental": {
    "clash_api": {
      "external_controller": "0.0.0.0:9090",
      "secret": "node-agent-stats"
    },
    "v2ray_api": {
      "listen": "127.0.0.1:10001",
      "stats": {
        "enabled": true,
        "outbounds": ["proxy", "direct"]
      }
    }
  }
}
```

**sing-box 1.12+ 迁移要点**：

| 旧格式（已废弃） | 新格式 | 说明 |
|------------------|--------|------|
| `dns.servers[].address` | `dns.servers[].type` + `server` | DNS 服务器格式变更 |
| `outbounds[].type: "dns"` | `route.rules[].action: "hijack-dns"` | DNS 出站已移除，改用路由动作 |
| `outbounds[].type: "block"` | `route.rules[].action: "reject"` | 阻断出站已移除，改用路由动作 |
| `clash_api.listen` | `clash_api.external_controller` | 字段重命名 |
| `tun.inet4_address` | `tun.address` (数组) | TUN 地址格式变更 |
| 缺少 `route.default_domain_resolver` | 必须指定 | 域名解析器配置变为必填 |

---

## 10. 扩展指南

### 10.1 新增协议支持

在 [core/configgen/generator.go](file:///Volumes/koeyx/box/node-agent/core/configgen/generator.go) 的 `generateOutbound` 方法中添加新协议分支：

```go
func (g *Generator) generateOutbound(req *DeployRequest) (*Outbound, error) {
    switch req.Protocol {
    case "hysteria2":
        // ... 已有实现
    case "vless":
        // ... 已有实现
    case "reality":
        // ... 已有实现
    case "shadowsocks":
        return &Outbound{
            Type:       "shadowsocks",
            Tag:        "proxy",
            Server:     req.Server,
            ServerPort: req.Port,
            Method:     req.Method,
            Password:   req.Password,
        }, nil
    default:
        return nil, fmt.Errorf("unsupported protocol: %s", req.Protocol)
    }
}
```

同时在 `DeployRequest` 结构体中添加协议特有字段。

### 10.2 自定义统计后端

实现 `stats.Collector` 接口即可替换或扩展统计后端：

```go
type Collector interface {
    GetTraffic() (*TrafficData, error)
    GetConnections() (*ConnectionData, error)
    GetStats() (*StatsResult, error)
}
```

例如，基于 iptables 的统计器：

```go
type IptablesCollector struct {
    // ...
}

func (c *IptablesCollector) GetTraffic() (*TrafficData, error) {
    // 解析 iptables -L -v -x 输出
    // 返回 upload/download 字节数
}

func (c *IptablesCollector) GetConnections() (*ConnectionData, error) {
    // 解析 conntrack -C 输出
    // 返回活跃连接数
}

func (c *IptablesCollector) GetStats() (*StatsResult, error) {
    traffic, _ := c.GetTraffic()
    connections, _ := c.GetConnections()
    return &StatsResult{Traffic: *traffic, Connections: *connections}, nil
}
```

在 main.go 中替换：

```go
iptablesCollector := NewIptablesCollector()
multiCollector := stats.NewMultiCollector(singboxCollector, iptablesCollector)
```

### 10.3 对接 Laravel 控制面

Laravel 端需要实现以下 API 端点：

#### `POST /api/node/heartbeat`

接收 Node Agent 的心跳上报。

**请求 Header**：

| Header | 说明 |
|--------|------|
| `X-Node-Token` | 节点认证 Token |
| `X-Node-ID` | 节点 ID |
| `Content-Type` | `application/json` |

**请求体**：与 [5.5 心跳数据接口](#55-心跳数据接口) 响应格式一致。

**Laravel 端处理逻辑**：

```php
// 1. 验证 Token
$node = Node::where('node_id', $request->header('X-Node-ID'))
    ->where('token', $request->header('X-Node-Token'))
    ->firstOrFail();

// 2. 更新节点状态
$node->update([
    'status' => $payload['status']['singbox_running'] ? 'online' : 'offline',
    'last_heartbeat' => now(),
    'cpu_percent' => $payload['system']['cpu_percent'],
    'mem_percent' => $payload['system']['mem_percent'],
]);

// 3. 记录流量
TrafficLog::create([
    'node_id' => $node->id,
    'upload' => $payload['traffic']['upload'],
    'download' => $payload['traffic']['download'],
    'online_users' => $payload['online']['user_count'],
]);
```

#### Laravel 向 Node Agent 下发配置

```php
// 在 Laravel 控制器中
public function deploy(Node $node, User $user)
{
    $response = Http::withHeaders([
        'X-Node-Token' => $node->agent_token,
        'Content-Type' => 'application/json',
    ])->post("http://{$node->ip}:{$node->agent_port}/deploy", [
        'user_id' => $user->id,
        'node_id' => $node->node_id,
        'protocol' => $node->protocol,
        'server' => $node->server,
        'port' => $node->port,
        'password' => $user->vpn_token,
        'sni' => $node->sni,
    ]);

    return $response->json();
}
```

### 10.4 多节点负载均衡

Node Agent 天然支持多节点架构。每个 VPS 运行独立的 Node Agent 实例，Laravel 控制面负责调度：

**策略一：基于地域分配**

```
用户选择地区 → Laravel 查询该地区可用节点 → 选择负载最低的节点 → 调用该节点 /deploy
```

**策略二：基于负载分配**

```
Laravel 定期收集所有节点心跳 → 按在线用户数/流量排序 → 新用户分配到负载最低的节点
```

**策略三：基于权重分配**

```
每个节点配置权重 → Laravel 按权重轮询分配 → 支持动态调整权重
```

---

## 11. 设计决策与权衡

| 决策 | 选择 | 原因 | 权衡 |
|------|------|------|------|
| HTTP 框架 | 标准库 `net/http` | 零依赖、性能足够、可读性好 | 不如 gin/echo 便捷 |
| 进程管理 | `exec.CommandContext` + `Setpgid` | 精确控制进程生命周期 | 不如 supervisor 功能丰富 |
| 配置写入 | `write .tmp → rename` | 原子性保证 | 多一次文件系统操作 |
| 全局流量统计 | Clash API + Fallback | 利用 sing-box 内置能力 | 依赖 sing-box API 稳定性 |
| 按用户流量统计 | V2Ray API gRPC | 原生支持按用户/出站粒度统计 | 需 sing-box 使用 with_v2ray_api 编译 |
| 设备管理 | 内存 map | 简单高效 | 进程重启后丢失（可接受，设备会重新注册） |
| 心跳上报 | HTTP POST | 简单可靠 | 不如 gRPC 高效 |
| 状态机 | 5 状态枚举 | 清晰表达进程生命周期 | 状态转换需严格校验 |
| Token 认证 | Header + Query 双模式 | 兼容不同客户端 | Query Token 可能泄露到日志 |
| CORS 支持 | 内置中间件 | 兼容 Web 测试页和前端管理面板 | 需注意生产环境安全配置 |
| 日志捕获 | RingBuffer | 固定内存、实时查询 | 缓冲区满时旧日志被覆盖 |
| 跨平台编译 | 平台特定文件 | Unix/Windows 进程信号差异 | 需维护多份平台代码 |
| 依赖管理 | gopsutil + gRPC | 最小化外部依赖 | 二进制体积稍大 |

---

*本文档基于 Node Agent v1.5.0 源码生成，与代码实现完全一致。*
