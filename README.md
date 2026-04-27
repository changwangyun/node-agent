# Node Agent — VPN 节点控制系统技术文档

> 版本：1.6.0  
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
  - [4.4 TLS 证书自动生成模块 (core/configgen/certgen)](#44-tls-证书自动生成模块-coreconfiggencertgen)
  - [4.5 流量统计模块 (core/stats)](#45-流量统计模块-corestats)
  - [4.6 设备限制模块 (core/device)](#46-设备限制模块-coredevice)
  - [4.7 心跳上报模块 (core/heartbeat)](#47-心跳上报模块-coreheartbeat)
  - [4.8 系统信息采集模块 (utils)](#48-系统信息采集模块-utils)
- [5. API 接口文档](#5-api-接口文档)
  - [5.1 认证方式](#51-认证方式)
  - [5.2 节点部署接口](#52-节点部署接口)
  - [5.3 客户端配置查询接口](#53-客户端配置查询接口)
  - [5.4 节点状态接口](#54-节点状态接口)
  - [5.5 流量统计接口](#55-流量统计接口)
  - [5.6 按用户流量统计接口](#56-按用户流量统计接口)
  - [5.7 心跳数据接口](#57-心跳数据接口)
  - [5.8 进程控制接口](#58-进程控制接口)
  - [5.9 设备管理接口](#59-设备管理接口)
  - [5.10 日志查询接口](#510-日志查询接口)
  - [5.11 健康检查接口](#511-健康检查接口)
- [6. 安全机制](#6-安全机制)
  - [6.1 Token 认证](#61-token-认证)
  - [6.2 IP 白名单](#62-ip-白名单)
  - [6.3 Deploy 签名验证](#63-deploy-签名验证)
  - [6.4 中间件执行链](#64-中间件执行链)
  - [6.5 CORS 跨域支持](#65-cors-跨域支持)
- [7. 配置参考](#7-配置参考)
  - [7.1 完整配置文件](#71-完整配置文件)
  - [7.2 配置项说明](#72-配置项说明)
- [8. 部署指南](#8-部署指南)
  - [8.1 编译](#81-编译)
  - [8.2 一键安装](#82-一键安装)
  - [8.3 手动安装](#83-手动安装)
  - [8.4 systemd 管理](#84-systemd-管理)
- [9. 客户端连接指南](#9-客户端连接指南)
  - [9.1 获取客户端配置](#91-获取客户端配置)
  - [9.2 使用 URI 连接](#92-使用-uri-连接)
  - [9.3 使用 JSON 配置连接](#93-使用-json-配置连接)
  - [9.4 各协议客户端参数说明](#94-各协议客户端参数说明)
- [10. 运维手册](#10-运维手册)
  - [10.1 日常运维命令](#101-日常运维命令)
  - [10.2 日志查看](#102-日志查看)
  - [10.3 故障排查](#103-故障排查)
  - [10.4 sing-box 配置模板](#104-sing-box-配置模板)
- [11. 扩展指南](#11-扩展指南)
  - [11.1 新增协议支持](#111-新增协议支持)
  - [11.2 自定义统计后端](#112-自定义统计后端)
  - [11.3 对接 Laravel 控制面](#113-对接-laravel-控制面)
  - [11.4 多节点负载均衡](#114-多节点负载均衡)
- [12. 设计决策与权衡](#12-设计决策与权衡)
- [13. 版本变更记录](#13-版本变更记录)

---

## 1. 项目概述

Node Agent 是一个运行在每台 VPS 上的 VPN 节点控制守护进程，是整个 VPN 平台的**核心执行层**。它负责：

- **sing-box 生命周期管理**：启动、停止、重启、崩溃自动恢复
- **动态配置下发**：接收控制面指令，生成服务端 sing-box 配置并热更新
- **多协议支持**：Hysteria2 / VLESS / Reality，可扩展
- **服务端配置生成**：生成服务端入站配置（inbound），同时生成客户端连接配置
- **TLS 证书管理**：自动生成自签名证书、支持 ACME 自动签发、支持自定义证书
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
│  │  ┌──────────────────────┐│   │
│  │  │ CertGen (TLS证书)    ││   │
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
| 执行层 | **Node Agent** | 接收指令、管理 sing-box、采集数据、上报状态、生成客户端配置 |
| 网络层 | sing-box | 实际处理 VPN 流量（代理、路由、加密） |

Node Agent 是业务层与网络层之间的**桥梁**，将高层业务指令翻译为底层 sing-box 服务端配置，同时生成客户端连接配置供用户使用，并将底层运行状态汇总上报给业务层。

### 2.3 数据流

```
Laravel ──POST /deploy──▶ Node Agent ──生成服务端 config.json──▶ sing-box restart
                                    └──生成客户端配置──▶ 返回 client_config 给 Laravel
Laravel ──GET /client-config──▶ Node Agent ──查询用户客户端配置──▶ 返回 URI + JSON
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
│   │   ├── generator.go                 # sing-box config.json 动态生成（服务端入站配置 + 客户端配置 + URI 生成）
│   │   └── certgen.go                   # TLS 自签名证书自动生成（ECDSA P256 + x509）
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
│   └── install.sh                       # 一键部署脚本（含源码编译 with_v2ray_api,with_quic,with_clash_api）
├── .github/
│   └── workflows/
│       ├── ci.yml                       # CI 测试工作流
│       └── release.yml                  # 自动编译发布工作流
└── test.php                             # Web 测试页面（API 测试 + Token 生成 + 客户端配置展示）
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
     │       Stopping ◀──────┘  Crashed
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
| `GetLastError() string` | 获取最近一次错误信息 |
| `GetCrashTime() time.Time` | 获取最近一次崩溃时间 |
| `CrashChannel() <-chan struct{}` | 获取崩溃通知 channel |

#### Watchdog 自动恢复

在 [main.go](file:///Volumes/koeyx/box/node-agent/main.go) 中启动独立 goroutine 运行 Watchdog：

```go
func startWatchdog(mgr *singbox.Manager, cfg *config.Config) {
    ticker := time.NewTicker(interval)
    for range ticker.C {
        if !mgr.IsRunning() && mgr.GetState() == singbox.StateCrashed {
            mgr.Start()
        }
    }
}
```

Watchdog 以可配置间隔（默认 5 秒）检测 sing-box 状态，发现崩溃后自动拉起。若配置文件不存在则跳过重启，避免反复启动失败。

---

### 4.3 配置动态生成模块 (core/configgen)

**文件**：[core/configgen/generator.go](file:///Volumes/koeyx/box/node-agent/core/configgen/generator.go)

#### 功能

- 根据 DeployRequest 动态生成**服务端** sing-box config.json（入站配置）
- 同时生成**客户端**连接配置（含 URI 和完整 JSON 配置）
- 支持 Hysteria2 / VLESS / Reality 三种协议
- 支持 TLS 自签名证书、ACME 自动签发、自定义证书
- 支持 Hysteria2 混淆（obfs）和带宽限制
- 支持 Reality 协议的密钥对和握手配置
- 原子写入配置文件（先写 `.tmp` 再 `rename`）
- 管理多用户部署记录

#### DeployRequest 结构

```go
type DeployRequest struct {
    UserID   string `json:"user_id"`              // 用户 ID（必填）
    NodeID   string `json:"node_id"`              // 节点 ID（必填）
    Protocol string `json:"protocol"`             // 协议：hysteria2 / vless / reality（必填）
    Server   string `json:"server"`               // 服务器地址（默认 0.0.0.0）
    Port     int    `json:"port"`                 // 服务器端口（必填）
    Password string `json:"password"`             // 密码 / Token

    UUID string `json:"uuid,omitempty"`           // VLESS/Reality UUID（默认使用 Password）

    SNI string `json:"sni,omitempty"`             // TLS SNI（默认使用 Server）

    ObfsType string `json:"obfs_type,omitempty"`       // Hysteria2 混淆类型
    ObfsPass string `json:"obfs_password,omitempty"`    // Hysteria2 混淆密码

    UpMbps   int `json:"up_mbps,omitempty"`       // Hysteria2 上行带宽限制
    DownMbps int `json:"down_mbps,omitempty"`      // Hysteria2 下行带宽限制

    TLSCertPath string `json:"tls_cert_path,omitempty"` // 自定义 TLS 证书路径
    TLSKeyPath  string `json:"tls_key_path,omitempty"`  // 自定义 TLS 密钥路径

    ACMEDomain string `json:"acme_domain,omitempty"`    // ACME 自动签发域名
    ACMEEmail  string `json:"acme_email,omitempty"`     // ACME 邮箱

    RealityPrivateKey string `json:"reality_private_key,omitempty"` // Reality 私钥（服务端）
    RealityPublicKey  string `json:"reality_public_key,omitempty"`  // Reality 公钥（客户端）
    RealityShortID    string `json:"reality_short_id,omitempty"`    // Reality Short ID
    RealityDest       string `json:"reality_dest,omitempty"`        // Reality 握手目标（默认 www.microsoft.com）
    RealityDestPort   int    `json:"reality_dest_port,omitempty"`   // Reality 握手目标端口（默认 443）
}
```

#### ClientConfigResult 结构

Deploy 成功后返回的客户端配置信息：

```go
type ClientConfigResult struct {
    SingBoxConfig string `json:"singbox_config"`  // 完整的 sing-box 客户端 JSON 配置
    URI           string `json:"uri"`             // 协议 URI（可直接导入客户端）
    Protocol      string `json:"protocol"`        // 协议类型
    Server        string `json:"server"`          // 服务器地址
    Port          int    `json:"port"`            // 服务器端口
    Insecure      bool   `json:"insecure"`        // 是否使用不安全 TLS（自签名证书时为 true）
}
```

#### 服务端配置生成逻辑

Deploy 时生成**服务端** sing-box 配置，核心流程：

1. **确定 TLS 证书来源**：
   - Reality 协议：不需要 TLS 证书
   - 指定了 ACME 域名：使用 ACME 自动签发
   - 指定了证书路径：使用自定义证书
   - 均未指定：自动生成自签名证书（ECDSA P256，有效期 10 年）

2. **生成服务端入站配置（inbound）**：
   - Hysteria2：`type: "hysteria2"`，含 users、TLS、obfs、带宽限制
   - VLESS：`type: "vless"`，含 users（UUID + flow）、TLS
   - Reality：`type: "vless"`，含 Reality TLS（private_key、short_id、handshake）

3. **生成客户端出站配置（outbound）**：
   - 根据协议生成对应的客户端出站配置
   - 自动从 Reality 私钥推导公钥
   - 自签名证书时标记 `insecure: true`

4. **生成客户端 URI**：
   - Hysteria2：`hysteria2://password@server:port?sni=xxx`
   - VLESS：`vless://uuid@server:port?security=tls&sni=xxx`
   - Reality：`vless://uuid@server:port?security=reality&pbk=xxx&sid=xxx`

#### 协议服务端配置映射

**Hysteria2 服务端入站**：

```json
{
  "type": "hysteria2",
  "tag": "hysteria2-in",
  "listen": "0.0.0.0",
  "listen_port": 443,
  "users": [{"password": "user_password"}],
  "tls": {
    "enabled": true,
    "server_name": "example.com",
    "certificate_path": "/path/to/cert.pem",
    "key_path": "/path/to/key.pem"
  },
  "obfs": {"type": "salamander", "password": "obfs_pass"},
  "up_mbps": 100,
  "down_mbps": 100
}
```

**VLESS 服务端入站**：

```json
{
  "type": "vless",
  "tag": "vless-in",
  "listen": "0.0.0.0",
  "listen_port": 443,
  "users": [{"uuid": "user_uuid", "flow": "xtls-rprx-vision"}],
  "tls": {
    "enabled": true,
    "server_name": "example.com",
    "certificate_path": "/path/to/cert.pem",
    "key_path": "/path/to/key.pem"
  }
}
```

**Reality 服务端入站**：

```json
{
  "type": "vless",
  "tag": "reality-in",
  "listen": "0.0.0.0",
  "listen_port": 443,
  "users": [{"uuid": "user_uuid", "flow": "xtls-rprx-vision"}],
  "tls": {
    "enabled": true,
    "server_name": "www.microsoft.com",
    "reality": {
      "enabled": true,
      "private_key": "base64_private_key",
      "short_id": ["abc123"],
      "handshake": {
        "server": "www.microsoft.com",
        "server_port": 443
      }
    }
  }
}
```

#### 生成的完整服务端配置结构

每次 Deploy 会生成包含以下部分的完整 sing-box 服务端配置：

| 部分 | 内容 |
|------|------|
| `log` | 日志级别 info，启用时间戳 |
| `dns` | Google DNS (tls, 8.8.8.8) + 阿里 DNS (udp, 223.5.5.5)，新格式（type + server） |
| `inbounds` | 协议对应的服务端入站配置 |
| `outbounds` | direct 出站 |
| `route` | sniff + hijack-dns 规则动作，default_domain_resolver，final 走 direct |
| `experimental` | Clash API (0.0.0.0:9090) + V2Ray API (127.0.0.1:10001, stats.enabled) |

#### 原子写入机制

```
写入 config.json.tmp → rename config.json.tmp → config.json
```

`os.Rename` 在同一文件系统上是原子操作，确保配置文件不会出现半写状态。

---

### 4.4 TLS 证书自动生成模块 (core/configgen/certgen)

**文件**：[core/configgen/certgen.go](file:///Volumes/koeyx/box/node-agent/core/configgen/certgen.go)

#### 功能

当 Hysteria2 或 VLESS 协议未提供 TLS 证书且未启用 ACME 时，自动生成自签名 TLS 证书。

#### 证书规格

| 属性 | 值 |
|------|------|
| 算法 | ECDSA P-256 |
| 有效期 | 10 年（3650 天） |
| 用途 | 数字签名 + 密钥加密 + 服务端认证 |
| CN | SNI 域名 |
| SAN | SNI 域名 |
| 组织 | Node Agent Self-Signed |

#### 自动生成逻辑

```
Deploy 请求
  │
  ├── Reality 协议？── 是 ──▶ 不需要证书
  │
  ├── 指定了 ACME 域名？── 是 ──▶ 使用 ACME 自动签发
  │
  ├── 指定了证书路径？── 是 ──▶ 使用自定义证书
  │
  └── 均未指定 ──▶ 自动生成自签名证书
       │
       ├── 证书已存在？── 是 ──▶ 跳过生成，复用已有证书
       │
       └── 证书不存在 ──▶ 生成新证书
            │
            ├── 证书保存到 {config_dir}/self-signed-cert.pem
            └── 私钥保存到 {config_dir}/self-signed-key.pem
```

#### 关键方法

| 方法 | 说明 |
|------|------|
| `GenerateSelfSignedCert(certPath, keyPath, domain)` | 生成自签名证书，已存在则跳过 |

#### 注意事项

- 自签名证书的客户端需要设置 `insecure: true` 才能连接
- 生产环境建议使用 ACME 或自定义证书
- 证书文件生成后会被持久化，重启不会重新生成

---

### 4.5 流量统计模块 (core/stats)

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

通过 V2Ray API 的 gRPC 接口采集按用户粒度的流量统计数据。**前提条件**：sing-box 必须使用 `-tags with_v2ray_api,with_quic,with_clash_api` 编译。

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
        "outbounds": ["direct"]
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
| `outbound>>>direct>>>traffic>>>uplink` | 出站 `direct` 的上传流量 |

---

### 4.6 设备限制模块 (core/device)

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

### 4.7 心跳上报模块 (core/heartbeat)

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

### 4.8 系统信息采集模块 (utils)

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

下发用户节点配置，生成服务端 sing-box config.json 并重启 sing-box，同时返回客户端连接配置。

**请求体**：

```json
{
  "user_id": "123",
  "node_id": "n1",
  "protocol": "hysteria2",
  "server": "hk1.xxx.com",
  "port": 443,
  "password": "token_xxx",
  "sni": "hk1.xxx.com"
}
```

**字段说明**：

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `user_id` | string | 是 | 用户 ID |
| `node_id` | string | 是 | 节点 ID |
| `protocol` | string | 是 | 协议类型：`hysteria2` / `vless` / `reality` |
| `server` | string | 否 | 服务器地址（默认 `0.0.0.0`，客户端配置中会替换为 `YOUR_SERVER_IP`） |
| `port` | int | 是 | 服务器端口 |
| `password` | string | 否 | 认证密码/Token |
| `uuid` | string | 否 | VLESS/Reality 的 UUID（默认使用 password） |
| `sni` | string | 否 | TLS SNI（默认使用 server） |
| `obfs_type` | string | 否 | Hysteria2 混淆类型（如 `salamander`） |
| `obfs_password` | string | 否 | Hysteria2 混淆密码 |
| `up_mbps` | int | 否 | Hysteria2 上行带宽限制（Mbps） |
| `down_mbps` | int | 否 | Hysteria2 下行带宽限制（Mbps） |
| `tls_cert_path` | string | 否 | 自定义 TLS 证书路径 |
| `tls_key_path` | string | 否 | 自定义 TLS 密钥路径 |
| `acme_domain` | string | 否 | ACME 自动签发域名（启用后自动签发 Let's Encrypt 证书） |
| `acme_email` | string | 否 | ACME 邮箱 |
| `reality_private_key` | string | 否 | Reality 私钥（服务端，protocol=reality 时必填） |
| `reality_public_key` | string | 否 | Reality 公钥（客户端使用，不填则从私钥自动推导） |
| `reality_short_id` | string | 否 | Reality Short ID |
| `reality_dest` | string | 否 | Reality 握手目标（默认 `www.microsoft.com`） |
| `reality_dest_port` | int | 否 | Reality 握手目标端口（默认 `443`） |

**TLS 证书优先级**：

```
Reality 协议 → 不需要证书
ACME 域名 → ACME 自动签发
自定义证书路径 → 使用自定义证书
均未指定 → 自动生成自签名证书
```

**成功响应** (`200 OK`)：

```json
{
  "success": true,
  "message": "deployed hysteria2 config for user 123",
  "node_id": "n1",
  "client_config": {
    "singbox_config": "{...完整客户端JSON配置...}",
    "uri": "hysteria2://token_xxx@hk1.xxx.com:443?sni=hk1.xxx.com#hysteria2-n1",
    "protocol": "hysteria2",
    "server": "hk1.xxx.com",
    "port": 443,
    "insecure": false
  }
}
```

**client_config 字段说明**：

| 字段 | 类型 | 说明 |
|------|------|------|
| `singbox_config` | string | 完整的 sing-box 客户端 JSON 配置，可直接写入客户端配置文件 |
| `uri` | string | 协议 URI，可导入支持该格式的客户端 |
| `protocol` | string | 协议类型 |
| `server` | string | 服务器地址（若 server 为 0.0.0.0 则替换为 YOUR_SERVER_IP） |
| `port` | int | 服务器端口 |
| `insecure` | bool | 是否使用不安全 TLS（自签名证书时为 true） |

**错误响应**：

| 状态码 | 场景 |
|--------|------|
| 400 | 缺少必填字段 |
| 500 | 配置生成失败 / sing-box 启动失败 |

---

### 5.3 客户端配置查询接口

#### `GET /client-config`

查询已部署用户的客户端连接配置。

**查询参数**：

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `user_id` | string | 是 | 用户 ID |

**请求示例**：

```
GET /client-config?user_id=123
```

**成功响应** (`200 OK`)：

```json
{
  "singbox_config": "{...完整客户端JSON配置...}",
  "uri": "hysteria2://token_xxx@hk1.xxx.com:443?sni=hk1.xxx.com#hysteria2-n1",
  "protocol": "hysteria2",
  "server": "hk1.xxx.com",
  "port": 443,
  "insecure": false
}
```

**错误响应**：

| 状态码 | 场景 |
|--------|------|
| 400 | 缺少 user_id 参数 |
| 404 | 该用户未部署配置 |

---

### 5.4 节点状态接口

#### `GET /status`

返回节点完整状态信息。

**响应** (`200 OK`)：

```json
{
  "node": {
    "singbox_running": true,
    "singbox_state": "running",
    "uptime_seconds": 3600,
    "pid": 12345,
    "last_error": "",
    "crash_time": ""
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

当 sing-box 发生崩溃时，`node` 中会包含 `last_error` 和 `crash_time` 字段。

---

### 5.5 流量统计接口

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

### 5.6 按用户流量统计接口

#### `GET /traffic/user`

通过 V2Ray API 查询按用户粒度的流量统计数据。

**前提条件**：sing-box 必须使用 `-tags with_v2ray_api,with_quic,with_clash_api` 编译，且配置中启用了 `v2ray_api.stats`。

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
      "user_id": "outbound:direct",
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
| 503 | V2Ray API 不可用（sing-box 未使用 with_v2ray_api,with_quic,with_clash_api 编译） |
| 500 | gRPC 查询失败 |

**与 /stats 接口的区别**：

| 接口 | 粒度 | 数据来源 | 用途 |
|------|------|----------|------|
| `GET /stats` | 节点级（全局总量）+ 在线用户 | Clash API | 监控节点整体负载、在线用户 |
| `GET /traffic/user` | 用户级（按用户统计） | V2Ray API | 计费、用户流量配额管理 |
| `GET /online` | 在线用户详情 | Clash API | 实时在线用户列表、IP追踪 |

---

### 5.7 在线用户接口

#### `GET /online`

查询当前在线的 VPN 用户列表，包含用户 ID、入站标签、来源 IP 和实时流量。

**前提条件**：sing-box 必须使用 `-tags with_v2ray_api,with_quic,with_clash_api` 编译，且配置中用户必须包含 `name` 字段。

**响应** (`200 OK`)：

```json
{
  "count": 2,
  "online_users": [
    {
      "user_id": "user-001",
      "inbound": "hysteria2-in",
      "ip": "203.0.113.50",
      "upload": 1048576,
      "download": 5242880
    },
    {
      "user_id": "user-002",
      "inbound": "vless-in",
      "ip": "198.51.100.25",
      "upload": 204800,
      "download": 1024000
    }
  ]
}
```

**工作原理**：通过 Clash API 的 `/connections` 接口获取所有活跃连接，解析每个连接的 `inboundUser` 字段（即部署时设置的 `name`），按用户聚合后返回。

> **注意**：`/stats` 接口现在也会返回 `online_users`（在线用户数）、`online_details`（在线用户详情）和 `user_traffic`（V2Ray 按用户流量统计）字段。

---

### 5.8 心跳数据接口

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

### 5.8 进程控制接口

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

### 5.9 设备管理接口

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

### 5.10 日志查询接口

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
    "[singbox:err] INFO[0000] network: updated default interface eth0, index 2",
    "[singbox:err] FATAL[0000] start service: create v2ray-server: v2ray api is not included in this build"
  ],
  "count": 2
}
```

**日志来源**：sing-box 的 stderr 输出，由 Manager 通过 RingBuffer 实时捕获。

---

### 5.11 健康检查接口

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
    "allowed_origins": ["*"]
  }
}
```

**CORS 响应头**（在中间件中固定配置）：

| 响应头 | 值 |
|--------|------|
| `Access-Control-Allow-Methods` | `GET, POST, PUT, DELETE, OPTIONS` |
| `Access-Control-Allow-Headers` | `Content-Type, X-Node-Token, X-Signature, X-Timestamp` |
| `Access-Control-Max-Age` | `86400`（24 小时预检缓存） |

**安全建议**：

- 生产环境应将 `allowed_origins` 设为具体域名，避免使用 `*`
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
    "allowed_origins": ["*"]
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

> CORS 的 `Access-Control-Allow-Methods` 和 `Access-Control-Allow-Headers` 在中间件中固定配置，无需手动设置。

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
3. 从源码编译 sing-box（含 `-tags with_v2ray_api,with_quic,with_clash_api`，支持按用户流量统计和 Hysteria2）
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
# 检查 sing-box 是否包含 v2ray_api 和 quic
sing-box version
# 输出应包含 "with_v2ray_api"、"with_quic" 和 "with_clash_api"

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

# 3. 安装 sing-box（从源码编译，含 with_v2ray_api,with_quic,with_clash_api）
# 3a. 安装 Go
wget https://go.dev/dl/go1.23.4.linux-amd64.tar.gz
tar -C /usr/local -xzf go1.23.4.linux-amd64.tar.gz
export PATH=$PATH:/usr/local/go/bin

# 3b. 编译 sing-box
go install -tags "with_v2ray_api,with_quic,with_clash_api" github.com/sagernet/sing-box/cmd/sing-box@latest
cp $(go env GOPATH)/bin/sing-box /usr/local/bin/
chmod +x /usr/local/bin/sing-box

# 3c. 验证
sing-box version  # 应显示 with_v2ray_api、with_quic 和 with_clash_api

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
    "allowed_origins": ["*"]
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

## 9. 客户端连接指南

### 9.1 获取客户端配置

通过 Deploy 接口部署协议配置后，响应中会包含 `client_config` 字段，提供客户端连接所需的所有信息。

**方式一：Deploy 时直接获取**

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
    "password": "hy2-password"
  }' | jq '.client_config'
```

**方式二：通过 /client-config 接口查询**

```bash
curl -s "http://localhost:8080/client-config?user_id=user-001" \
  -H "X-Node-Token: your-token" | jq .
```

### 9.2 使用 URI 连接

客户端配置中的 `uri` 字段可直接导入支持该格式的客户端：

**Hysteria2 URI 格式**：

```
hysteria2://password@server:port?sni=xxx&insecure=1#hysteria2-node-id
```

**VLESS URI 格式**：

```
vless://uuid@server:port?encryption=none&flow=xtls-rprx-vision&security=tls&sni=xxx&type=tcp&fp=chrome#vless-node-id
```

**Reality URI 格式**：

```
vless://uuid@server:port?encryption=none&flow=xtls-rprx-vision&security=reality&sni=xxx&type=tcp&fp=chrome&pbk=public_key&sid=short_id#reality-node-id
```

**支持的客户端**：

| 客户端 | 平台 | URI 导入 | JSON 配置 |
|--------|------|----------|-----------|
| sing-box | iOS/Android/macOS/Windows/Linux | 支持 | 支持 |
| Clash Verge | Windows/macOS/Linux | 支持 | 需转换 |
| v2rayN | Windows | 支持 | 需转换 |
| Shadowrocket | iOS | 支持 | 不支持 |
| NekoBox | Android | 支持 | 支持 |

### 9.3 使用 JSON 配置连接

客户端配置中的 `singbox_config` 字段是完整的 sing-box 客户端 JSON 配置，可直接用于 sing-box 客户端。

**使用方法**：

1. 将 `singbox_config` 内容保存为 `config.json`
2. 使用 sing-box 客户端加载：`sing-box run -c config.json`

**客户端配置结构**：

| 部分 | 内容 |
|------|------|
| `log` | 日志级别 info，启用时间戳 |
| `dns` | Google DNS (tls) + 阿里 DNS (udp) |
| `inbounds` | TUN 模式入站（172.19.0.1/30，MTU 9000，自动路由） |
| `outbounds` | 协议对应的代理出站 + direct 直连 |
| `route` | sniff + hijack-dns，final 走 proxy |

**注意**：客户端配置使用 TUN 模式，需要管理员/root 权限运行。移动端客户端（如 sing-box iOS/Android）会自动处理 TUN 权限。

### 9.4 各协议客户端参数说明

#### Hysteria2

| 参数 | 说明 | 默认值 |
|------|------|--------|
| `password` | 认证密码 | 必填 |
| `sni` | TLS SNI | 使用 server 值 |
| `obfs_type` | 混淆类型（如 `salamander`） | 无 |
| `obfs_password` | 混淆密码 | 无 |
| `insecure` | 允许不安全 TLS（自签名证书时为 `true`） | `false` |

**自签名证书说明**：当未提供 TLS 证书且未启用 ACME 时，Node Agent 会自动生成自签名证书。此时客户端配置中 `insecure` 为 `true`，客户端需要信任该证书才能连接。生产环境建议使用 ACME 或自定义证书。

#### VLESS

| 参数 | 说明 | 默认值 |
|------|------|--------|
| `uuid` | 用户 UUID | 使用 password 值 |
| `flow` | 流控模式 | `xtls-rprx-vision` |
| `sni` | TLS SNI | 使用 server 值 |
| `insecure` | 允许不安全 TLS | `false` |

#### Reality

| 参数 | 说明 | 默认值 |
|------|------|--------|
| `uuid` | 用户 UUID | 使用 password 值 |
| `flow` | 流控模式 | `xtls-rprx-vision` |
| `sni` | TLS SNI（伪装域名） | 使用 reality_dest 值 |
| `public_key` | Reality 公钥 | 从私钥自动推导 |
| `short_id` | Short ID | 部署时指定 |

**Reality 密钥对**：Reality 协议使用 X25519 密钥对。服务端配置 `private_key`，客户端配置 `public_key`。如果在 Deploy 请求中只提供了 `reality_private_key`，Node Agent 会自动推导出公钥并填入客户端配置。

---

## 10. 运维手册

### 10.1 日常运维命令

```bash
# 查看服务状态
systemctl status node-agent

# 查看实时日志
journalctl -u node-agent -f

# 查看最近 100 行日志
journalctl -u node-agent -n 100

# 重启服务
systemctl restart node-agent

# 停止服务
systemctl stop node-agent

# 修改配置后重启
vi /etc/node-agent/config.json
systemctl restart node-agent
```

**通过 API 管理**：

```bash
TOKEN="your-api-token"
HOST="http://localhost:8080"

# 查看节点状态
curl -s -H "X-Node-Token: $TOKEN" $HOST/status | jq .

# 查看流量统计
curl -s -H "X-Node-Token: $TOKEN" $HOST/stats | jq .

# 查看按用户流量
curl -s -H "X-Node-Token: $TOKEN" "$HOST/traffic/user" | jq .

# 查看指定用户流量
curl -s -H "X-Node-Token: $TOKEN" "$HOST/traffic/user?user_id=user-001" | jq .

# 查看 sing-box 日志
curl -s -H "X-Node-Token: $TOKEN" "$HOST/logs?lines=20" | jq .

# 重启 sing-box
curl -s -X POST -H "X-Node-Token: $TOKEN" $HOST/restart | jq .

# 查看客户端配置
curl -s -H "X-Node-Token: $TOKEN" "$HOST/client-config?user_id=user-001" | jq .

# 部署协议配置
curl -s -X POST -H "X-Node-Token: $TOKEN" -H "Content-Type: application/json" \
  $HOST/deploy -d '{
    "user_id": "user-001",
    "node_id": "node-hk-001",
    "protocol": "hysteria2",
    "server": "hk1.example.com",
    "port": 443,
    "password": "hy2-password"
  }' | jq .
```

### 10.2 日志查看

**Node Agent 日志**（systemd journal）：

```bash
# 实时跟踪
journalctl -u node-agent -f

# 按时间过滤
journalctl -u node-agent --since "2024-01-01 00:00:00" --until "2024-01-02 00:00:00"

# 按优先级过滤
journalctl -u node-agent -p err
```

**sing-box 日志**（通过 API 查询）：

```bash
# 最近 50 行日志
curl -s -H "X-Node-Token: $TOKEN" "$HOST/logs?lines=50" | jq '.lines[]'

# 最近 10 行日志
curl -s -H "X-Node-Token: $TOKEN" "$HOST/logs?lines=10" | jq '.lines[]'
```

**日志前缀说明**：

| 前缀 | 来源 | 说明 |
|------|------|------|
| `[singbox:out]` | sing-box stdout | 正常输出 |
| `[singbox:err]` | sing-box stderr | 错误和警告信息 |
| `[main]` | Node Agent 主进程 | 启动/停止/信号处理 |
| `[watchdog]` | Watchdog | 崩溃检测和自动恢复 |
| `[heartbeat]` | 心跳上报 | 心跳发送状态 |

### 10.3 故障排查

#### sing-box 启动失败

**症状**：`/status` 显示 `singbox_state: "crashed"`

**排查步骤**：

1. 查看崩溃原因：
   ```bash
   curl -s -H "X-Node-Token: $TOKEN" $HOST/status | jq '.node.last_error'
   curl -s -H "X-Node-Token: $TOKEN" $HOST/status | jq '.node.crash_time'
   ```

2. 查看 sing-box 日志：
   ```bash
   curl -s -H "X-Node-Token: $TOKEN" "$HOST/logs?lines=20" | jq '.lines[]'
   ```

3. 常见错误及解决方案：

| 错误信息 | 原因 | 解决方案 |
|----------|------|----------|
| `sing-box config not found` | 未部署配置 | 通过 `POST /deploy` 部署协议配置 |
| `legacy DNS servers is deprecated` | 使用了旧版 DNS 格式 | 确保使用 v1.6.0+ 版本，已自动使用新格式 |
| `v2ray api is not included in this build` | sing-box 未使用 with_v2ray_api 编译 | 重新编译：`go install -tags "with_v2ray_api,with_quic,with_clash_api" github.com/sagernet/sing-box/cmd/sing-box@latest` |
| `QUIC is not included in this build` | sing-box 未使用 with_quic 编译（Hysteria2 需要） | 重新编译：`go install -tags "with_v2ray_api,with_quic,with_clash_api" github.com/sagernet/sing-box/cmd/sing-box@latest` |
| `clash api is not included in this build` | sing-box 未使用 with_clash_api 编译（流量统计需要） | 重新编译：`go install -tags "with_v2ray_api,with_quic,with_clash_api" github.com/sagernet/sing-box/cmd/sing-box@latest` |
| `certificate path not found` | TLS 证书路径错误 | 检查证书文件是否存在，或让系统自动生成自签名证书 |
| `reality_private_key is required` | Reality 协议未提供私钥 | Deploy 请求中必须包含 `reality_private_key` |

#### 客户端无法连接

**排查步骤**：

1. 确认 sing-box 正在运行：
   ```bash
   curl -s -H "X-Node-Token: $TOKEN" $HOST/status | jq '.node.singbox_running'
   ```

2. 确认端口已监听：
   ```bash
   ss -tlnp | grep sing-box
   ```

3. 确认防火墙放行：
   ```bash
   # 检查 UFW
   ufw status | grep 443
   # 或检查 iptables
   iptables -L -n | grep 443
   ```

4. 确认客户端配置正确：
   - 自签名证书客户端需要开启 `insecure`
   - Reality 协议需要正确的 `public_key` 和 `short_id`
   - 服务器地址不能是 `0.0.0.0`，需要替换为实际 IP 或域名

5. 获取客户端配置验证：
   ```bash
   curl -s -H "X-Node-Token: $TOKEN" "$HOST/client-config?user_id=user-001" | jq .
   ```

#### V2Ray API 不可用

**症状**：`GET /traffic/user` 返回 503

**解决方案**：

1. 确认 sing-box 包含 v2ray_api：
   ```bash
   sing-box version
   # 应显示 with_v2ray_api、with_quic 和 with_clash_api
   ```

2. 若不含，重新编译：
   ```bash
   go install -tags "with_v2ray_api,with_quic,with_clash_api" github.com/sagernet/sing-box/cmd/sing-box@latest
   cp $(go env GOPATH)/bin/sing-box /usr/local/bin/
   systemctl restart node-agent
   ```

3. 确认配置中启用了 V2Ray API：
   ```bash
   cat /etc/sing-box/config.json | jq '.experimental.v2ray_api'
   ```

### 10.4 sing-box 配置模板

以下为 Node Agent 生成的各协议服务端配置模板，供参考和手动调试。

**Hysteria2 服务端配置**：

```json
{
  "log": {"level": "info", "timestamp": true},
  "dns": {
    "servers": [
      {"tag": "google", "type": "tls", "server": "8.8.8.8"},
      {"tag": "local", "type": "udp", "server": "223.5.5.5"}
    ]
  },
  "inbounds": [{
    "type": "hysteria2",
    "tag": "hysteria2-in",
    "listen": "0.0.0.0",
    "listen_port": 443,
    "users": [{"password": "your-password"}],
    "tls": {
      "enabled": true,
      "server_name": "example.com",
      "certificate_path": "/etc/sing-box/self-signed-cert.pem",
      "key_path": "/etc/sing-box/self-signed-key.pem"
    }
  }],
  "outbounds": [{"type": "direct", "tag": "direct"}],
  "route": {
    "rules": [{"action": "sniff"}, {"protocol": "dns", "action": "hijack-dns"}],
    "default_domain_resolver": "google",
    "final": "direct"
  },
  "experimental": {
    "clash_api": {"external_controller": "0.0.0.0:9090", "secret": "node-agent-stats"},
    "v2ray_api": {"listen": "127.0.0.1:10001", "stats": {"enabled": true, "outbounds": ["direct"]}}
  }
}
```

**Reality 服务端配置**：

```json
{
  "log": {"level": "info", "timestamp": true},
  "dns": {
    "servers": [
      {"tag": "google", "type": "tls", "server": "8.8.8.8"},
      {"tag": "local", "type": "udp", "server": "223.5.5.5"}
    ]
  },
  "inbounds": [{
    "type": "vless",
    "tag": "reality-in",
    "listen": "0.0.0.0",
    "listen_port": 443,
    "users": [{"uuid": "user-uuid", "flow": "xtls-rprx-vision"}],
    "tls": {
      "enabled": true,
      "server_name": "www.microsoft.com",
      "reality": {
        "enabled": true,
        "private_key": "base64-private-key",
        "short_id": ["abc12345"],
        "handshake": {"server": "www.microsoft.com", "server_port": 443}
      }
    }
  }],
  "outbounds": [{"type": "direct", "tag": "direct"}],
  "route": {
    "rules": [{"action": "sniff"}, {"protocol": "dns", "action": "hijack-dns"}],
    "default_domain_resolver": "google",
    "final": "direct"
  },
  "experimental": {
    "clash_api": {"external_controller": "0.0.0.0:9090", "secret": "node-agent-stats"},
    "v2ray_api": {"listen": "127.0.0.1:10001", "stats": {"enabled": true, "outbounds": ["direct"]}}
  }
}
```

---

## 11. 扩展指南

### 11.1 新增协议支持

以添加 Shadowsocks 协议为例：

1. **在 `generator.go` 中添加协议处理**：

```go
func (g *Generator) generateShadowsocksInbound(req *DeployRequest) (*Inbound, error) {
    return &Inbound{
        Type:       "shadowsocks",
        Tag:        "shadowsocks-in",
        Listen:     "0.0.0.0",
        ListenPort: req.Port,
        Method:     req.Method,
        Password:   req.Password,
    }, nil
}
```

2. **在 `generateInbound` 中添加分支**：

```go
case "shadowsocks":
    return g.generateShadowsocksInbound(req)
```

3. **在 `buildClientConfig` 中添加客户端配置**：

```go
case "shadowsocks":
    clientOutbound = Outbound{
        Type:       "shadowsocks",
        Tag:        "proxy",
        Server:     server,
        ServerPort: req.Port,
        Method:     req.Method,
        Password:   req.Password,
    }
    uri = g.buildShadowsocksURI(req, server)
```

4. **在 `DeployRequest` 中添加协议特有字段**：

```go
Method string `json:"method,omitempty"` // Shadowsocks 加密方法
```

5. **更新 `handler.go` 中的参数校验**（如需要）

### 11.2 自定义统计后端

实现 `stats.Collector` 接口即可替换默认的 Clash API 统计：

```go
type Collector interface {
    GetTraffic() (*TrafficData, error)
    GetConnections() (*ConnectionData, error)
    GetStats() (*StatsResult, error)
}
```

例如，基于 iptables 的统计：

```go
type IptablesCollector struct{}

func (c *IptablesCollector) GetTraffic() (*TrafficData, error) {
    // 解析 iptables -L -v -x 输出
    // 返回上传/下载字节数
}
```

在 `main.go` 中替换：

```go
customCollector := &IptablesCollector{}
multiCollector := stats.NewMultiCollector(customCollector, fallbackCollector)
```

### 11.3 对接 Laravel 控制面

Laravel 控制面需要实现以下 API 端点：

**接收心跳**：

```
POST /api/node/heartbeat
Header: X-Node-Token, X-Node-ID
Body: {心跳数据 JSON}
```

**下发部署指令**（Laravel 调用 Node Agent）：

```
POST http://node-ip:8080/deploy
Header: X-Node-Token
Body: {部署请求 JSON}
```

**查询节点状态**（Laravel 调用 Node Agent）：

```
GET http://node-ip:8080/status
Header: X-Node-Token
```

**Laravel 端示例代码**：

```php
// 部署协议配置到节点
public function deployToNode($node, $user, $protocol)
{
    $response = Http::withHeaders([
        'X-Node-Token' => $node->api_token,
    ])->post("http://{$node->ip}:8080/deploy", [
        'user_id'   => $user->id,
        'node_id'   => $node->id,
        'protocol'  => $protocol->type,
        'server'    => $node->domain ?? $node->ip,
        'port'      => $protocol->port,
        'password'  => $protocol->password,
        'sni'       => $protocol->sni,
    ]);

    if ($response->successful()) {
        $data = $response->json();
        // 保存 client_config 供用户下载
        $user->update([
            'client_config' => $data['client_config']['singbox_config'],
            'client_uri'    => $data['client_config']['uri'],
        ]);
    }
}
```

### 11.4 多节点负载均衡

**DNS 轮询方案**：

1. 为每个节点分配子域名（如 `hk1.example.com`、`hk2.example.com`）
2. 在 DNS 中配置 A 记录指向各节点 IP
3. 用户连接时使用统一域名，DNS 自动轮询分配

**控制面调度方案**：

1. Laravel 维护节点负载表（基于心跳数据）
2. 用户请求连接时，Laravel 选择负载最低的节点
3. 调用该节点的 `/deploy` 接口下发配置
4. 返回客户端配置给用户

**健康检查方案**：

1. 外部监控器定期调用各节点的 `/health` 端点
2. 节点不可用时自动从 DNS/负载均衡器中移除
3. Node Agent 的 Watchdog 机制确保 sing-box 崩溃后自动恢复

---

## 12. 设计决策与权衡

| 决策 | 选择 | 原因 | 权衡 |
|------|------|------|------|
| 进程管理 | 子进程模式 | sing-box 是独立二进制，子进程模式最稳定 | 需要处理僵尸进程、进程组信号 |
| 配置写入 | 原子写入（tmp + rename） | 避免半写状态导致 sing-box 启动失败 | 需要同一文件系统 |
| 流量统计 | Clash API + V2Ray API 双 API | Clash API 提供全局统计，V2Ray API 提供按用户统计 | V2Ray API 需要特殊编译标签 |
| 统计降级 | MultiCollector 自动降级 | Clash API 不可用时自动切换到 Fallback | Fallback 精度较低 |
| TLS 证书 | 自动生成自签名证书 | 零配置即可使用，降低部署门槛 | 自签名证书客户端需设置 insecure |
| Reality 公钥 | 从私钥自动推导 | 减少客户端配置复杂度 | 依赖 X25519 密钥推导正确性 |
| DNS 格式 | sing-box 1.12+ 新格式 | 避免废弃警告，面向未来兼容 | 旧版 sing-box 不兼容 |
| 路由规则 | sniff + hijack-dns 动作 | 替代废弃的 dns outbound | 需要 sing-box 1.11+ |
| CORS | 内置中间件 | 方便 Web 测试页面跨域访问 | 生产环境需限制 allowed_origins |
| 日志捕获 | RingBuffer（200 行） | 内存可控，避免日志文件膨胀 | 仅保留最近 200 行 |
| 客户端配置 | Deploy 时同时生成 | 一次部署即可获取所有连接信息 | 配置存储在内存中，重启后需重新部署 |
| Watchdog | 配置文件存在时才重启 | 避免无配置时反复启动失败 | 首次部署前 sing-box 不会自动启动 |

---

## 13. 版本变更记录

### v1.7.0 (2026-04-28)

**新功能**：

- 在线用户自动检测：通过 Clash API `/connections` 接口实时获取在线用户列表
- 新增 `GET /online` 接口：返回在线用户 ID、入站标签、来源 IP、实时流量
- `/stats` 接口增强：新增 `online_users`（在线用户数）、`online_details`（在线用户详情）、`user_traffic`（V2Ray 按用户流量）
- sing-box 用户配置添加 `name` 字段：将 `user_id` 写入用户名，使 V2Ray API 和 Clash API 能按用户识别流量

**修复**：

- sing-box 编译标签补全：完整标签为 `with_v2ray_api,with_quic,with_clash_api`
- 修复 Hysteria2 协议因缺少 `with_quic` 标签无法启动的问题
- 修复 Clash API 因缺少 `with_clash_api` 标签无法启动的问题

### v1.6.0 (2026-04-28)

**重大变更**：

- 配置生成从客户端模式改为**服务端模式**：生成服务端入站配置（inbound），同时生成客户端连接配置
- 新增 TLS 证书自动生成模块（`certgen.go`）：支持自签名证书、ACME 自动签发、自定义证书
- 新增客户端配置查询接口 `GET /client-config`
- Deploy 接口响应新增 `client_config` 字段，包含完整客户端配置和 URI

**新功能**：

- Reality 协议支持：自动从私钥推导公钥，支持握手目标配置
- Hysteria2 混淆（obfs）和带宽限制支持
- VLESS + TLS 服务端入站配置
- 客户端 URI 生成：Hysteria2 / VLESS / Reality 三种协议的 URI 格式

**兼容性修复**：

- 迁移到 sing-box 1.12+ 新 DNS 格式（`type` + `server` 替代 `address`）
- 添加 `route.default_domain_resolver` 字段
- 移除废弃的 `dns` outbound，改用 `sniff` + `hijack-dns` 路由动作
- `clash_api.listen` 重命名为 `external_controller`
- 移除废弃的 `block` outbound 和 `inet4_address` 字段
- sing-box 编译标签新增 `with_quic`（Hysteria2 协议依赖 QUIC 支持）和 `with_clash_api`（流量统计依赖 Clash API）

**其他改进**：

- 新增 CORS 跨域中间件
- sing-box 日志实时捕获（RingBuffer，200 行）
- 崩溃原因和崩溃时间追踪
- `/status` 接口返回 `last_error` 和 `crash_time`
- 新增 `/logs` 接口查询 sing-box 日志
- V2Ray API 按用户流量统计（gRPC 客户端）
- 新增 `GET /traffic/user` 接口
- 安装脚本支持从源码编译 sing-box（含 `with_v2ray_api,with_quic,with_clash_api` 标签）
- GitHub Actions 自动编译发布（推送 tag 触发）
- 跨平台编译支持（process_unix.go / process_windows.go）

### v1.0.0 (2026-04-25)

**初始版本**：

- sing-box 进程生命周期管理（启动/停止/重启/崩溃自动恢复）
- 动态配置生成（Hysteria2 / VLESS / Reality）
- HTTP API 服务（14 个接口端点）
- 流量统计（Clash API 全局统计 + MultiCollector 降级）
- 设备限制（设备绑定 + 并发会话控制）
- 心跳上报（节点状态/流量/在线信息/系统信息）
- 安全机制（Token 认证 / IP 白名单 / HMAC 签名验证）
- 一键安装脚本
- Web 测试页面（test.php）
- systemd 服务管理