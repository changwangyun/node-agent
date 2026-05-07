# Node Agent — VPN 节点控制系统技术文档

> 版本：1.15.0  
> 最后更新：2026-05-08

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
  - [5.12 用户移除接口](#512-用户移除接口)
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
  - [11.4 对接 Xboard 面板](#114-对接-xboard-面板)
  - [11.5 多节点负载均衡](#115-多节点负载均衡)
- [12. Laravel 控制面板开发指南](#12-laravel-控制面板开发指南)
  - [12.1 数据库设计](#121-数据库设计)
  - [12.2 核心服务实现](#122-核心服务实现)
  - [12.3 心跳接收与节点监控](#123-心跳接收与节点监控)
  - [12.4 用户流量采集与计费](#124-用户流量采集与计费)
  - [12.5 Deploy 签名密码机制](#125-deploy-签名密码机制)
  - [12.6 定时任务](#126-定时任务)
- [13. 客户端开发指南](#13-客户端开发指南)
  - [13.1 整体架构](#131-整体架构)
  - [13.2 连接流程](#132-连接流程)
  - [13.3 sing-box 客户端配置组装](#133-sing-box-客户端配置组装)
  - [13.4 Hysteria2 性能优化配置](#134-hysteria2-性能优化配置)
  - [13.5 流量监控与上报](#135-流量监控与上报)
  - [13.6 iOS 客户端实现要点](#136-ios-客户端实现要点)
  - [13.7 Android 客户端实现要点](#137-android-客户端实现要点)
- [14. 设计决策与权衡](#14-设计决策与权衡)
- [15. 版本变更记录](#15-版本变更记录)

---

## 1. 项目概述

Node Agent 是一个运行在每台 VPS 上的 VPN 节点控制守护进程，是整个 VPN 平台的**核心执行层**。它负责：

- **sing-box 生命周期管理**：启动、停止、重启、崩溃自动恢复
- **动态配置下发**：接收控制面指令，生成服务端 sing-box 配置并热更新（SIGHUP Reload，不断开用户连接）
- **多协议支持**：Hysteria2 / VLESS / Reality / Trojan，可扩展
- **服务端配置生成**：生成服务端入站配置（inbound），同时生成客户端连接配置
- **TLS 证书管理**：自动生成自签名证书、支持 ACME 自动签发、支持自定义证书
- **流量统计**：通过 Clash API 采集全局实时流量数据，通过 V2Ray API 采集按用户流量数据，累计流量持久化到磁盘
- **设备限制**：用户级设备绑定与并发会话控制
- **心跳上报**：定时向控制面汇报节点状态、流量、在线用户数，支持指数退避重试
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
│   ├── xboard/
│   │   ├── client.go                    # Xboard UniProxy API 客户端（/config, /user, /push）
│   │   └── sync.go                      # Xboard 同步逻辑（定时拉取配置/用户、上报流量）
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
| `Reload() error` | 热更新 sing-box（发送 SIGHUP，不断开用户连接，仅 Unix 平台支持） |
| `IsRunning() bool` | 检查是否运行中 |
| `GetState() ProcessState` | 获取当前状态 |
| `GetUptime() time.Duration` | 获取运行时长 |
| `GetPID() int` | 获取进程 PID |
| `GetLastError() string` | 获取最近一次错误信息 |
| `GetCrashTime() time.Time` | 获取最近一次崩溃时间 |
| `CrashChannel() <-chan struct{}` | 获取崩溃通知 channel |

#### 热更新机制（Reload）

`Reload()` 方法通过向 sing-box 进程发送 `SIGHUP` 信号实现配置热更新，无需重启进程，**不会断开已有用户连接**。

**工作原理**：

1. 获取 sing-box 进程 PID
2. 通过 `syscall.Kill(pid, syscall.SIGHUP)` 发送 SIGHUP 信号
3. sing-box 收到信号后重新加载配置文件

**Reload vs Restart 对比**：

| 特性 | Reload (SIGHUP) | Restart (Stop + Start) |
|------|-----------------|----------------------|
| 已有连接 | 保持不断开 | 全部断开 |
| 生效速度 | 毫秒级 | 数秒（停止+启动） |
| 适用场景 | 新增/移除用户、配置变更 | sing-box 版本升级、端口变更 |
| 平台支持 | 仅 Unix（Linux/macOS） | 全平台 |
| 失败回退 | 自动回退到 Restart | 无 |

**Deploy 流程中的使用**：

```
Deploy / RemoveDeploy
  │
  ├── 生成新配置 → 写入 config.json
  │
  ├── sing-box 运行中？
  │   ├── 是 → Reload()
  │   │   ├── 成功 → 返回成功
  │   │   └── 失败 → 回退 Restart()
  │   └── 否 → 不操作（下次 Deploy 时 Start）
  │
  └── 返回客户端配置
```

**注意**：Windows 平台不支持 SIGHUP，`Reload()` 会返回错误，Deploy 流程会自动回退到 `Restart()`。

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
- 支持 Hysteria2 / VLESS / Reality / Trojan 四种协议
- 支持 TLS 自签名证书、ACME 自动签发、自定义证书
- 支持 Hysteria2 混淆（obfs）和带宽限制
- 支持 Reality 协议的密钥对和握手配置
- 原子写入配置文件（先写 `.tmp` 再 `rename`）
- 管理多用户部署记录
- **多用户合并**：同一协议+端口的多个用户自动合并到同一个 inbound 的 users 列表中，支持多次 Deploy 追加用户而不覆盖

#### DeployRequest 结构

```go
type DeployRequest struct {
    UserID   string `json:"user_id"`              // 用户 ID（必填）
    NodeID   string `json:"node_id"`              // 节点 ID（必填）
    Protocol string `json:"protocol"`             // 协议：hysteria2 / vless / reality / trojan（必填）
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

1. **记录用户部署信息**：将 DeployRequest 存入 `deploys` 映射表

2. **重建完整配置**：遍历所有已部署用户，按协议+端口分组，将同一协议+端口的用户合并到同一个 inbound 的 `users` 列表中

3. **确定 TLS 证书来源**：
   - Reality 协议：不需要 TLS 证书
   - 指定了 ACME 域名：使用 ACME 自动签发
   - 指定了证书路径：使用自定义证书
   - 均未指定：自动生成自签名证书（ECDSA P256，有效期 10 年）

4. **生成服务端入站配置（inbound）**：
   - Hysteria2：`type: "hysteria2"`，含 users（name + password）、TLS、obfs、带宽限制
   - VLESS：`type: "vless"`，含 users（name + UUID + flow）、TLS
   - Reality：`type: "vless"`，含 Reality TLS（private_key、short_id、handshake）
   - Trojan：`type: "trojan"`，含 users（name + password）、TLS，可选 WebSocket 传输

5. **生成客户端出站配置（outbound）**：
   - 根据协议生成对应的客户端出站配置
   - 自动从 Reality 私钥推导公钥
   - 自签名证书时标记 `insecure: true`

6. **生成客户端 URI**：
   - Hysteria2：`hysteria2://password@server:port?sni=xxx`
   - VLESS：`vless://uuid@server:port?security=tls&sni=xxx`
   - Reality：`vless://uuid@server:port?security=reality&pbk=xxx&sid=xxx`
   - Trojan：`trojan://password@server:port?security=tls&sni=xxx&type=tcp&fp=chrome`

> **多用户合并机制**：每次 Deploy 不会覆盖已有用户配置，而是将新用户追加到对应 inbound 的 users 列表中。例如先部署 user-001（hysteria2, port 443），再部署 user-002（hysteria2, port 443），最终生成的 inbound 会包含 `users: [{name: "user-001", password: "..."}, {name: "user-002", password: "..."}]`。RemoveDeploy 删除用户后也会重新构建配置。

#### 协议服务端配置映射

**Hysteria2 服务端入站**：

```json
{
  "type": "hysteria2",
  "tag": "hysteria2-in",
  "listen": "0.0.0.0",
  "listen_port": 443,
  "users": [{"name": "user-001", "password": "user_password"}],
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
  "users": [{"name": "user-001", "uuid": "user_uuid", "flow": "xtls-rprx-vision"}],
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
  "users": [{"name": "user-001", "uuid": "user_uuid", "flow": "xtls-rprx-vision"}],
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

**Trojan 服务端入站**：

```json
{
  "type": "trojan",
  "tag": "trojan-in",
  "listen": "0.0.0.0",
  "listen_port": 443,
  "users": [{"name": "user-001", "password": "user_password"}],
  "tls": {
    "enabled": true,
    "server_name": "example.com",
    "certificate_path": "/path/to/cert.pem",
    "key_path": "/path/to/key.pem"
  }
}
```

**Trojan + WebSocket 传输**（指定 `obfs_type` 时自动启用）：

```json
{
  "type": "trojan",
  "tag": "trojan-in",
  "listen": "0.0.0.0",
  "listen_port": 443,
  "users": [{"name": "user-001", "password": "user_password"}],
  "tls": {
    "enabled": true,
    "server_name": "example.com",
    "certificate_path": "/path/to/cert.pem",
    "key_path": "/path/to/key.pem"
  },
  "transport": {
    "type": "ws",
    "path": "/obfs_password",
    "headers": {}
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

#### 流量持久化

累计流量数据持久化到磁盘，Node Agent 重启后自动恢复：

**持久化机制**：

| 项目 | 说明 |
|------|------|
| 存储路径 | `/var/lib/node-agent/traffic.json` |
| 写入频率 | 每 30 秒自动保存 |
| 停止保存 | Node Agent 优雅停止时立即保存 |
| 加载时机 | `NewSingBoxStatsCollector` 初始化时从磁盘加载 |
| 写入方式 | 原子写入（先写 `.tmp` 再 `rename`），避免半写状态 |

**持久化数据格式**：

```json
{
  "upload": 1073741824,
  "download": 2147483648
}
```

**工作流程**：

1. 启动时：从 `traffic.json` 加载已保存的累计流量
2. 运行中：每 30 秒将当前 `totalTraffic` 保存到磁盘
3. 停止时：通过 `stopPersist` channel 触发最终保存

**注意**：持久化仅保存全局累计流量（`totalTraffic`），不保存按用户流量（V2Ray API 的流量数据由 sing-box 进程维护，重启后重置）。

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
| `user_count` | int | 在线用户数（从 Clash API `/connections` 获取，不可用时回退到 DeviceLimiter 计数） |
| `device_count` | int | 总设备数 |
| `active_sessions` | int | 活跃会话数 |
| `online_users` | array | 在线用户详情列表（v1.13.0 新增） |

**online_users 数组元素**：

| 字段 | 类型 | 说明 |
|------|------|------|
| `user_id` | string | 用户唯一标识（UUID） |
| `inbound` | string | 入站标签（如 `hysteria2-in`、`vless-in`） |
| `ip` | string | 用户来源 IP |
| `upload` | int64 | 该用户上传字节数 |
| `download` | int64 | 该用户下载字节数 |

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
- 请求路径：`{ControlPlane.URL}/api/v1/node/heartbeat`
- 认证方式：`X-Node-Token` + `X-Node-ID` Header
- 启动时立即发送一次心跳，之后定时发送
- 支持优雅停止（通过 `stopCh` channel）

#### 指数退避重试机制

心跳发送失败时自动重试，采用指数退避策略避免对控制面造成压力：

**重试策略**：

| 参数 | 值 | 说明 |
|------|------|------|
| 最大重试次数 | 3 | 每次心跳最多重试 3 次 |
| 退避算法 | 指数退避 | `2^(attempt-1)` 秒 |
| 最大退避时间 | 30 秒 | 单次退避不超过 30 秒 |
| 重试条件 | 5xx 错误 / 网络错误 | 4xx 错误不重试 |

**退避时间表**：

| 重试次数 | 等待时间 |
|----------|----------|
| 第 1 次 | 1 秒 |
| 第 2 次 | 2 秒 |
| 第 3 次 | 4 秒 |
| 最大 | 30 秒 |

**重试流程**：

```
发送心跳
  │
  ├── 成功 (200 OK) → 重置 consecutiveFails，结束
  │
  ├── 4xx 错误 → 不重试，记录日志，结束
  │
  ├── 5xx 错误 → 重试
  │
  └── 网络错误 → 重试
       │
       ├── 等待退避时间
       │   └── 收到 stopCh → 立即停止
       │
       └── 重试次数耗尽 → consecutiveFails++，记录日志
```

**注意**：重试期间若收到停止信号（`stopCh`），会立即退出重试循环，确保 Node Agent 可以快速优雅停止。

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

下发用户节点配置，生成服务端 sing-box config.json 并热更新（SIGHUP Reload，不断开用户连接），同时返回客户端连接配置。若 Reload 失败则自动回退到 Restart。

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
| `protocol` | string | 是 | 协议类型：`hysteria2` / `vless` / `reality` / `trojan` |
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

**在线用户检测机制（双源合并）**：

Node Agent 使用 Clash API 和 V2Ray API 双数据源检测在线用户，自动合并去重：

| 协议 | Clash API `inboundUser` | V2Ray API 用户流量 | 检测方式 |
|------|------------------------|-------------------|----------|
| VLESS / Trojan / Reality | ✅ 有 | ✅ 有 | Clash API 精确匹配（含 IP） |
| Hysteria2 | ❌ 无 | ✅ 有 | V2Ray API 流量增量检测 + Clash API 补充 IP/inbound |
| 混合部署 | 部分有 | 全部有 | 双源合并，按 user_id 去重 |

工作流程：
1. **Clash API**：通过 `/connections` 获取活跃连接，解析 `inboundUser` 字段识别在线用户（VLESS/Trojan 等协议）
2. **V2Ray API**：通过 `QueryStats` gRPC 获取每用户累计流量，对比上次快照检测流量增量判断活跃用户（Hysteria2 等无 inboundUser 的协议）
3. **合并**：将两个数据源的结果按 `user_id` 合并去重，V2Ray API 的用户从 Clash API 连接中补充 inbound 和 IP 信息
4. **降级**：任一数据源失败时，仍返回另一数据源的结果

> **注意**：`/stats` 和 `/status` 接口也会返回 `online_users`（在线用户数）、`online_details`（在线用户详情）和 `user_traffic`（V2Ray 按用户流量统计）字段。

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
    "active_sessions": 15,
    "online_users": [
      {
        "user_id": "fc977ae4-1234-5678-abcd-ef0123456789",
        "inbound": "hysteria2-in",
        "ip": "1.2.3.4",
        "upload": 102400,
        "download": 204800
      },
      {
        "user_id": "a1b2c3d4-5678-9abc-def0-123456789abc",
        "inbound": "vless-in",
        "ip": "5.6.7.8",
        "upload": 51200,
        "download": 102400
      }
    ]
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

### 5.12 用户移除接口

#### `POST /deploy/remove`

移除已部署的用户配置，从 sing-box 配置中删除该用户并热更新（SIGHUP Reload），不影响其他在线用户。

**请求体**：

```json
{
  "user_id": "123"
}
```

**字段说明**：

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `user_id` | string | 是 | 要移除的用户 ID |

**成功响应** (`200 OK`)：

```json
{
  "success": true,
  "message": "user 123 removed"
}
```

**工作流程**：

1. 加锁（`deployMu`），确保与 `/deploy` 操作互斥
2. 从 `deploys` 映射表中删除该用户的部署记录
3. 重新构建 sing-box 配置（其他用户不受影响）
4. 原子写入配置文件
5. 若 sing-box 运行中，发送 SIGHUP 热更新；若 Reload 失败则回退到 Restart

**错误响应**：

| 状态码 | 场景 |
|--------|------|
| 400 | 缺少 `user_id` 字段 |
| 500 | 移除失败（用户不存在）/ Reload 和 Restart 均失败 |

**原子性保证**：`/deploy` 和 `/deploy/remove` 共享同一把互斥锁（`deployMu`），确保并发请求不会导致配置竞态条件。

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
  "panel_type": "",
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
    "timeout": 10,
    "heartbeat_path": "/api/v1/node/heartbeat"
  },
  "xboard": {
    "api_host": "https://YOUR-XBOARD-DOMAIN",
    "api_key": "YOUR-XBOARD-NODE-TOKEN",
    "node_id": 1,
    "node_type": "hysteria2",
    "sync_interval": 60,
    "timeout": 30,
    "device_limit": 0,
    "node_secret": "",
    "rotation_interval": 0
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
| `panel_type` | string | `""` | 面板类型，设为 `"xboard"` 启用 Xboard 模式，留空使用 Laravel 模式 |

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
| `heartbeat_path` | string | `/api/v1/node/heartbeat` | 心跳上报路径，可按控制面实际路由修改 |

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

#### xboard 配置（Xboard 面板模式）

当 `panel_type` 设为 `"xboard"` 时，Node Agent 从 Xboard 面板拉取配置和用户，不再使用 Laravel 控制面的推送模式。

| 字段 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `api_host` | string | `""` | Xboard 面板地址（如 `https://panel.example.com`） |
| `api_key` | string | `""` | Xboard 节点通信 Token（在 Xboard 后台节点设置中获取） |
| `node_id` | int | `0` | Xboard 中的节点 ID |
| `node_type` | string | `""` | 协议类型：`hysteria2` / `vless` / `trojan` / `reality` |
| `sync_interval` | int | `60` | 同步间隔（秒），最小 10 秒 |
| `timeout` | int | `30` | API 请求超时（秒） |
| `device_limit` | int | `0` | 每用户最大设备数（0 = 使用 Xboard 面板配置） |
| `node_secret` | string | `""` | 动态密码密钥（32 字节随机字符串，空 = 禁用动态密码） |
| `rotation_interval` | int | `0` | 密码轮换间隔（秒），0 = 禁用动态密码，推荐 3600 |

> **模式切换**：`panel_type` 为空或非 `xboard` 时，使用 Laravel 模式（心跳上报 + 面板推送配置）；设为 `xboard` 时，使用 Xboard 模式（定时拉取配置/用户 + 上报流量）。两种模式互斥，不可同时使用。

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
    "timeout": 10,
    "heartbeat_path": "/api/v1/node/heartbeat"
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

**Trojan URI 格式**：

```
trojan://password@server:port?security=tls&sni=xxx&type=tcp&fp=chrome#trojan-node-id
```

**Trojan + WebSocket URI 格式**（指定 `obfs_type` 时）：

```
trojan://password@server:port?security=tls&sni=xxx&type=ws&path=/obfs_password#trojan-node-id
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

#### Trojan

| 参数 | 说明 | 默认值 |
|------|------|--------|
| `password` | 认证密码 | 必填 |
| `sni` | TLS SNI | 使用 server 值 |
| `insecure` | 允许不安全 TLS（自签名证书时为 `true`） | `false` |
| `obfs_type` | 传输层混淆（指定时启用 WebSocket 传输） | 无 |
| `obfs_password` | WebSocket 路径密码 | 无 |

**Trojan 传输模式**：

- **直连模式**（默认）：`type=tcp`，标准 Trojan + TLS 连接
- **WebSocket 模式**：指定 `obfs_type` 后自动启用，`type=ws`，通过 WebSocket 传输，路径为 `/{obfs_password}`，适用于需要穿越 HTTP 代理或 CDN 的场景

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
POST /api/v1/node/heartbeat
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

### 11.4 对接 Xboard 面板

Node Agent 支持对接 Xboard 面板，使用 UniProxy API 协议进行通信。与 Laravel 模式（面板推送配置到节点）不同，Xboard 模式采用**节点主动拉取**的方式。

#### 工作模式对比

| 特性 | Laravel 模式 | Xboard 模式 |
|------|-------------|-------------|
| 配置下发 | 面板推送（POST /deploy） | 节点拉取（GET /config） |
| 用户管理 | 面板逐个推送 | 节点批量拉取用户列表 |
| 流量上报 | 心跳附带流量数据 | 主动推送流量到面板 |
| 状态上报 | 心跳定时上报 | 无心跳，通过同步日志查看 |
| 触发方式 | 面板主动调用 | 节点定时轮询 |

#### Xboard UniProxy API

Node Agent 实现了 Xboard 的核心 UniProxy API：

| API | 方法 | 说明 |
|-----|------|------|
| `/api/v1/server/UniProxy/config` | GET | 获取节点配置（地址、端口、TLS、Reality 等），支持 ETag 缓存 |
| `/api/v1/server/UniProxy/user` | GET | 获取用户列表（UUID、速度限制、设备限制），支持 ETag 缓存 |
| `/api/v1/server/UniProxy/push` | POST | 上报用户增量流量数据 |
| `/api/v1/server/UniProxy/alive` | POST | 上报在线设备状态 |
| `/api/v1/server/UniProxy/status` | POST | 上报节点系统状态（CPU、内存、磁盘） |
| `/api/v1/server/UniProxy/alivelist` | GET | 获取在线设备列表 |

#### 同步流程

```
┌─────────────┐     定时拉取      ┌─────────────┐
│  Xboard 面板 │ ◄────────────── │  Node Agent  │
│  (UniProxy) │                  │   (节点端)    │
│             │ ──────────────► │             │
└─────────────┘   返回配置/用户   └──────┬──────┘
                                       │
                                       │ 生成配置 + 重载
                                       ▼
                                 ┌─────────────┐
                                 │  sing-box    │
                                 └──────┬──────┘
                                        │ 采集流量
                                        ▼
                                 ┌─────────────┐
                                 │ v2ray-stats  │
                                 └──────┬──────┘
                                        │ 上报流量
                                        ▼
                                 ┌─────────────┐
                                 │  Xboard 面板 │
                                 └─────────────┘
```

1. **拉取节点配置**：获取节点服务器配置（地址、端口、TLS、Reality 密钥等），支持 ETag 缓存减少带宽
2. **拉取用户列表**：获取允许连接的用户（UUID、速度限制、设备限制），支持 ETag 缓存
3. **生成 sing-box 配置**：根据节点配置和用户列表自动生成完整的服务端配置
4. **重载/启动 sing-box**：配置或用户变更后自动重载（SIGHUP），首次自动启动
5. **上报增量流量**：通过 V2Ray API 采集按用户流量，计算增量后推送到 Xboard
6. **上报节点状态**：定时上报 CPU、内存、磁盘等系统信息到 Xboard

#### 动态密码（可选）

启用后，用户密码按时间窗口自动轮换，密码泄露仅影响当前窗口，窗口结束后自动失效。

**算法**：`HMAC-SHA256(user_uuid, node_secret:time_window)` → Base64 → 密码

**过渡期机制**：密码轮换时，sing-box 同时接受当前窗口和前一窗口的密码，确保客户端无感知切换。

**配置方式**：

```json
{
  "xboard": {
    "node_secret": "your-random-32-byte-secret",
    "rotation_interval": 3600
  }
}
```

| 参数 | 说明 |
|------|------|
| `node_secret` | 动态密码密钥，Panel 和 Node Agent 必须一致 |
| `rotation_interval` | 轮换间隔（秒），0 = 禁用，推荐 3600（1小时） |

**Panel 端要求**：
- `/config` 接口需返回 `node_secret` 和 `rotation_interval`
- `/user` 接口需返回 `dynamic_password` 和 `password_expires_at`
- 订阅接口需使用动态密码替换原始 UUID

**客户端要求**：
- 解析订阅中的密码过期时间
- 过期前 5 分钟自动调用刷新接口获取新密码
- 连接失败时立即刷新订阅

#### 配置方式

**方式一：安装脚本配置**

运行安装脚本时选择面板类型为 Xboard：

```bash
bash <(curl -fsSL https://raw.githubusercontent.com/changwangyun/node-agent/main/deploy/install.sh)
# 选择 "2) Xboard 面板"
# 输入面板地址、节点 Token、节点 ID、协议类型
```

**方式二：手动修改配置文件**

编辑 `/etc/node-agent/config.json`：

```json
{
  "panel_type": "xboard",
  "xboard": {
    "api_host": "https://your-xboard-domain.com",
    "api_key": "your-node-token",
    "node_id": 1,
    "node_type": "hysteria2",
    "sync_interval": 60,
    "timeout": 30,
    "device_limit": 0,
    "node_secret": "",
    "rotation_interval": 0
  }
}
```

修改后重启服务：`systemctl restart node-agent`

#### Xboard 面板端设置

1. 在 Xboard 后台 **添加节点**，记录节点 ID
2. 选择协议类型（hysteria2 / vless / trojan / reality）
3. 配置节点地址、端口、TLS 等参数
4. 获取节点的通信 Token（`api_key`）
5. 将以上信息填入 Node Agent 配置文件

#### 支持的协议

| 协议 | Xboard node_type | 说明 |
|------|-----------------|------|
| Hysteria2 | `hysteria2` | 支持 TLS、ACME、自签名证书、混淆、带宽限制 |
| VLESS | `vless` | 支持 TLS + ACME/自签名证书 |
| Reality | `reality` | 支持 Reality 密钥对、Short ID、握手配置 |
| Trojan | `trojan` | 支持 TLS + ACME/自签名证书、WebSocket 传输 |

#### 核心模块

| 文件 | 说明 |
|------|------|
| [core/xboard/client.go](file:///Volumes/koeyx/box/node-agent/core/xboard/client.go) | UniProxy API 客户端，封装 HTTP 请求和响应解析 |
| [core/xboard/sync.go](file:///Volumes/koeyx/box/node-agent/core/xboard/sync.go) | 同步逻辑，定时拉取配置/用户、生成 sing-box 配置、上报流量 |

#### 日志

Xboard 模式的日志前缀为 `[xboard]`，可通过以下命令查看：

```bash
journalctl -u node-agent -f | grep xboard
```

常见日志：

```
[xboard] starting sync, interval=60s
[xboard] config updated: 10 users deployed
[xboard] failed to get node info: server returned 401: ...
[xboard] failed to report traffic: ...
```

### 11.5 多节点负载均衡

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

## 12. Laravel 控制面板开发指南

本章节为 Laravel 控制面板的开发提供详细建议，涵盖数据库设计、核心服务、心跳监控、流量计费和定时任务。完整的 AI 编程提示词见 `docs/AI_PROMPTS.md`。

### 12.1 数据库设计

#### 核心表结构

| 表名 | 用途 | 关键字段 |
|------|------|----------|
| `users` | 用户账户 | `uuid`(用户唯一标识), `status`, `traffic_used/limit`, `speed_limit`, `device_limit` |
| `user_devices` | 设备绑定 | `user_id`, `node_id`, `device_id`, `ip`, `inbound`, `platform`, `last_online_at` |
| `nodes` | 节点信息 | `code`(节点编码), `api_url`, `api_token`, `password_secret`, `status`, `current_users` |
| `node_protocols` | 节点协议配置 | `node_id`, `protocol`, `port`, `config`(JSON), `is_default` |
| `plans` | 套餐定义 | `price_monthly/quarterly/yearly`, `traffic_limit`, `speed_limit`, `device_limit` |
| `subscriptions` | 用户订阅 | `user_id`, `plan_id`, `status`, `traffic_used/limit`, `expired_at` |
| `traffic_logs` | 流量日志 | `user_id`, `node_id`, `upload`, `download`, `date` |
| `node_heartbeats` | 心跳记录 | `node_id`, `cpu_percent`, `mem_percent`, `online_users`, `singbox_running` |

#### node_protocols.config JSON 格式

```json
// Hysteria2
{"sni":"example.com","obfs_type":"salamander","obfs_password":"xxx","up_mbps":100,"down_mbps":200}

// VLESS + Reality
{"sni":"www.microsoft.com","reality_public_key":"xxx","reality_short_id":"xxx","flow":"xtls-rprx-vision"}

// VLESS + TLS
{"sni":"example.com","acme_domain":"example.com","acme_email":"admin@example.com"}

// Trojan + TLS
{"sni":"example.com"}

// Trojan + WebSocket
{"sni":"example.com","obfs_type":"ws","obfs_password":"xxx"}
```

### 12.2 核心服务实现

#### NodeAgentClient — 与 Node Agent 通信

```php
class NodeAgentClient
{
    public function deploy(Node $node, array $payload): array
    {
        return $this->post($node, '/deploy', $payload);
    }

    public function status(Node $node): array
    {
        return $this->get($node, '/status');
    }

    public function removeUser(Node $node, string $userId): array
    {
        return $this->post($node, '/deploy/remove', ['user_id' => $userId]);
    }

    public function online(Node $node): array
    {
        return $this->get($node, '/online');
    }

    public function restart(Node $node): array
    {
        return $this->post($node, '/restart');
    }

    public function registerDevice(Node $node, string $userId, string $deviceId, string $ip): array
    {
        return $this->post($node, '/device/register', [
            'user_id' => $userId,
            'device_id' => $deviceId,
            'ip' => $ip,
        ]);
    }

    public function userTraffic(Node $node, ?string $userId = null): array
    {
        $path = '/traffic/user';
        if ($userId) {
            $path .= '?user_id=' . $userId;
        }
        return $this->get($node, $path);
    }

    private function get(Node $node, string $path): array
    {
        return Http::withHeaders(['X-Node-Token' => $node->api_token])
            ->get(rtrim($node->api_url, '/') . $path)
            ->json();
    }

    private function post(Node $node, string $path, array $data = []): array
    {
        return Http::withHeaders(['X-Node-Token' => $node->api_token])
            ->post(rtrim($node->api_url, '/') . $path, $data)
            ->json();
    }
}
```

#### SyncService — 用户配置同步

```php
class SyncService
{
    public function __construct(private NodeAgentClient $client) {}

    public function syncUserToNode(User $user, Node $node, string $protocolType = null): array
    {
        $protocol = $this->resolveProtocol($node, $protocolType);
        $password = $this->generatePassword($user, $node);

        $payload = [
            'user_id' => (string) $user->id,
            'node_id' => $node->code,
            'protocol' => $protocol->protocol,
            'server' => '0.0.0.0',
            'port' => $protocol->port,
            'sni' => $protocol->config['sni'] ?? $node->server,
        ];

        if ($protocol->protocol === 'hysteria2') {
            $payload['password'] = $password;
            if (isset($protocol->config['obfs_type'])) {
                $payload['obfs_type'] = $protocol->config['obfs_type'];
                $payload['obfs_password'] = $protocol->config['obfs_password'] ?? '';
            }
            if ($user->speed_limit > 0) {
                $payload['up_mbps'] = $user->speed_limit;
                $payload['down_mbps'] = $user->speed_limit;
            }
        }

        if ($protocol->protocol === 'vless') {
            $payload['uuid'] = $user->uuid;
        }

        if ($protocol->protocol === 'reality') {
            $payload['uuid'] = $user->uuid;
            $payload['reality_private_key'] = $protocol->config['reality_private_key'];
            $payload['reality_public_key'] = $protocol->config['reality_public_key'];
            $payload['reality_short_id'] = $protocol->config['reality_short_id'];
            $payload['reality_dest'] = $protocol->config['reality_dest'] ?? 'www.microsoft.com';
            $payload['reality_dest_port'] = $protocol->config['reality_dest_port'] ?? 443;
        }

        if ($protocol->protocol === 'trojan') {
            $payload['password'] = $password;
            if (isset($protocol->config['obfs_type'])) {
                $payload['obfs_type'] = $protocol->config['obfs_type'];
                $payload['obfs_password'] = $protocol->config['obfs_password'] ?? '';
            }
        }

        return $this->client->deploy($node, $payload);
    }

    private function generatePassword(User $user, Node $node): string
    {
        $timestamp = time();
        $message = $user->uuid . '.' . $timestamp;
        $signature = hash_hmac('sha256', $message, $node->password_secret);
        return base64_encode($message . '.' . $signature);
    }

    private function resolveProtocol(Node $node, ?string $protocolType): NodeProtocol
    {
        if ($protocolType) {
            return $node->protocols()->where('protocol', $protocolType)->first()
                ?? $node->protocols()->where('is_default', true)->firstOrFail();
        }
        return $node->protocols()->where('is_default', true)->firstOrFail();
    }
}
```

### 12.3 心跳接收与节点监控

Node Agent 每 60 秒主动向 Laravel 上报心跳，Laravel 需实现接收端点：

```php
// routes/api.php
Route::post('/v1/node/heartbeat', [NodeHeartbeatController::class, 'receive']);

class NodeHeartbeatController extends Controller
{
    public function receive(Request $request)
    {
        $nodeId = $request->header('X-Node-ID');
        $token = $request->header('X-Node-Token');

        $node = Node::where('code', $nodeId)->firstOrFail();

        if ($token !== $node->api_token) {
            return response()->json(['error' => 'unauthorized'], 401);
        }

        $payload = $request->all();

        $node->update([
            'status' => $payload['status']['singbox_running'] ? 'online' : 'offline',
            'last_heartbeat_at' => now(),
            'current_users' => $payload['online']['user_count'] ?? 0,
        ]);

        NodeHeartbeat::create([
            'node_id' => $node->id,
            'cpu_percent' => $payload['system']['cpu_percent'] ?? 0,
            'mem_percent' => $payload['system']['mem_percent'] ?? 0,
            'active_connections' => $payload['online']['active_sessions'] ?? 0,
            'online_users' => $payload['online']['user_count'] ?? 0,
            'upload_speed' => $payload['traffic']['upload'] ?? 0,
            'download_speed' => $payload['traffic']['download'] ?? 0,
            'singbox_running' => $payload['status']['singbox_running'] ?? false,
        ]);

        // 处理在线用户详情
        if (!empty($payload['online']['online_users'])) {
            foreach ($payload['online']['online_users'] as $onlineUser) {
                UserDevice::updateOrCreate(
                    [
                        'user_id'  => $onlineUser['user_id'],
                        'node_id'  => $node->id,
                    ],
                    [
                        'ip'            => $onlineUser['ip'] ?? '',
                        'inbound'       => $onlineUser['inbound'] ?? '',
                        'last_online_at' => now(),
                    ]
                );
            }
        }

        return response()->json(['success' => true]);
    }
}
```

**⚠️ 心跳路径注意**：Node Agent 默认心跳路径为 `/api/v1/node/heartbeat`，可在 Node Agent 配置文件中通过 `heartbeat_path` 修改。确保 Laravel 路由与 Node Agent 配置一致。

### 12.4 用户流量采集与计费

#### 采集方式

推荐两种流量采集方式，可组合使用：

**方式一：主动拉取（推荐）**

Laravel 定时任务每 5 分钟从各节点拉取流量数据：

```php
class CollectNodeStats implements ShouldQueue
{
    public function handle(NodeAgentClient $client)
    {
        $nodes = Node::where('status', 'online')->get();

        foreach ($nodes as $node) {
            try {
                $stats = $client->userTraffic($node);

                foreach ($stats['users'] ?? [] as $userData) {
                    $user = User::where('uuid', $userData['user_id'])->first();
                    if (!$user) continue;

                    TrafficLog::updateOrCreate(
                        [
                            'user_id' => $user->id,
                            'node_id' => $node->id,
                            'date' => now()->toDateString(),
                        ],
                        [
                            'upload' => $userData['upload'],
                            'download' => $userData['download'],
                        ]
                    );
                }
            } catch (\Throwable $e) {
                Log::warning("Failed to collect stats from node {$node->code}: " . $e->getMessage());
            }
        }
    }
}
```

**方式二：客户端上报**

客户端每 30 秒调用 `POST /traffic/report` 上报流量，Laravel 记录并检查限额：

```php
class TrafficController extends Controller
{
    public function report(Request $request)
    {
        $request->validate([
            'node_id' => 'required|string',
            'upload' => 'required|integer',
            'download' => 'required|integer',
        ]);

        $user = $request->user();
        $node = Node::where('code', $request->node_id)->firstOrFail();

        TrafficLog::updateOrCreate(
            [
                'user_id' => $user->id,
                'node_id' => $node->id,
                'date' => now()->toDateString(),
            ],
            [
                'upload' => \DB::raw('upload + ' . $request->upload),
                'download' => \DB::raw('download + ' . $request->download),
            ]
        );

        $totalUsed = $user->traffic_used + $request->upload + $request->download;
        $isLimited = $user->traffic_limit > 0 && $totalUsed >= $user->traffic_limit;

        if ($isLimited) {
            $user->update(['status' => 'limited']);
        }

        return response()->json([
            'traffic_used' => $totalUsed,
            'traffic_limit' => $user->traffic_limit,
            'is_limited' => $isLimited,
        ]);
    }
}
```

#### 流量限额处理

```php
class TrafficLimitService
{
    public function __construct(private NodeAgentClient $client) {}

    public function checkAndEnforce(User $user): void
    {
        if ($user->traffic_limit <= 0) return;
        if ($user->traffic_used < $user->traffic_limit) return;

        $user->update(['status' => 'limited']);

        // 从所有节点移除用户配置
        foreach ($user->activeNodes as $node) {
            try {
                $this->client->stop($node);
            } catch (\Throwable $e) {
                Log::warning("Failed to stop node {$node->code}: " . $e->getMessage());
            }
        }

        // 推送通知
        if ($user->fcm_token) {
            $this->pushNotification($user->fcm_token, '流量已用尽', '您的流量已达上限，请升级套餐');
        }
    }
}
```

### 12.5 Deploy 签名密码机制

Hysteria2 协议使用签名密码，由 Laravel 生成后传给客户端：

```
签名密码 = base64(uuid.timestamp.hmac_sha256_signature)
```

- `uuid`：用户唯一标识（users 表的 uuid 字段）
- `timestamp`：当前 Unix 时间戳
- `hmac_sha256_signature`：使用节点的 `password_secret` 对 `uuid.timestamp` 进行 HMAC-SHA256 签名
- 密码有效期 24 小时（timestamp 过期后 Node Agent 仍接受连接，但 Laravel 可在续签时更新）

**为什么用签名密码**：

1. 每个节点有独立的 `password_secret`，密码不可跨节点使用
2. 密码包含时间戳，可设置有效期
3. 无需在 Node Agent 和 Laravel 之间同步密码数据库
4. Node Agent 的 sing-box 配置中直接使用此密码作为 Hysteria2 的 `password` 字段

### 12.6 定时任务

```php
// app/Console/Kernel.php

protected function schedule(Schedule $schedule)
{
    // 每5分钟: 从节点采集用户流量
    $schedule->command('nodes:collect-stats')->everyFiveMinutes();

    // 每分钟: 检查过期订阅
    $schedule->command('subscriptions:check-expired')->everyMinute();

    // 每天0点: 重置月流量（按订阅周期）
    $schedule->command('traffic:reset-monthly')->dailyAt('00:00');

    // 每5分钟: 检查节点健康（心跳超时标记离线）
    $schedule->command('nodes:check-health')->everyFiveMinutes();

    // 每10分钟: 检查流量限额
    $schedule->command('traffic:check-limits')->everyTenMinutes();
}
```

**节点健康检查命令**：

```php
class CheckNodeHealth
{
    public function handle()
    {
        $threshold = now()->subMinutes(5);

        Node::where('status', 'online')
            ->where('last_heartbeat_at', '<', $threshold)
            ->each(function ($node) {
                $node->update(['status' => 'offline']);
                Log::warning("Node {$node->code} marked offline (heartbeat timeout)");
            });
    }
}
```

---

## 13. 客户端开发指南

本章节为 iOS / Android 客户端开发提供架构建议和实现要点。完整的 AI 编程提示词见 `docs/AI_PROMPTS.md`。

### 13.1 整体架构

```
┌─────────────────────────────────────────────┐
│                  客户端 App                   │
├──────────────┬──────────────┬───────────────┤
│   UI 层      │  业务逻辑层   │   服务层       │
│  SwiftUI /   │  ViewModel   │  VPNManager   │
│  Compose     │              │  ConfigBuilder│
│              │              │  TrafficMonitor│
├──────────────┴──────────────┴───────────────┤
│           NetworkExtension / VpnService      │
│              sing-box Mobile Library         │
└─────────────────────────────────────────────┘
         │ HTTP                    │ sing-box
         ▼                        ▼
   Laravel API              Node Agent
```

#### 目录结构（iOS）

```
VPNApp/
├── Views/           LoginView, HomeView, NodesView, ProfileView
├── ViewModels/      AuthVM, HomeVM, NodesVM, ProfileVM
├── Services/        APIService, VPNManager, ConfigBuilder, TrafficMonitor
├── Models/          User, Node, Subscription, TrafficSummary
└── PacketTunnel/    PacketTunnelProvider (NetworkExtension Target)
```

#### 目录结构（Android）

```
app/
├── ui/screen/       LoginScreen, HomeScreen, NodesScreen, ProfileScreen
├── ui/viewmodel/    AuthVM, HomeVM, NodesVM, ProfileVM
├── data/api/        ApiService (Retrofit)
├── data/repository/ AuthRepo, NodeRepo, TrafficRepo
├── service/         VpnService, SingboxManager, TrafficMonitor
└── util/            ConfigBuilder
```

### 13.2 连接流程

```
1. 用户选择节点 → 点击连接按钮
2. 调用 Laravel API: POST /connection/connect { node_id, protocol_type }
3. Laravel 调用 Node Agent: POST /deploy { 协议配置 }
4. Laravel 返回: { node参数, 签名密码, 配置参数 }
5. 客户端 ConfigBuilder 本地组装 sing-box 配置
6. 启动 VPN 隧道 (NetworkExtension / VpnService)
7. sing-box Mobile Library 建立代理连接
8. TrafficMonitor 每 30 秒上报流量
9. 收到 is_limited=true → 自动断开
```

### 13.3 sing-box 客户端配置组装

客户端从 Laravel API 获取节点参数后，本地组装完整 sing-box 配置：

```json
{
  "log": { "level": "warn" },
  "dns": {
    "servers": [
      { "tag": "google", "type": "tls", "server": "8.8.8.8" },
      { "tag": "local", "type": "udp", "server": "223.5.5.5" }
    ]
  },
  "inbounds": [
    {
      "type": "tun",
      "tag": "tun-in",
      "address": ["172.19.0.1/30", "fdfe:dcba:9876::1/126"],
      "mtu": 4160,
      "auto_route": true,
      "strict_route": true
    }
  ],
  "outbounds": [
    { "...协议outbound..." },
    { "type": "direct", "tag": "direct" },
    { "type": "block", "tag": "block" },
    { "type": "dns", "tag": "dns-out" }
  ],
  "route": {
    "rules": [
      { "action": "sniff" },
      { "protocol": ["dns"], "action": "hijack-dns" }
    ],
    "default_domain_resolver": "google",
    "final": "proxy"
  }
}
```

#### 各协议 Outbound 模板

**Hysteria2**：

```json
{
  "type": "hysteria2",
  "tag": "proxy",
  "server": "节点IP或域名",
  "server_port": 443,
  "password": "签名密码",
  "up_mbps": 100,
  "down_mbps": 200,
  "tls": {
    "enabled": true,
    "server_name": "SNI域名",
    "insecure": false,
    "alpn": ["h3"]
  },
  "obfs": {
    "type": "salamander",
    "password": "混淆密码"
  }
}
```

**VLESS + Reality**：

```json
{
  "type": "vless",
  "tag": "proxy",
  "server": "节点IP或域名",
  "server_port": 443,
  "uuid": "用户UUID",
  "flow": "xtls-rprx-vision",
  "tls": {
    "enabled": true,
    "server_name": "SNI域名",
    "utls": { "enabled": true, "fingerprint": "chrome" },
    "reality": {
      "enabled": true,
      "public_key": "Reality公钥",
      "short_id": "ShortID"
    }
  }
}
```

**VLESS + TLS**：

```json
{
  "type": "vless",
  "tag": "proxy",
  "server": "节点IP或域名",
  "server_port": 443,
  "uuid": "用户UUID",
  "tls": {
    "enabled": true,
    "server_name": "SNI域名"
  }
}
```

**Trojan + TLS**：

```json
{
  "type": "trojan",
  "tag": "proxy",
  "server": "节点IP或域名",
  "server_port": 443,
  "password": "用户密码",
  "tls": {
    "enabled": true,
    "server_name": "SNI域名",
    "utls": { "enabled": true, "fingerprint": "chrome" }
  }
}
```

**Trojan + WebSocket**：

```json
{
  "type": "trojan",
  "tag": "proxy",
  "server": "节点IP或域名",
  "server_port": 443,
  "password": "用户密码",
  "tls": {
    "enabled": true,
    "server_name": "SNI域名"
  },
  "transport": {
    "type": "ws",
    "path": "/obfs_password"
  }
}
```

### 13.4 Hysteria2 性能优化配置

Hysteria2 是基于 QUIC 的协议，客户端配置对性能影响很大。以下是关键优化点：

#### TUN MTU 设置

| 场景 | 推荐 MTU | 说明 |
|------|----------|------|
| Hysteria2 | **4160** | QUIC 在 UDP 之上，MTU 过大会导致 IP 分片，严重影响性能 |
| VLESS/TCP | 9000 | TCP 协议可使用较大 MTU |

**⚠️ 关键说明**：Hysteria2 的 QUIC 包在 UDP 之上传输，如果 TUN MTU 设为 9000，每个 QUIC 包会被 IP 层分片成多个小包。一旦丢失任何一个分片，整个 QUIC 包都要重传，性能急剧下降。4160 是经过测试的平衡值。

#### ALPN 协商

Hysteria2 客户端必须设置 `alpn: ["h3"]`，否则 TLS 握手时无法正确协商 HTTP/3 协议，导致连接失败或降级。

#### Brutal 拥塞控制

Hysteria2 的核心优势是 Brutal 拥塞控制算法，比标准 BBR 更激进地利用带宽。启用条件：

- 客户端 outbound 设置 `up_mbps` 和 `down_mbps`
- 服务端 inbound 设置 `ignore_client_bandwidth: true`（Node Agent 已自动处理）

```json
{
  "type": "hysteria2",
  "tag": "proxy",
  "server": "...",
  "server_port": 443,
  "password": "...",
  "up_mbps": 100,
  "down_mbps": 200,
  "tls": {
    "enabled": true,
    "server_name": "...",
    "alpn": ["h3"]
  }
}
```

`up_mbps` / `down_mbps` 应根据用户套餐的 `speed_limit` 设置。如果用户没有速度限制，建议设为客户端实际带宽的 80%。

#### 服务端系统优化

Node Agent 安装脚本已自动优化以下系统参数，无需手动配置：

| 参数 | 优化值 | 效果 |
|------|--------|------|
| `net.core.rmem_max` | 16MB | UDP 接收缓冲区，Hysteria2 性能瓶颈 |
| `net.core.wmem_max` | 16MB | UDP 发送缓冲区 |
| `net.ipv4.tcp_congestion_control` | bbr | TCP 层面加速 |
| `net.ipv4.tcp_fastopen` | 3 | 减少 TCP 握手延迟 |
| systemd CPU 调度 | `rr:99` | 实时调度，减少延迟抖动 |

### 13.5 流量监控与上报

#### 客户端流量上报

```swift
// iOS: TrafficMonitor
class TrafficMonitor {
    private var timer: Timer?
    private var lastUpload: Int64 = 0
    private var lastDownload: Int64 = 0

    func startMonitoring() {
        timer = Timer.scheduledTimer(withTimeInterval: 30, repeats: true) { [weak self] _ in
            self?.reportTraffic()
        }
    }

    func reportTraffic() {
        let currentUpload = getCurrentUpload()
        let currentDownload = getCurrentDownload()
        let deltaUpload = currentUpload - lastUpload
        let deltaDownload = currentDownload - lastDownload
        lastUpload = currentUpload
        lastDownload = currentDownload

        APIService.shared.reportTraffic(
            nodeId: currentNodeId,
            upload: deltaUpload,
            download: deltaDownload
        ) { result in
            if case .success(let response) = result, response.is_limited {
                VPNManager.shared.disconnect()
            }
        }
    }
}
```

#### 实时网速显示

从 sing-box Clash API 读取实时速度（客户端连接到本地 Clash API）：

```swift
func fetchRealtimeSpeed(completion: @escaping (Int64, Int64) -> Void) {
    guard let url = URL(string: "http://127.0.0.1:9090/traffic") else { return }

    URLSession.shared.dataTask(with: url) { data, _, _ in
        guard let data = data,
              let json = try? JSONSerialization.jsonObject(with: data) as? [String: Any],
              let up = json["up"] as? Int64,
              let down = json["down"] as? Int64 else { return }
        completion(up, down)
    }.resume()
}
```

### 13.6 iOS 客户端实现要点

#### sing-box Mobile Library 集成

```swift
// PacketTunnelProvider.swift
import NetworkExtension
import SingBoxLibrary

class PacketTunnelProvider: NEPacketTunnelProvider {
    private var singbox: SingBox?

    override func startTunnel(options: [String: NSObject]?) async throws {
        guard let config = options?["config"] as? String else {
            throw NEPacketTunnelProviderError.invalidConfiguration
        }

        let fd = self.packetFlow.value(forKey: "socket") as! Int32

        singbox = SingBox()
        try singbox?.start(configPath: writeConfigToFile(config), tunFd: fd)

        // 启动流量监控
        startTrafficMonitor()
    }

    override func stopTunnel(with reason: NEProviderStopReason) async {
        singbox?.stop()
        singbox = nil
    }

    private func writeConfigToFile(_ config: String) -> String {
        let path = NSTemporaryDirectory() + "sing-box-config.json"
        try? config.write(toFile: path, atomically: true, encoding: .utf8)
        return path
    }
}
```

#### VPNManager — 连接管理

```swift
class VPNManager: ObservableObject {
    static let shared = VPNManager()
    @Published var status: NEVPNStatus = .disconnected

    func connect(node: Node, protocol: NodeProtocol, password: String) {
        let config = ConfigBuilder.shared.buildConfig(
            node: node,
            protocol: `protocol`,
            password: password
        )

        let options: [String: NSObject] = ["config": config as NSObject]

        let vpnManager = NEVPNManager.shared()
        let proto = NETunnelProviderProtocol()
        proto.providerBundleIdentifier = "com.yourapp.PacketTunnel"
        proto.providerConfiguration = options
        proto.serverAddress = node.server
        vpnManager.protocolConfiguration = proto
        vpnManager.isEnabled = true
        vpnManager.saveToPreferences { error in
            if error == nil {
                try? vpnManager.connection.startVPNTunnel(options: options)
            }
        }
    }

    func disconnect() {
        NEVPNManager.shared().connection.stopVPNTunnel()
    }
}
```

#### 注意事项

1. **NetworkExtension 权限**：需要在 Xcode 中开启 Network Extension 能力，添加 `com.apple.developer.networking.networkextension` 权限
2. **sing-box 编译**：iOS 需要从源码编译 sing-box Mobile Library（XCFramework），参考 [sing-box 官方文档](https://sing-box.sagernet.org/installation/package/ios/)
3. **TUN MTU**：iOS 的 NetworkExtension 对 MTU 有限制，建议设为 4160（Hysteria2）或 9000（VLESS）
4. **后台保活**：iOS VPN 连接后系统自动保活，但 App 被杀后流量上报会中断，建议在 `PacketTunnelProvider` 中实现流量上报
5. **Keychain 存储**：JWT Token 和用户凭证必须存储在 Keychain，不要用 UserDefaults

### 13.7 Android 客户端实现要点

#### sing-box AAR Library 集成

```kotlin
class SingboxVpnService : VpnService() {
    private var singboxProcess: Long = 0

    override fun onStartCommand(intent: Intent?, flags: Int, startId: Int): Int {
        val config = intent?.getStringExtra("config") ?: return START_NOT_STICKY

        val fd = Builder()
            .addAddress("172.19.0.1", 30)
            .addRoute("0.0.0.0", 0)
            .addDnsServer("8.8.8.8")
            .setSession("VPN")
            .setMtu(4160)
            .establish()?.fd ?: return START_NOT_STICKY

        singboxProcess = Singbox.start(config, fd)
        return START_STICKY
    }

    override fun onDestroy() {
        Singbox.stop(singboxProcess)
        super.onDestroy()
    }
}
```

#### ConfigBuilder — 配置组装

```kotlin
object ConfigBuilder {

    fun buildHysteria2Config(
        server: String, port: Int, password: String,
        sni: String, upMbps: Int, downMbps: Int,
        obfsType: String? = null, obfsPassword: String? = null
    ): String {
        val outbound = mutableMapOf<String, Any>(
            "type" to "hysteria2",
            "tag" to "proxy",
            "server" to server,
            "server_port" to port,
            "password" to password,
            "up_mbps" to upMbps,
            "down_mbps" to downMbps,
            "tls" to mapOf(
                "enabled" to true,
                "server_name" to sni,
                "alpn" to listOf("h3")
            )
        )

        if (obfsType != null) {
            outbound["obfs"] = mapOf(
                "type" to obfsType,
                "password" to (obfsPassword ?: "")
            )
        }

        return buildFullConfig(outbound)
    }

    fun buildVlessRealityConfig(
        server: String, port: Int, uuid: String,
        sni: String, publicKey: String, shortId: String
    ): String {
        val outbound = mapOf<String, Any>(
            "type" to "vless",
            "tag" to "proxy",
            "server" to server,
            "server_port" to port,
            "uuid" to uuid,
            "flow" to "xtls-rprx-vision",
            "tls" to mapOf(
                "enabled" to true,
                "server_name" to sni,
                "utls" to mapOf("enabled" to true, "fingerprint" to "chrome"),
                "reality" to mapOf(
                    "enabled" to true,
                    "public_key" to publicKey,
                    "short_id" to shortId
                )
            )
        )

        return buildFullConfig(outbound)
    }

    fun buildTrojanConfig(
        server: String, port: Int, password: String,
        sni: String, obfsType: String? = null, obfsPassword: String? = null
    ): String {
        val outbound = mutableMapOf<String, Any>(
            "type" to "trojan",
            "tag" to "proxy",
            "server" to server,
            "server_port" to port,
            "password" to password,
            "tls" to mapOf(
                "enabled" to true,
                "server_name" to sni,
                "utls" to mapOf("enabled" to true, "fingerprint" to "chrome")
            )
        )

        if (obfsType != null) {
            outbound["transport"] = mapOf(
                "type" to "ws",
                "path" to "/${obfsPassword ?: ""}"
            )
        }

        return buildFullConfig(outbound)
    }

    private fun buildFullConfig(outbound: Map<String, Any>): String {
        val config = mapOf<String, Any>(
            "log" to mapOf("level" to "warn"),
            "dns" to mapOf(
                "servers" to listOf(
                    mapOf("tag" to "google", "type" to "tls", "server" to "8.8.8.8"),
                    mapOf("tag" to "local", "type" to "udp", "server" to "223.5.5.5")
                )
            ),
            "inbounds" to listOf(
                mapOf(
                    "type" to "tun",
                    "tag" to "tun-in",
                    "address" to listOf("172.19.0.1/30", "fdfe:dcba:9876::1/126"),
                    "mtu" to 4160,
                    "auto_route" to true,
                    "strict_route" to true
                )
            ),
            "outbounds" to listOf(outbound,
                mapOf("type" to "direct", "tag" to "direct"),
                mapOf("type" to "block", "tag" to "block"),
                mapOf("type" to "dns", "tag" to "dns-out")
            ),
            "route" to mapOf(
                "rules" to listOf(
                    mapOf("action" to "sniff"),
                    mapOf("protocol" to listOf("dns"), "action" to "hijack-dns")
                ),
                "default_domain_resolver" to "google",
                "final" to "proxy"
            )
        )

        return JSONObject(config).toString()
    }
}
```

#### 注意事项

1. **VpnService 权限**：AndroidManifest.xml 中声明 `<service android:name=".SingboxVpnService" android:permission="android.permission.BIND_VPN_SERVICE">`，并添加 `<intent-filter><action android:name="android.net.VpnService"/></intent-filter>`
2. **用户授权**：首次连接需调用 `VpnService.prepare(context)` 弹出系统授权对话框
3. **sing-box AAR**：从 [sing-box 官方](https://github.com/SagerNet/sing-box) 获取 AAR Library，放入 `app/libs/` 目录
4. **MTU 设置**：Android VpnService.Builder 的 `setMtu()` 对 Hysteria2 建议设为 4160
5. **前台服务**：Android 8.0+ 需要 Foreground Service 通知栏常驻，确保 VPN 不被系统杀死
6. **流量上报**：在 VpnService 中启动协程定时上报，App 被杀后 VpnService 仍可运行
7. **DataStore 存储**：JWT Token 使用 EncryptedDataStore 或 Jetpack Security 存储，不要用 SharedPreferences

---

## 14. 设计决策与权衡

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
| 配置热更新 | SIGHUP Reload 优先 | 不断开用户连接，毫秒级生效 | Windows 不支持，自动回退到 Restart |
| Deploy 原子性 | deployMu 互斥锁 | 防止并发 Deploy/Remove 竞态条件 | 增加少量锁等待延迟 |
| 心跳重试 | 指数退避（最多 3 次） | 应对控制面临时不可用 | 增加心跳延迟（最坏情况 +7s） |
| 流量持久化 | 每 30s 写入磁盘 | 重启后恢复累计流量 | 最多丢失 30s 数据 |
| Trojan 传输 | 可选 WebSocket | 支持 CDN 中转和穿越 HTTP 代理 | WS 传输性能低于直连 |
| 心跳在线用户 | Clash API 直取 | 不依赖面板调用 /device/register，直接从 sing-box 获取真实连接 | 需要启用 Clash API |
| 心跳用户详情 | 包含在心跳 payload | Laravel 面板无需额外调用 /online 接口即可获取在线设备 | 心跳 payload 略大 |

---

## 15. 版本变更记录

### v1.14.0 (2026-05-08)

**新功能**：

- **Xboard 面板对接**：新增 `core/xboard/` 模块，支持通过 UniProxy API 对接 Xboard 面板
  - `client.go`：UniProxy API 客户端，实现 `/config`、`/user`、`/push`、`/alive`、`/status`、`/alivelist` 六个接口
  - `sync.go`：定时同步逻辑，拉取节点配置和用户列表，自动生成 sing-box 配置并重载，上报增量流量和节点状态
- **ETag 缓存**：`/config` 和 `/user` 接口支持 ETag 缓存，减少不必要的带宽消耗
- **增量流量上报**：流量数据按增量方式上报（当前值 - 上次上报值），避免重复计算
- **节点状态上报**：定时上报 CPU、内存、Swap、磁盘使用情况到 Xboard `/status` 接口
- **配置变更检测**：区分节点配置变更和用户列表变更，仅在有变化时才重载 sing-box
- **双面板模式**：配置文件新增 `panel_type` 字段，支持 Laravel（推送模式）和 Xboard（拉取模式）两种面板类型
- **Xboard 配置块**：新增 `xboard` 配置项，包含 `api_host`、`api_key`、`node_id`、`node_type`、`sync_interval`、`timeout`、`device_limit`
- **安装脚本面板选择**：`install.sh` 安装时支持选择面板类型（Laravel / Xboard），Xboard 模式引导配置面板地址、Token、节点 ID、协议类型
- **完整协议参数**：`buildDeployRequest` 支持 Hysteria2/VLESS/Reality/Trojan/Shadowsocks 全协议参数构建，包括 TLS 证书/ACME、WebSocket/gRPC 传输层、Reality 密钥对等
- **Generator 批量用户方法**：`Generator` 新增 `DeployBatch()` 和 `RemoveAllDeploys()` 方法，支持批量用户部署和清除

**支持的协议**：

| 协议 | Xboard node_type | 特性 |
|------|-----------------|------|
| Hysteria2 | `hysteria2` | TLS/ACME/自签名/混淆/带宽 |
| VLESS | `vless` | TLS/ACME/自签名 |
| Reality | `reality` | 密钥对/Short ID/握手 |
| Trojan | `trojan` | TLS/ACME/自签名/WebSocket |

### v1.15.0 (2026-05-08)

**新功能 — 动态密码**：

- **HMAC-SHA256 时间窗口密码轮换**：密码按时间窗口自动轮换，泄露仅影响当前窗口
- **过渡期双密码**：密码轮换时 sing-box 同时接受当前窗口和前一窗口密码，客户端无感知切换
- **Panel 配置同步**：`node_secret` 和 `rotation_interval` 从 Panel `/config` 接口动态获取
- **本地计算过渡期密码**：Node Agent 本地 HMAC 计算前一窗口密码，无需额外回调 Panel，自主容错
- **配置变更检测扩展**：新增 `NodeSecret`、`RotationInterval` 变更检测
- **XboardConfig 新增字段**：`node_secret`（动态密码密钥）、`rotation_interval`（轮换间隔秒数）
- **UserInfo 新增字段**：`dynamic_password`（当前窗口密码）、`password_expires_at`（过期时间戳）
- **DeployRequest 新增字段**：`prev_password`、`prev_uuid`（过渡期凭证）
- **rebuildConfig 过渡期用户**：为每个启用动态密码的用户生成 `@prev` 后缀的过渡期 inbound user

### v1.13.0 (2026-05-06)

**新功能**：

- **心跳上报在线用户详情**：心跳 payload 的 `online` 字段新增 `online_users` 数组，包含每个在线用户的 `user_id`、`inbound`、`ip`、`upload`、`download`，Laravel 面板可直接从心跳获取在线设备信息
- **在线用户数从 Clash API 获取**：心跳的 `user_count` 不再依赖 DeviceLimiter 的 sessions（需要面板调用 `/device/register`），改为直接从 Clash API `/connections` 获取真实在线用户数

**修复**：

- **Hysteria2 密码为空**：当 Laravel 面板未传递 `password` 参数时，自动使用 `user_id` 作为 fallback 密码，避免认证失败
- **sing-box 版本兼容性**：`initial_packet_size` 字段仅在 sing-box >= 1.14.0 时生成（之前误判为 >= 1.10.0），避免旧版 sing-box 配置解析失败
- **版本未知时保守策略**：当 sing-box 版本为 "unknown" 时，`VersionAtLeast()` 和 `supportsInitialPacketSize()` 返回 `false`，不生成可能不兼容的配置字段
- **安装脚本自动安装 git**：编译 sing-box 前自动检测并安装 git（支持 apt/yum/apk）
- **sing-box 编译嵌入版本信息**：`go install` 添加 `-ldflags` 注入版本号，解决 `sing-box version unknown` 问题
- **Token 自动同步**：`control_plane.token` 为空时自动从 `api_token` 同步，减少手动配置

### v1.12.0 (2026-05-06)

**修复**：

- **心跳 401 错误**：区分 API Token（面板 → 节点）和 Control Plane Token（节点 → 面板），确保心跳使用正确的 Token
- **心跳 404 错误**：默认心跳路径改为 `/api/v1/node/heartbeat`

### v1.11.0 (2026-04-30)

**新功能**：

- **Trojan 协议支持**：新增 Trojan inbound/outbound/URI 生成，支持 TLS 直连和 WebSocket 传输两种模式
- **用户移除接口**：`POST /deploy/remove`，移除用户配置并热更新，不影响其他在线用户
- **配置热更新**：`Manager.Reload()` 方法，通过 SIGHUP 信号热更新 sing-box 配置，不断开用户连接，Reload 失败自动回退到 Restart
- **Deploy 原子性**：`deployMu` 互斥锁保护 Deploy 和 Remove 操作，防止并发竞态条件
- **心跳重试机制**：指数退避重试（最多 3 次），5xx 错误和网络错误自动重试，4xx 错误不重试
- **流量统计持久化**：累计流量数据每 30 秒保存到 `/var/lib/node-agent/traffic.json`，重启后自动恢复

**文档更新**：

- 新增 Trojan 服务端入站配置示例（直连 + WebSocket）
- 新增 Trojan URI 格式和客户端参数说明
- 新增热更新机制（Reload）详细文档和 Reload vs Restart 对比表
- 新增 `/deploy/remove` API 接口文档
- 新增流量持久化机制说明
- 新增心跳指数退避重试机制说明
- 新增设计决策：热更新、原子性、重试、持久化、Trojan 传输
- Laravel 控制面板指南添加 Trojan 协议配置和 removeUser 方法
- Android 客户端指南添加 buildTrojanConfig 方法

### v1.10.0 (2026-04-30)

**新功能 — Hysteria2 性能优化**：

- 服务端 inbound 添加 `initial_packet_size: 1400`，避免 QUIC 包 IP 分片
- 服务端 inbound 添加 `masquerade: https://www.bing.com`，伪装为正常网站
- 服务端/客户端 TLS 添加 `alpn: ["h3"]`，明确 HTTP/3 协商
- 客户端 TUN MTU 从 9000 降为 4160，避免 QUIC over UDP 的 IP 分片问题
- 设置带宽时自动启用 `ignore_client_bandwidth`，强制使用 Brutal CC
- 安装脚本新增 `optimize_system()`：UDP 缓冲区 16MB、BBR 拥塞控制、tcp_fastopen
- systemd 服务添加 `CPUSchedulingPolicy=rr` 和 `CPUSchedulingPriority=99`

**文档更新**：

- 新增第 12 章「Laravel 控制面板开发指南」：数据库设计、核心服务、心跳监控、流量计费、签名密码机制、定时任务
- 新增第 13 章「客户端开发指南」：整体架构、连接流程、配置组装、Hysteria2 性能优化、流量监控、iOS/Android 实现要点

### v1.9.3 (2026-04-30)

**修复**：

- 修复混合协议部署（VLESS + Hysteria2）在线用户丢失：之前 Clash API 找到 VLESS 用户后直接返回，跳过 V2Ray API 导致 Hysteria2 用户丢失
- 现在双源合并：同时查询 Clash API 和 V2Ray API，按 `user_id` 去重合并
- 任一数据源失败时自动降级到另一数据源

### v1.9.2 (2026-04-30)

**修复**：

- Hysteria2 连接的 `inbound` 字段为空：从 Clash API 的 `type` 字段（如 `hysteria2/hysteria2-in`）提取 inbound tag
- `/traffic/user` 接口过滤掉 `outbound:direct` 条目，只返回真实用户

### v1.9.1 (2026-04-30)

**新功能**：

- 在线用户 `inbound` 和 `ip` 字段填充：结合 Clash API 连接信息为 V2Ray API 检测到的用户补充 IP 和入站标签
- Hysteria2 用户从 Clash API 活跃连接中提取 inbound tag 和 sourceIP

### v1.9.0 (2026-04-30)

**重大变更**：

- 在线用户检测支持 Hysteria2 协议：Clash API 不返回 Hysteria2 的 `inboundUser`，新增 V2Ray API 回退机制
- `MultiCollector` 注入 `V2RayStatsCollector`，当 Clash API 无法识别用户时，通过 V2Ray API 流量增量检测在线用户
- sing-box 配置生成自动添加 `stats.users` 和 `stats.inbounds`，启用 V2Ray API 按用户流量统计

**修复**：

- 修复 `getOnlineUsersFromV2Ray` 中 `lastTraffic` 为 nil 时的 panic（导致 `/online` 返回 `internal server error`）
- 过滤 `outbound:direct` 条目，不将其计入在线用户

### v1.8.8 (2026-04-30)

**修复**：

- sing-box V2Ray API 配置添加 `stats.users` 和 `stats.inbounds` 字段，启用按用户流量统计
- 之前只有 `outbounds: ["direct"]`，V2Ray API 只统计 outbound 流量，不统计用户级流量

### v1.8.7 (2026-04-30)

**修复**：

- 修复 V2Ray stat name 解析 bug：`splitStatName` 自定义解析器处理 `>>>` 分隔符时第三个 `>` 残留到下一个 part
- 改用 `strings.Split(name, ">>>")` 正确解析
- 添加跳过 stat 的调试日志

### v1.8.6 (2026-04-30)

**修复**：

- V2Ray gRPC 连接改用懒连接模式：移除 `grpc.WithBlock()`，避免 sing-box 重启期间 `context deadline exceeded`
- `grpc.Dial()` 立即返回，实际连接在第一次 RPC 调用时建立
- 查询失败时添加日志并标记重连

### v1.8.5 (2026-04-30)

**修复**：

- 移除 `/traffic/user` 接口的 `IsEnabled()` 前置检查，允许 `refresh()` 内部自动重连
- 添加 V2Ray gRPC 连接成功/失败日志

### v1.8.4 (2026-04-30)

**修复**：

- 移除 `MultiCollector.GetOnlineUsers()` 中的 `IsEnabled()` 前置检查
- `V2RayStatsCollector.refresh()` 内部已处理重连，无需外部预检

### v1.8.3 (2026-04-30)

**修复**：

- 修复 `/online` 端点类型断言 bug：`MultiCollector` 断言为 `*SingBoxStatsCollector` 失败返回 `"clash api not available"`
- 现在正确处理 `MultiCollector` 和 `SingBoxStatsCollector` 两种类型
- `/status` 和 `/stats` 接口也添加 V2Ray API 回退

### v1.8.2 (2026-04-30)

**新功能**：

- 心跳路径默认值改为 `/api/v1/node/heartbeat`（适配控制面 v1 路由前缀）
- 安装脚本显示版本号
- `node-agent -version` 命令行参数支持
- Makefile 通过 LDFLAGS 注入版本号

### v1.8.1 (2026-04-30)

**性能优化**：

- `GetStats()` 单次 Clash API `/connections` 调用同时返回统计数据和在线用户列表，消除重复请求
- `/status` 和 `/stats` 接口不再单独调用 `GetOnlineUsers()`，预期延迟降低约 50%（~1s → ~500ms）
- 心跳上报同样复用 `GetStats()` 返回的在线用户数据，不再额外请求

**新功能**：

- `control_plane.heartbeat_path` 配置项：心跳上报路径可配置，默认 `/api/v1/node/heartbeat`，适配不同控制面路由

**修复**：

- 修复心跳上报 404：控制面路由与硬编码路径不一致时，可通过配置项修改

### v1.8.0 (2026-04-30)

**重大变更**：

- 配置生成支持**多用户合并**：同一协议+端口的多个用户自动合并到同一个 inbound 的 users 列表中，多次 Deploy 不再覆盖已有用户配置
- `RemoveDeploy` 删除用户后自动重新构建配置，确保其他用户不受影响
- 心跳上报的 `user_count` 优先从 Clash API `/connections` 获取真实在线用户数，不可用时回退到 DeviceLimiter 计数

**新功能**：

- `MultiCollector` 新增 `GetOnlineUsers()` 方法，委托给内部 `SingBoxStatsCollector`

**修复**：

- 修复多次 Deploy 导致之前用户配置被覆盖，Clash API 无法识别在线用户的问题
- 修复心跳上报在线用户数始终为 0（当控制面未调用 `/device/register` 时）的问题

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