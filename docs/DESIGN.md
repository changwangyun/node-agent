# VPN 商业平台 — 完整方案设计

> 基于 Laravel + Node Agent + 自有客户端的商业 VPN 平台

---

## 目录

1. [系统架构](#1-系统架构)
2. [数据库设计](#2-数据库设计)
3. [Laravel 后端设计](#3-laravel-后端设计)
4. [客户端设计](#4-客户端设计)
5. [核心业务流程](#5-核心业务流程)
6. [安全设计](#6-安全设计)
7. [API 接口规范](#7-api-接口规范)
8. [定时任务](#8-定时任务)
9. [管理后台](#9-管理后台)
10. [部署架构](#10-部署架构)
11. [开发计划](#11-开发计划)

---

## 1. 系统架构

### 1.1 全局架构图

```
                         ┌─────────────────────────────┐
                         │        自有客户端 App         │
                         │  iOS / Android / Mac / Win   │
                         │  ┌───────────────────────┐   │
                         │  │  内嵌 sing-box 引擎    │   │
                         │  │  本地组装配置+代理     │   │
                         │  └───────────┬───────────┘   │
                         └──────────────┼────────────────┘
                                        │ HTTPS
                         ┌──────────────▼────────────────┐
                         │     Laravel 控制面板 (API)      │
                         │                                │
                         │  ┌──────┐ ┌──────┐ ┌──────┐   │
                         │  │认证  │ │节点  │ │套餐  │   │
                         │  │设备  │ │连接  │ │支付  │   │
                         │  │流量  │ │推送  │ │工单  │   │
                         │  └──────┘ └──────┘ └──────┘   │
                         │                                │
                         │  MySQL + Redis + Queue         │
                         └──────────────┬────────────────┘
                                        │ HTTP (Token Auth)
                   ┌────────────────────┼────────────────────┐
                   │                    │                    │
              ┌────▼─────┐        ┌────▼─────┐        ┌────▼─────┐
              │Node Agent│        │Node Agent│        │Node Agent│
              │ VPS-01   │        │ VPS-02   │        │ VPS-03   │
              │ 东京     │        │ 洛杉矶   │        │ 法兰克福 │
              │ sing-box │        │ sing-box │        │ sing-box │
              └──────────┘        └──────────┘        └──────────┘
```

### 1.2 数据流

```
用户注册/登录
    │
    ▼
购买套餐 ──→ 创建订阅 ──→ 支付回调
    │
    ▼
同步用户到节点 ──→ Node Agent POST /deploy
    │                         │
    ▼                         ▼
客户端获取节点列表      sing-box 生成服务端配置（多用户合并到同一 inbound）
    │                         │
    ▼                         ▼
客户端本地组装配置      sing-box 重启生效
    │
    ▼
用户一键连接 ──→ VPN 隧道建立
    │
    ▼
客户端每30秒上报流量 ──→ Laravel 记录+校验
    │
    ▼
流量超限 ──→ 自动断网 + 推送通知
```

### 1.3 与第三方客户端模式的核心区别

| 维度 | 第三方客户端 | 自有客户端 |
|------|------------|-----------|
| 配置下发 | 生成完整 sing-box/clash 配置文件 | 只下节点参数，客户端自行组装 |
| 认证方式 | 订阅链接 Token | 用户登录 + JWT + 设备绑定 |
| 流量统计 | 仅依赖 Node Agent 采集 | 客户端主动上报 + Node Agent 校验 |
| 节点选择 | 用户手动选 | 智能推荐 + 自动故障切换 |
| 限速 | 仅服务端 sing-box 配置 | 服务端配置 + 客户端配合 |
| 安全性 | 订阅链接可被分享 | 密码签名 + 设备指纹 + API签名 |
| 推送 | 无 | APNs/FCM 实时推送 |
| 用户体验 | 需手动配置 | 一键连接 |

---

## 2. 数据库设计

### 2.1 ER 关系图

```
users ──1:N── user_devices
  │
  ├──1:N── subscriptions ──N:1── plans
  │
  ├──1:N── traffic_logs ──N:1── nodes
  │
  ├──1:N── connection_logs ──N:1── nodes
  │
  └──1:N── tickets ──1:N── ticket_replies

nodes ──1:N── node_protocols
  │
  ├──1:N── node_heartbeats
  │
  └──M:N── plans (node_plan)

plans ──1:N── subscriptions
```

### 2.2 完整表结构

#### users — 用户表

```sql
CREATE TABLE users (
    id                    BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    email                 VARCHAR(191) NOT NULL UNIQUE,
    password              VARCHAR(255) NOT NULL,
    name                  VARCHAR(100) NOT NULL,
    phone                 VARCHAR(20) NULL,
    avatar                VARCHAR(255) NULL,
    status                ENUM('active','disabled','expired','limited') NOT NULL DEFAULT 'active',
    email_verified_at     TIMESTAMP NULL,
    balance               DECIMAL(10,2) NOT NULL DEFAULT 0.00 COMMENT '余额(元)',
    traffic_used          BIGINT UNSIGNED NOT NULL DEFAULT 0 COMMENT '本月已用流量(bytes)',
    traffic_limit         BIGINT UNSIGNED NOT NULL DEFAULT 0 COMMENT '月流量上限(0=无限)',
    traffic_reset_day     TINYINT UNSIGNED NOT NULL DEFAULT 1 COMMENT '每月流量重置日(1-28)',
    speed_limit           INT UNSIGNED NOT NULL DEFAULT 0 COMMENT '限速(Mbps, 0=不限)',
    device_limit          TINYINT UNSIGNED NOT NULL DEFAULT 3 COMMENT '同时在线设备数',
    uuid                  CHAR(36) NOT NULL UNIQUE COMMENT '用户UUID(用于V2Ray/sing-box)',
    invite_code           VARCHAR(50) NOT NULL UNIQUE COMMENT '专属邀请码',
    invited_by            BIGINT UNSIGNED NULL COMMENT '邀请人ID',
    fcm_token             VARCHAR(255) NULL COMMENT 'Android推送Token',
    apns_token            VARCHAR(255) NULL COMMENT 'iOS推送Token',
    last_login_at         TIMESTAMP NULL,
    last_login_ip         VARCHAR(45) NULL,
    remember_token        VARCHAR(100) NULL,
    created_at            TIMESTAMP NULL,
    updated_at            TIMESTAMP NULL,

    INDEX idx_status (status),
    INDEX idx_invite_code (invite_code),
    INDEX idx_uuid (uuid)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
```

#### user_devices — 用户设备表

```sql
CREATE TABLE user_devices (
    id                    BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    user_id               BIGINT UNSIGNED NOT NULL,
    device_name           VARCHAR(100) NOT NULL COMMENT '设备名称',
    device_id             VARCHAR(255) NOT NULL COMMENT '设备指纹',
    platform              VARCHAR(50) NOT NULL COMMENT 'ios/android/macos/windows',
    app_version           VARCHAR(20) NULL COMMENT '客户端版本',
    last_ip               VARCHAR(45) NULL,
    last_online_at        TIMESTAMP NULL,
    created_at            TIMESTAMP NULL,
    updated_at            TIMESTAMP NULL,

    UNIQUE KEY uk_user_device (user_id, device_id),
    INDEX idx_user_id (user_id),
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
```

#### nodes — 节点表

```sql
CREATE TABLE nodes (
    id                    BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    name                  VARCHAR(100) NOT NULL COMMENT '节点名称(如: 东京01)',
    code                  VARCHAR(50) NOT NULL UNIQUE COMMENT '节点代码(如: jp-tokyo-01)',
    region                VARCHAR(50) NOT NULL COMMENT '地区(如: 亚太)',
    country               CHAR(2) NOT NULL COMMENT '国家代码(如: JP)',
    flag_emoji            VARCHAR(10) NOT NULL DEFAULT '' COMMENT '国旗emoji',
    server                VARCHAR(255) NOT NULL COMMENT '服务器IP/域名',
    port                  INT UNSIGNED NOT NULL COMMENT '主端口',
    protocol              VARCHAR(50) NOT NULL COMMENT '默认协议: hy2/vless-reality/trojan/ss',
    api_url               VARCHAR(255) NOT NULL COMMENT 'Node Agent API地址',
    api_token             VARCHAR(255) NOT NULL COMMENT 'Node Agent API密钥',
    clash_api_secret      VARCHAR(255) NOT NULL DEFAULT '' COMMENT 'Clash API密钥',
    v2ray_api_addr        VARCHAR(50) NOT NULL DEFAULT '127.0.0.1:10001' COMMENT 'V2Ray API地址',
    password_secret       VARCHAR(255) NOT NULL COMMENT '密码签名密钥(每节点独立)',
    status                ENUM('online','offline','maintenance') NOT NULL DEFAULT 'offline',
    is_visible            BOOLEAN NOT NULL DEFAULT TRUE COMMENT '是否对用户可见',
    sort_order            INT NOT NULL DEFAULT 0 COMMENT '排序权重(越大越靠前)',
    max_users             INT UNSIGNED NOT NULL DEFAULT 0 COMMENT '最大用户数(0=无限)',
    current_users         INT UNSIGNED NOT NULL DEFAULT 0 COMMENT '当前用户数',
    upload_bandwidth      BIGINT UNSIGNED NOT NULL DEFAULT 0 COMMENT '上行带宽(bps)',
    download_bandwidth    BIGINT UNSIGNED NOT NULL DEFAULT 0 COMMENT '下行带宽(bps)',
    traffic_used          BIGINT UNSIGNED NOT NULL DEFAULT 0 COMMENT '节点已用流量(bytes)',
    traffic_limit         BIGINT UNSIGNED NOT NULL DEFAULT 0 COMMENT '节点流量上限(0=无限)',
    health_score          TINYINT UNSIGNED NOT NULL DEFAULT 100 COMMENT '健康分(0-100)',
    avg_latency           INT UNSIGNED NOT NULL DEFAULT 0 COMMENT '平均延迟(ms)',
    last_heartbeat_at     TIMESTAMP NULL,
    created_at            TIMESTAMP NULL,
    updated_at            TIMESTAMP NULL,

    INDEX idx_status (status),
    INDEX idx_region (region),
    INDEX idx_sort (sort_order DESC)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
```

#### node_protocols — 节点协议配置表

```sql
CREATE TABLE node_protocols (
    id                    BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    node_id               BIGINT UNSIGNED NOT NULL,
    protocol              VARCHAR(50) NOT NULL COMMENT 'hy2/vless-reality/vless-ws/trojan/ss',
    port                  INT UNSIGNED NOT NULL,
    config                JSON NOT NULL COMMENT '协议特有配置',
    is_default            BOOLEAN NOT NULL DEFAULT FALSE COMMENT '是否默认协议',
    sort_order            INT NOT NULL DEFAULT 0,
    created_at            TIMESTAMP NULL,
    updated_at            TIMESTAMP NULL,

    INDEX idx_node_id (node_id),
    FOREIGN KEY (node_id) REFERENCES nodes(id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- config JSON 示例:
-- Hysteria2:     {"sni":"example.com","insecure":false,"obfs_type":"","obfs_password":""}
-- VLESS+Reality: {"sni":"www.microsoft.com","reality_public_key":"xxx","reality_short_id":"xxx","reality_dest":"www.microsoft.com:443","flow":"xtls-rprx-vision"}
-- VLESS+WS:      {"sni":"example.com","ws_path":"/ws","ws_host":"example.com"}
-- Trojan:        {"sni":"example.com","insecure":false}
-- Shadowsocks:   {"method":"2022-blake3-aes-256-gcm"}
```

#### node_heartbeats — 节点心跳表

```sql
CREATE TABLE node_heartbeats (
    id                    BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    node_id               BIGINT UNSIGNED NOT NULL,
    cpu_percent           FLOAT UNSIGNED NOT NULL DEFAULT 0,
    mem_percent           FLOAT UNSIGNED NOT NULL DEFAULT 0,
    disk_percent          FLOAT UNSIGNED NOT NULL DEFAULT 0,
    active_connections    INT UNSIGNED NOT NULL DEFAULT 0,
    online_users          INT UNSIGNED NOT NULL DEFAULT 0,
    upload_speed          BIGINT UNSIGNED NOT NULL DEFAULT 0 COMMENT '上传速率(bytes/s)',
    download_speed        BIGINT UNSIGNED NOT NULL DEFAULT 0 COMMENT '下载速率(bytes/s)',
    upload_total          BIGINT UNSIGNED NOT NULL DEFAULT 0 COMMENT '累计上传(bytes)',
    download_total        BIGINT UNSIGNED NOT NULL DEFAULT 0 COMMENT '累计下载(bytes)',
    singbox_running       BOOLEAN NOT NULL DEFAULT FALSE,
    load_1                FLOAT UNSIGNED NOT NULL DEFAULT 0,
    load_5                FLOAT UNSIGNED NOT NULL DEFAULT 0,
    load_15               FLOAT UNSIGNED NOT NULL DEFAULT 0,
    created_at            TIMESTAMP NULL,

    INDEX idx_node_created (node_id, created_at),
    FOREIGN KEY (node_id) REFERENCES nodes(id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
```

#### plans — 套餐表

```sql
CREATE TABLE plans (
    id                    BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    name                  VARCHAR(100) NOT NULL COMMENT '如: 基础版/标准版/旗舰版',
    code                  VARCHAR(50) NOT NULL UNIQUE COMMENT '如: basic/standard/premium',
    description           TEXT NULL,
    price_monthly         DECIMAL(10,2) NOT NULL COMMENT '月付价格(元)',
    price_quarterly       DECIMAL(10,2) NULL COMMENT '季付价格(元)',
    price_half_yearly     DECIMAL(10,2) NULL COMMENT '半年付价格(元)',
    price_yearly          DECIMAL(10,2) NULL COMMENT '年付价格(元)',
    traffic_limit         BIGINT UNSIGNED NOT NULL DEFAULT 0 COMMENT '月流量上限(bytes, 0=无限)',
    speed_limit           INT UNSIGNED NOT NULL DEFAULT 0 COMMENT '限速(Mbps, 0=不限)',
    device_limit          TINYINT UNSIGNED NOT NULL DEFAULT 3 COMMENT '同时在线设备数',
    features              JSON NULL COMMENT '功能特性列表',
    is_visible            BOOLEAN NOT NULL DEFAULT TRUE,
    sort_order            INT NOT NULL DEFAULT 0,
    created_at            TIMESTAMP NULL,
    updated_at            TIMESTAMP NULL,

    INDEX idx_visible_sort (is_visible, sort_order)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- features JSON 示例:
-- ["全球节点","4K视频","游戏加速","专属IP"]
```

#### node_plan — 节点套餐关联表

```sql
CREATE TABLE node_plan (
    node_id               BIGINT UNSIGNED NOT NULL,
    plan_id               BIGINT UNSIGNED NOT NULL,
    created_at            TIMESTAMP NULL,

    PRIMARY KEY (node_id, plan_id),
    FOREIGN KEY (node_id) REFERENCES nodes(id) ON DELETE CASCADE,
    FOREIGN KEY (plan_id) REFERENCES plans(id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
```

#### subscriptions — 用户订阅表

```sql
CREATE TABLE subscriptions (
    id                    BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    user_id               BIGINT UNSIGNED NOT NULL,
    plan_id               BIGINT UNSIGNED NOT NULL,
    order_no              VARCHAR(64) NOT NULL UNIQUE COMMENT '订单号',
    type                  ENUM('monthly','quarterly','half_yearly','yearly') NOT NULL,
    price                 DECIMAL(10,2) NOT NULL COMMENT '实付金额',
    payment_method        VARCHAR(50) NULL COMMENT 'alipay/wechat/stripe/coin',
    payment_status        ENUM('pending','paid','refunded') NOT NULL DEFAULT 'pending',
    payment_no            VARCHAR(255) NULL COMMENT '支付流水号',
    paid_at               TIMESTAMP NULL,
    started_at            TIMESTAMP NULL,
    expired_at            TIMESTAMP NULL,
    status                ENUM('active','expired','cancelled') NOT NULL DEFAULT 'active',
    traffic_used          BIGINT UNSIGNED NOT NULL DEFAULT 0 COMMENT '本周期已用流量',
    traffic_limit         BIGINT UNSIGNED NOT NULL DEFAULT 0 COMMENT '本周期流量上限',
    traffic_reset_at      TIMESTAMP NULL COMMENT '下次流量重置时间',
    auto_renew            BOOLEAN NOT NULL DEFAULT FALSE COMMENT '是否自动续费',
    cancelled_at          TIMESTAMP NULL,
    created_at            TIMESTAMP NULL,
    updated_at            TIMESTAMP NULL,

    INDEX idx_user_id (user_id),
    INDEX idx_status (status),
    INDEX idx_expired (expired_at),
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
    FOREIGN KEY (plan_id) REFERENCES plans(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
```

#### traffic_logs — 流量日志表

```sql
CREATE TABLE traffic_logs (
    id                    BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    user_id               BIGINT UNSIGNED NOT NULL,
    node_id               BIGINT UNSIGNED NOT NULL,
    upload                BIGINT UNSIGNED NOT NULL DEFAULT 0,
    download              BIGINT UNSIGNED NOT NULL DEFAULT 0,
    date                  DATE NOT NULL,
    created_at            TIMESTAMP NULL,
    updated_at            TIMESTAMP NULL,

    UNIQUE KEY uk_user_node_date (user_id, node_id, date),
    INDEX idx_user_date (user_id, date),
    INDEX idx_node_date (node_id, date),
    INDEX idx_date (date),
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
    FOREIGN KEY (node_id) REFERENCES nodes(id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
```

#### connection_logs — 连接日志表

```sql
CREATE TABLE connection_logs (
    id                    BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    user_id               BIGINT UNSIGNED NOT NULL,
    node_id               BIGINT UNSIGNED NOT NULL,
    protocol              VARCHAR(50) NOT NULL,
    client_version        VARCHAR(20) NULL,
    device_id             VARCHAR(255) NULL,
    upload                BIGINT UNSIGNED NOT NULL DEFAULT 0,
    download              BIGINT UNSIGNED NOT NULL DEFAULT 0,
    duration              INT UNSIGNED NOT NULL DEFAULT 0 COMMENT '连接时长(秒)',
    connected_at          TIMESTAMP NOT NULL,
    disconnected_at       TIMESTAMP NULL,
    created_at            TIMESTAMP NULL,

    INDEX idx_user_connected (user_id, connected_at),
    INDEX idx_node_connected (node_id, connected_at),
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
    FOREIGN KEY (node_id) REFERENCES nodes(id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
```

#### tickets — 工单表

```sql
CREATE TABLE tickets (
    id                    BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    user_id               BIGINT UNSIGNED NOT NULL,
    subject               VARCHAR(255) NOT NULL,
    status                ENUM('open','replied','closed') NOT NULL DEFAULT 'open',
    priority              ENUM('low','medium','high') NOT NULL DEFAULT 'medium',
    category              VARCHAR(50) NOT NULL DEFAULT 'other' COMMENT '连接/支付/流量/其他',
    created_at            TIMESTAMP NULL,
    updated_at            TIMESTAMP NULL,

    INDEX idx_user_id (user_id),
    INDEX idx_status (status),
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
```

#### ticket_replies — 工单回复表

```sql
CREATE TABLE ticket_replies (
    id                    BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    ticket_id             BIGINT UNSIGNED NOT NULL,
    user_id               BIGINT UNSIGNED NULL COMMENT '用户回复时非空',
    admin_id              BIGINT UNSIGNED NULL COMMENT '管理员回复时非空',
    content               TEXT NOT NULL,
    is_admin              BOOLEAN NOT NULL DEFAULT FALSE,
    created_at            TIMESTAMP NULL,

    INDEX idx_ticket_id (ticket_id),
    FOREIGN KEY (ticket_id) REFERENCES tickets(id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
```

#### invite_codes — 邀请码表

```sql
CREATE TABLE invite_codes (
    id                    BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    code                  VARCHAR(50) NOT NULL UNIQUE,
    user_id               BIGINT UNSIGNED NOT NULL COMMENT '创建者',
    used_by               BIGINT UNSIGNED NULL COMMENT '使用者',
    max_uses              INT UNSIGNED NOT NULL DEFAULT 1,
    used_count            INT UNSIGNED NOT NULL DEFAULT 0,
    reward_traffic        BIGINT UNSIGNED NOT NULL DEFAULT 0 COMMENT '奖励流量(bytes)',
    reward_days           INT UNSIGNED NOT NULL DEFAULT 0 COMMENT '奖励天数',
    expires_at            TIMESTAMP NULL,
    created_at            TIMESTAMP NULL,

    INDEX idx_code (code),
    INDEX idx_user_id (user_id),
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
```

#### client_versions — 客户端版本管理表

```sql
CREATE TABLE client_versions (
    id                    BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    platform              ENUM('ios','android','macos','windows') NOT NULL,
    version               VARCHAR(20) NOT NULL,
    build_number          INT UNSIGNED NOT NULL,
    download_url          VARCHAR(255) NOT NULL,
    release_notes         TEXT NULL,
    is_force_update       BOOLEAN NOT NULL DEFAULT FALSE COMMENT '是否强制更新',
    min_supported_version VARCHAR(20) NOT NULL DEFAULT '1.0.0' COMMENT '最低支持版本',
    created_at            TIMESTAMP NULL,
    updated_at            TIMESTAMP NULL,

    INDEX idx_platform (platform)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
```

#### settings — 系统设置表

```sql
CREATE TABLE settings (
    `key`                 VARCHAR(100) NOT NULL PRIMARY KEY,
    value                 TEXT NULL,
    updated_at            TIMESTAMP NULL
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- 预置数据:
-- site_name             站点名称
-- site_url              站点URL
-- register_mode         注册模式: open/invite/closed
-- default_plan_id       默认套餐(注册即送)
-- default_days          默认赠送天数
-- default_traffic       默认赠送流量(bytes)
-- currency              货币: CNY/USD
-- stripe_key            Stripe公钥
-- stripe_secret         Stripe密钥
-- alipay_app_id         支付宝AppID
-- alipay_private_key    支付宝私钥
-- fcm_server_key        FCM推送密钥
-- apns_certificate      APNs证书路径
-- traffic_warn_percent  流量告警百分比(如80)
-- expire_warn_days      到期提前告警天数(如3)
```

---

## 3. Laravel 后端设计

### 3.1 项目目录结构

```
app/
├── Models/
│   ├── User.php
│   ├── UserDevice.php
│   ├── Node.php
│   ├── NodeProtocol.php
│   ├── NodeHeartbeat.php
│   ├── Plan.php
│   ├── Subscription.php
│   ├── TrafficLog.php
│   ├── ConnectionLog.php
│   ├── Ticket.php
│   ├── TicketReply.php
│   ├── InviteCode.php
│   ├── ClientVersion.php
│   └── Setting.php
│
├── Services/
│   ├── NodeService.php                -- 节点管理
│   ├── NodeAgentClient.php            -- Node Agent HTTP 客户端
│   ├── SyncService.php                -- 用户配置同步到节点
│   ├── UserService.php                -- 用户管理
│   ├── SubscriptionService.php        -- 订阅管理
│   ├── TrafficService.php             -- 流量统计与限额
│   ├── PlanService.php                -- 套餐管理
│   ├── NodePasswordService.php        -- 连接密码签名
│   ├── NodeRecommendationService.php  -- 智能节点推荐
│   ├── PaymentService.php             -- 支付集成
│   ├── DeviceService.php              -- 设备管理
│   ├── PushService.php                -- 推送通知
│   └── InviteService.php              -- 邀请返利
│
├── Http/Controllers/Api/
│   ├── V1/
│   │   ├── AuthController.php         -- 登录/注册/忘记密码
│   │   ├── UserController.php         -- 用户信息/仪表盘
│   │   ├── NodeController.php         -- 节点列表/推荐
│   │   ├── ConnectionController.php   -- 连接/断开
│   │   ├── TrafficController.php      -- 流量上报/查询
│   │   ├── SubscriptionController.php -- 订阅/购买/续费
│   │   ├── PaymentController.php      -- 支付回调
│   │   ├── DeviceController.php       -- 设备管理
│   │   ├── TicketController.php       -- 工单
│   │   ├── PlanController.php         -- 套餐列表
│   │   └── AppController.php          -- 版本检查/更新
│   │
│   └── Admin/
│       ├── AuthController.php
│       ├── DashboardController.php
│       ├── UserManageController.php
│       ├── NodeManageController.php
│       ├── PlanManageController.php
│       ├── OrderManageController.php
│       ├── TicketManageController.php
│       └── SettingController.php
│
├── Http/Middleware/
│   ├── JwtAuth.php                    -- JWT认证
│   ├── AdminAuth.php                  -- 管理员认证
│   ├── CheckSubscription.php          -- 订阅有效性
│   ├── CheckTrafficLimit.php          -- 流量限额
│   ├── VerifyClientSignature.php      -- 客户端请求签名验证
│   └── RateLimitPerUser.php           -- 用户级限流
│
├── Http/Requests/
│   ├── Auth/
│   │   ├── LoginRequest.php
│   │   └── RegisterRequest.php
│   ├── Node/
│   │   └── ConnectRequest.php
│   ├── Traffic/
│   │   └── ReportRequest.php
│   └── Subscription/
│       └── PurchaseRequest.php
│
├── Http/Resources/
│   ├── UserResource.php
│   ├── NodeResource.php
│   ├── PlanResource.php
│   ├── SubscriptionResource.php
│   └── TrafficResource.php
│
├── Console/Commands/
│   ├── SyncNodesCommand.php           -- 同步节点配置和用户
│   ├── CheckExpiredCommand.php        -- 检查过期订阅
│   ├── ResetTrafficCommand.php        -- 每月流量重置
│   ├── CollectNodeStatsCommand.php    -- 采集节点统计
│   ├── CleanTrafficLogsCommand.php    -- 清理历史日志
│   ├── CheckNodeHealthCommand.php     -- 节点健康检查
│   └── SendExpiryWarningsCommand.php  -- 到期提醒推送
│
├── Jobs/
│   ├── SyncUserToNodes.php            -- 异步同步用户到节点
│   ├── RemoveUserFromNodes.php        -- 异步移除用户
│   ├── ProcessPayment.php             -- 异步处理支付
│   ├── SendPushNotification.php       -- 异步推送
│   └── CollectNodeStat.php            -- 异步采集单节点统计
│
├── Events/
│   ├── UserRegistered.php
│   ├── UserSubscribed.php
│   ├── SubscriptionExpired.php
│   ├── TrafficLimitReached.php
│   └── NodeDown.php
│
├── Listeners/
│   ├── GenerateUserUuid.php           -- 注册时生成UUID
│   ├── GenerateInviteCode.php         -- 注册时生成邀请码
│   ├── SyncUserOnSubscribe.php        -- 订阅后同步到节点
│   ├── RemoveUserOnExpire.php         -- 过期后从节点移除
│   ├── NotifyTrafficLimit.php         -- 流量超限通知
│   └── NotifyNodeDown.php             -- 节点下线通知
│
├── Exceptions/
│   ├── SubscriptionExpiredException.php
│   ├── TrafficLimitException.php
│   ├── DeviceLimitException.php
│   └── NodeUnavailableException.php
│
└── Enums/
    ├── UserStatus.php
    ├── NodeStatus.php
    ├── SubscriptionStatus.php
    ├── PaymentStatus.php
    ├── BillingCycle.php
    └── Protocol.php
```

### 3.2 核心服务详细设计

#### NodeAgentClient — 与 Node Agent 通信

```php
<?php

namespace App\Services;

use App\Models\Node;
use Illuminate\Http\Client\ConnectionException;
use Illuminate\Support\Facades\Http;
use Illuminate\Support\Facades\Log;

class NodeAgentClient
{
    private Node $node;
    private int $timeout;

    public function __construct(Node $node, int $timeout = 10)
    {
        $this->node = $node;
        $this->timeout = $timeout;
    }

    public function deploy(array $request): array
    {
        return $this->post('/deploy', $request);
    }

    public function status(): array
    {
        return $this->get('/status');
    }

    public function stats(): array
    {
        return $this->get('/stats');
    }

    public function online(): array
    {
        return $this->get('/online');
    }

    public function restart(): array
    {
        return $this->post('/restart');
    }

    public function stop(): array
    {
        return $this->post('/stop');
    }

    public function start(): array
    {
        return $this->post('/start');
    }

    public function logs(): array
    {
        return $this->get('/logs');
    }

    public function healthCheck(): bool
    {
        try {
            $this->get('/health');
            return true;
        } catch (\Throwable $e) {
            return false;
        }
    }

    private function get(string $path, array $query = []): array
    {
        try {
            $response = Http::withHeaders($this->headers())
                ->timeout($this->timeout)
                ->get($this->url($path), $query);

            return $response->json() ?? [];
        } catch (ConnectionException $e) {
            Log::warning("Node Agent connection failed: {$this->node->code}", [
                'error' => $e->getMessage()
            ]);
            return [];
        }
    }

    private function post(string $path, array $data = []): array
    {
        try {
            $response = Http::withHeaders($this->headers())
                ->timeout($this->timeout)
                ->post($this->url($path), $data);

            return $response->json() ?? [];
        } catch (ConnectionException $e) {
            Log::warning("Node Agent request failed: {$this->node->code}", [
                'path' => $path,
                'error' => $e->getMessage()
            ]);
            return [];
        }
    }

    private function url(string $path): string
    {
        return rtrim($this->node->api_url, '/') . $path;
    }

    private function headers(): array
    {
        return [
            'X-Node-Token' => $this->node->api_token,
            'Accept' => 'application/json',
        ];
    }
}
```

#### SyncService — 用户配置同步（最核心）

```php
<?php

namespace App\Services;

use App\Models\Node;
use App\Models\User;
use App\Models\Subscription;
use Illuminate\Support\Facades\Log;

class SyncService
{
    public function syncUserToNodes(User $user): void
    {
        $subscription = $user->activeSubscription;

        if (!$subscription || !$subscription->isActive()) {
            $this->removeUserFromAllNodes($user);
            return;
        }

        $plan = $subscription->plan;
        $nodes = $plan->nodes()->where('status', 'online')->get();

        foreach ($nodes as $node) {
            $this->syncUserToNode($user, $node, $plan);
        }
    }

    public function syncUserToNode(User $user, Node $node, $plan = null): void
    {
        if (!$plan) {
            $subscription = $user->activeSubscription;
            if (!$subscription) return;
            $plan = $subscription->plan;
        }

        $client = new NodeAgentClient($node);
        $protocols = $node->protocols;

        foreach ($protocols as $protocol) {
            $deployRequest = $this->buildDeployRequest(
                $user, $node, $protocol, $plan
            );

            $result = $client->deploy($deployRequest);

            if (empty($result)) {
                Log::error("Failed to sync user to node", [
                    'user_id' => $user->id,
                    'node_id' => $node->id,
                    'protocol' => $protocol->protocol,
                ]);
            }
        }
    }

    public function removeUserFromAllNodes(User $user): void
    {
        $nodes = Node::where('status', 'online')->get();

        foreach ($nodes as $node) {
            $this->removeUserFromNode($user, $node);
        }
    }

    public function removeUserFromNode(User $user, Node $node): void
    {
        // 调用 Node Agent DELETE /deploy?user_id=xxx 移除用户
        // Node Agent 会自动重新构建配置（不含此用户），保留其他用户
    }

    public function syncAllUsersToNode(Node $node): void
    {
        $planIds = $node->plans()->pluck('plans.id');

        $users = User::whereHas('activeSubscription', function ($q) use ($planIds) {
            $q->where('status', 'active')
              ->whereIn('plan_id', $planIds);
        })->get();

        foreach ($users as $user) {
            $this->syncUserToNode($user, $node);
        }

        $node->update(['current_users' => $users->count()]);
    }

    private function buildDeployRequest(
        User $user,
        Node $node,
        $protocol,
        $plan
    ): array {
        $config = $protocol->config;
        $password = app(NodePasswordService::class)
            ->generatePassword($user, $node);

        $request = [
            'user_id'   => $user->uuid,
            'node_id'   => $node->code,
            'protocol'  => $protocol->protocol,
            'server'    => $node->server,
            'port'      => $protocol->port,
            'password'  => $password,
            'uuid'      => $user->uuid,
            'up_mbps'   => $plan->speed_limit ?: 0,
            'down_mbps' => $plan->speed_limit ?: 0,
        ];

        switch ($protocol->protocol) {
            case 'hy2':
                $request['sni'] = $config['sni'] ?? $node->server;
                $request['insecure'] = $config['insecure'] ?? false;
                if (!empty($config['obfs_type'])) {
                    $request['obfs_type'] = $config['obfs_type'];
                    $request['obfs_password'] = $config['obfs_password'] ?? '';
                }
                break;

            case 'vless-reality':
                $request['sni'] = $config['sni'] ?? 'www.microsoft.com';
                $request['reality_public_key'] = $config['reality_public_key'] ?? '';
                $request['reality_short_id'] = $config['reality_short_id'] ?? '';
                $request['reality_dest'] = $config['reality_dest'] ?? '';
                $request['reality_dest_port'] = $config['reality_dest_port'] ?? 443;
                break;

            case 'vless-ws':
                $request['sni'] = $config['sni'] ?? $node->server;
                $request['ws_path'] = $config['ws_path'] ?? '/ws';
                $request['ws_host'] = $config['ws_host'] ?? $node->server;
                break;

            case 'trojan':
                $request['sni'] = $config['sni'] ?? $node->server;
                $request['insecure'] = $config['insecure'] ?? false;
                break;
        }

        return $request;
    }
}
```

#### NodePasswordService — 连接密码签名

```php
<?php

namespace App\Services;

use App\Models\Node;
use App\Models\User;

class NodePasswordService
{
    public function generatePassword(User $user, Node $node): string
    {
        $payload = $user->uuid . '.' . time();
        $signature = hash_hmac('sha256', $payload, $node->password_secret);
        return base64_encode($payload . '.' . $signature);
    }

    public function verifyPassword(string $password, Node $node): ?User
    {
        $decoded = base64_decode($password);
        if (!$decoded) return null;

        $parts = explode('.', $decoded, 3);
        if (count($parts) !== 3) return null;

        [$uuid, $timestamp, $signature] = $parts;

        $expected = hash_hmac('sha256', "$uuid.$timestamp", $node->password_secret);
        if (!hash_equals($expected, $signature)) return null;

        if (time() - (int)$timestamp > 86400) return null;

        return User::where('uuid', $uuid)->first();
    }
}
```

#### TrafficService — 流量采集与限额

```php
<?php

namespace App\Services;

use App\Models\Node;
use App\Models\User;
use App\Models\TrafficLog;
use App\Events\TrafficLimitReached;
use Illuminate\Support\Facades\DB;

class TrafficService
{
    public function collectFromNodes(): void
    {
        $nodes = Node::where('status', 'online')->get();

        foreach ($nodes as $node) {
            dispatch(new \App\Jobs\CollectNodeStat($node));
        }
    }

    public function recordNodeHeartbeat(Node $node, array $stats): void
    {
        $node->heartbeats()->create([
            'cpu_percent'        => $stats['system']['cpu_percent'] ?? 0,
            'mem_percent'        => $stats['system']['mem_percent'] ?? 0,
            'active_connections' => $stats['connections']['active'] ?? 0,
            'online_users'       => $stats['online']['user_count'] ?? ($stats['devices']['online_users'] ?? 0),
            'upload_speed'       => $stats['speed']['upload'] ?? 0,
            'download_speed'     => $stats['speed']['download'] ?? 0,
            'upload_total'       => $stats['traffic']['upload'] ?? 0,
            'download_total'     => $stats['traffic']['download'] ?? 0,
            'singbox_running'    => $stats['node']['singbox_running'] ?? false,
            'load_1'             => $stats['system']['load_1'] ?? 0,
            'load_5'             => $stats['system']['load_5'] ?? 0,
            'load_15'            => $stats['system']['load_15'] ?? 0,
        ]);

        $node->update(['last_heartbeat_at' => now()]);
    }

    public function recordUserTraffic(User $user, int $nodeId, int $upload, int $download): void
    {
        $total = $upload + $download;

        TrafficLog::updateOrCreate(
            [
                'user_id' => $user->id,
                'node_id' => $nodeId,
                'date'    => today(),
            ],
            [
                'upload'   => DB::raw("upload + $upload"),
                'download' => DB::raw("download + $download"),
            ]
        );

        $user->increment('traffic_used', $total);

        $this->checkTrafficLimit($user);
    }

    public function checkTrafficLimit(User $user): void
    {
        if ($user->traffic_limit <= 0) return;

        if ($user->traffic_used >= $user->traffic_limit) {
            $user->update(['status' => 'limited']);

            app(SyncService::class)->removeUserFromAllNodes($user);

            event(new TrafficLimitReached($user));
        }
    }

    public function resetMonthlyTraffic(): void
    {
        $today = now()->day;

        User::where('traffic_reset_day', $today)
            ->where('status', '!=', 'disabled')
            ->update([
                'traffic_used' => 0,
                'status' => DB::raw("CASE WHEN status = 'limited' THEN 'active' ELSE status END"),
            ]);

        User::where('traffic_reset_day', $today)
            ->where('status', 'active')
            ->whereHas('activeSubscription')
            ->chunk(100, function ($users) {
                foreach ($users as $user) {
                    dispatch(new \App\Jobs\SyncUserToNodes($user));
                }
            });
    }
}
```

#### NodeRecommendationService — 智能节点推荐

```php
<?php

namespace App\Services;

use App\Models\User;
use App\Models\Node;
use Illuminate\Support\Collection;

class NodeRecommendationService
{
    public function getRecommended(User $user, int $count = 3): Collection
    {
        $subscription = $user->activeSubscription;
        if (!$subscription) return collect();

        $nodes = $subscription->plan->nodes()
            ->where('is_visible', true)
            ->where('status', 'online')
            ->get();

        return $nodes->map(fn($node) => tap($node, function ($n) use ($user) {
            $n->score = $this->calculateScore($n, $user);
        }))
        ->sortByDesc('score')
        ->take($count)
        ->values();
    }

    private function calculateScore(Node $node, User $user): float
    {
        $score = 100.0;

        // 健康分 30%
        $score -= (100 - $node->health_score) * 0.3;

        // 负载 25%
        if ($node->max_users > 0) {
            $loadRatio = $node->current_users / $node->max_users;
            $score -= $loadRatio * 25;
        }

        // 延迟 25%
        $score -= min($node->avg_latency / 10, 25);

        // 地理距离 20%
        $userCountry = $this->ipToCountry($user->last_login_ip);
        if ($userCountry && $userCountry !== $node->country) {
            $distance = $this->geoDistance($userCountry, $node->country);
            $score -= min($distance / 500, 20);
        }

        return max($score, 0);
    }

    private function ipToCountry(?string $ip): ?string
    {
        if (!$ip) return null;
        // 使用 GeoIP 库
        return null;
    }

    private function geoDistance(string $country1, string $country2): float
    {
        $regions = [
            'CN' => 'asia', 'JP' => 'asia', 'KR' => 'asia', 'SG' => 'asia',
            'HK' => 'asia', 'TW' => 'asia',
            'US' => 'america', 'CA' => 'america',
            'DE' => 'europe', 'FR' => 'europe', 'GB' => 'europe', 'NL' => 'europe',
            'AU' => 'oceania',
        ];

        $r1 = $regions[$country1] ?? 'other';
        $r2 = $regions[$country2] ?? 'other';

        if ($r1 === $r2) return 100;
        if ($r1 === 'asia' && $r2 === 'oceania') return 300;
        return 800;
    }
}
```

#### PushService — 推送通知

```php
<?php

namespace App\Services;

use App\Models\User;

class PushService
{
    public function send(User $user, string $title, string $body, array $data = []): bool
    {
        if ($user->fcm_token) {
            return $this->sendFcm($user->fcm_token, $title, $body, $data);
        }

        if ($user->apns_token) {
            return $this->sendApns($user->apns_token, $title, $body, $data);
        }

        return false;
    }

    public function sendToMany($users, string $title, string $body, array $data = []): void
    {
        foreach ($users as $user) {
            dispatch(new \App\Jobs\SendPushNotification($user, $title, $body, $data));
        }
    }

    private function sendFcm(string $token, string $title, string $body, array $data): bool
    {
        // Firebase Cloud Messaging 实现
        return true;
    }

    private function sendApns(string $token, string $title, string $body, array $data): bool
    {
        // Apple Push Notification Service 实现
        return true;
    }
}
```

### 3.3 事件与监听器

```php
// EventServiceProvider

protected $listen = [
    \App\Events\UserRegistered::class => [
        \App\Listeners\GenerateUserUuid::class,
        \App\Listeners\GenerateInviteCode::class,
        \App\Listeners\HandleInviteReward::class,
    ],
    \App\Events\UserSubscribed::class => [
        \App\Listeners\SyncUserOnSubscribe::class,
        \App\Listeners\SendSubscribeNotification::class,
    ],
    \App\Events\SubscriptionExpired::class => [
        \App\Listeners\RemoveUserOnExpire::class,
        \App\Listeners\SendExpireNotification::class,
    ],
    \App\Events\TrafficLimitReached::class => [
        \App\Listeners\NotifyTrafficLimit::class,
        \App\Listeners\DisconnectUserDevices::class,
    ],
    \App\Events\NodeDown::class => [
        \App\Listeners\NotifyNodeDown::class,
        \App\Listeners\AutoSwitchUsersNode::class,
    ],
];
```

---

## 4. 客户端设计

### 4.1 客户端架构

```
客户端 App
├── UI 层
│   ├── 登录/注册页
│   ├── 首页（一键连接 + 状态）
│   ├── 节点列表（延迟/负载/国旗）
│   ├── 流量仪表盘（实时网速 + 用量环形图）
│   ├── 套餐/购买页
│   ├── 我的（设备管理/邀请/设置）
│   └── 工单页
│
├── 业务层
│   ├── AuthService          -- JWT 登录/刷新
│   ├── NodeService          -- 节点列表/测速
│   ├── ConnectionService    -- 连接管理
│   ├── TrafficService       -- 流量上报
│   ├── SubscriptionService  -- 套餐查询
│   └── PushService          -- 推送接收
│
├── 代理层（sing-box 引擎）
│   ├── SBManager            -- sing-box 生命周期
│   ├── ConfigBuilder        -- 根据节点参数构建配置
│   ├── VPNAdapter           -- 系统 VPN 接口适配
│   └── TrafficMonitor       -- 本地流量监控
│
└── 系统层
    ├── VPNExtension         -- iOS NetworkExtension / Android VpnService
    ├── Keychain/Keystore    -- 安全存储
    └── Reachability         -- 网络状态
```

### 4.2 一键连接流程

```
用户点击「连接」
    │
    ▼
1. 客户端 → Laravel: POST /api/v1/connection/connect
   { node_id: 5, protocol: "hy2" }
   ← 返回签名密码 + 节点参数
    │
    ▼
2. ConfigBuilder 本地组装 sing-box 配置:
   {
     inbounds: [{ type: "tun", ... }],
     outbounds: [{
       type: "hysterium2",
       server: "jp1.example.com",
       port: 443,
       password: "签名的密码",
       ...
     }],
     route: { ... }
   }
    │
    ▼
3. SBManager 启动 sing-box
    │
    ▼
4. 系统建立 VPN 隧道
    │
    ▼
5. TrafficMonitor 开始监控
    │
    ▼
6. 每30秒 → Laravel: POST /api/v1/traffic/report
   { node_id, upload, download }
    │
    ▼
7. UI 实时显示网速和用量
```

### 4.3 客户端 sing-box 配置组装

Laravel 只返回轻量节点参数，客户端本地组装完整配置：

```json
// Laravel API 返回
{
  "id": 5,
  "name": "东京01",
  "country": "JP",
  "flag": "🇯🇵",
  "protocols": [
    {
      "type": "hysteria2",
      "server": "jp1.example.com",
      "port": 443,
      "password": "base64签名密码",
      "sni": "jp1.example.com",
      "insecure": false
    }
  ],
  "load": 35,
  "latency": 85
}
```

客户端本地组装完整 sing-box 配置：

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
      "mtu": 9000,
      "auto_route": true,
      "strict_route": true
    }
  ],
  "outbounds": [
    {
      "type": "hysteria2",
      "tag": "proxy",
      "server": "jp1.example.com",
      "server_port": 443,
      "password": "base64签名密码",
      "tls": {
        "enabled": true,
        "server_name": "jp1.example.com"
      }
    },
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

### 4.4 客户端技术选型

| 平台 | 框架 | sing-box 集成 | VPN 接口 |
|------|------|--------------|----------|
| iOS | Swift + SwiftUI | sing-box iOS Framework | NetworkExtension (NEPacketTunnelProvider) |
| Android | Kotlin + Compose | sing-box AAR | VpnService |
| macOS | Swift + SwiftUI | 同 iOS Framework | NetworkExtension |
| Windows | C# WPF/WinUI | sing-box exe 子进程 | WinTUN |

sing-box 官方提供 Mobile Library：https://github.com/SagerNet/sing-box/tree/dev/platform

---

## 5. 核心业务流程

### 5.1 用户注册流程

```
用户输入 email + password + (invite_code?)
    │
    ▼
Laravel 验证
    │
    ├─ 注册模式=open → 直接注册
    ├─ 注册模式=invite → 验证邀请码
    └─ 注册模式=closed → 拒绝
    │
    ▼
创建用户
    ├─ 生成 UUID
    ├─ 生成 专属邀请码
    └─ 如果有默认套餐 → 自动创建试用订阅
    │
    ▼
如果有试用订阅 → 异步同步到节点
    │
    ▼
返回 JWT Token + 用户信息
```

### 5.2 购买套餐流程

```
用户选择套餐 + 周期
    │
    ▼
创建订阅订单 (status=pending)
    │
    ▼
调用支付接口
    ├─ Stripe → 返回 PaymentIntent
    ├─ 支付宝 → 返回支付链接
    └─ 余额 → 直接扣款
    │
    ▼
支付回调
    ├─ 验证签名
    ├─ 更新 payment_status=paid
    ├─ 更新订阅 status=active, started_at, expired_at
    └─ 触发 UserSubscribed 事件
    │
    ▼
UserSubscribed 监听器
    ├─ 同步用户到所有可用节点
    └─ 发送推送通知
```

### 5.3 连接流程

```
客户端请求连接
    │
    ▼
Laravel 验证
    ├─ JWT 有效？
    ├─ 订阅有效？
    ├─ 流量未超限？
    └─ 设备数未超限？
    │
    ▼
生成签名密码 (NodePasswordService)
    │
    ▼
返回节点参数 + 签名密码
    │
    ▼
客户端本地组装 sing-box 配置
    │
    ▼
启动 VPN 隧道
    │
    ▼
每30秒上报流量
    │
    ▼
Laravel 累加流量 + 检查限额
    ├─ 未超限 → 返回 { ok, remaining }
    └─ 已超限 → 返回 { limited } + 客户端自动断开
```

### 5.4 过期处理流程

```
定时任务每分钟检查
    │
    ▼
查找 expired_at < now() 且 status=active 的订阅
    │
    ▼
更新 status=expired
    │
    ▼
触发 SubscriptionExpired 事件
    ├─ 从所有节点移除用户配置
    ├─ 更新用户 status=expired
    └─ 推送通知用户续费
```

### 5.5 节点故障自动切换

```
定时任务每5分钟检查节点心跳
    │
    ▼
发现节点 last_heartbeat_at > 3分钟前
    │
    ▼
标记 status=offline
    │
    ▼
触发 NodeDown 事件
    ├─ 通知所有在线用户该节点不可用
    └─ 客户端收到推送自动切换到推荐节点
```

---

## 6. 安全设计

### 6.1 多层安全体系

```
第1层: 客户端请求签名
  每个请求带 X-Signature: hmac(body.timestamp.client_secret)
  防止 API 被第三方工具直接调用

第2层: JWT + 设备绑定
  登录时绑定 device_id
  Token 与设备关联，换设备需重新登录

第3层: 连接密码签名
  密码 = base64(uuid.timestamp.hmac)
  24小时有效，每节点独立密钥

第4层: 流量双向校验
  客户端上报 + Node Agent 采集
  差异过大则告警（防刷流量）

第5层: 设备数限制
  指纹 + 在线数双重校验
  超限自动踢最旧设备
```

### 6.2 客户端请求签名中间件

```php
<?php

namespace App\Http\Middleware;

use Closure;
use Illuminate\Http\Request;

class VerifyClientSignature
{
    public function handle(Request $request, Closure $next)
    {
        $signature = $request->header('X-Signature');
        $timestamp = $request->header('X-Timestamp');
        $nonce = $request->header('X-Nonce');

        if (!$signature || !$timestamp || !$nonce) {
            return response()->json(['message' => 'Missing signature headers'], 401);
        }

        if (abs(time() - (int)$timestamp) > 300) {
            return response()->json(['message' => 'Request expired'], 401);
        }

        $cacheKey = "nonce:{$nonce}";
        if (cache()->has($cacheKey)) {
            return response()->json(['message' => 'Duplicate request'], 401);
        }
        cache()->put($cacheKey, true, 300);

        $body = $request->getContent();
        $expected = hash_hmac(
            'sha256',
            "{$body}.{$timestamp}.{$nonce}",
            config('app.client_secret')
        );

        if (!hash_equals($expected, $signature)) {
            return response()->json(['message' => 'Invalid signature'], 401);
        }

        return $next($request);
    }
}
```

### 6.3 防共享措施

| 措施 | 说明 |
|------|------|
| 设备指纹 | 每台设备唯一ID，绑定到用户 |
| 同时在线限制 | 套餐设定设备数上限 |
| 密码时效 | 签名密码24小时过期 |
| IP 异常检测 | 同一账户短时间内多IP登录告警 |
| 流量异常检测 | 单设备日流量异常高则告警 |

---

## 7. API 接口规范

### 7.1 通用规范

```
Base URL: https://api.your-domain.com/api/v1

请求头:
  Authorization: Bearer {jwt_token}
  X-Signature: {hmac_signature}
  X-Timestamp: {unix_timestamp}
  X-Nonce: {random_string}
  X-Device-Id: {device_fingerprint}
  X-App-Version: {client_version}
  X-Platform: ios|android|macos|windows

响应格式:
{
  "code": 0,
  "message": "success",
  "data": { ... }
}

错误码:
  0     成功
  1001  参数错误
  1002  未认证
  1003  无权限
  2001  订阅过期
  2002  流量超限
  2003  设备数超限
  3001  节点不可用
  4001  支付失败
  5001  服务器错误
```

### 7.2 认证接口

```
POST /api/v1/auth/register
  请求: { email, password, name, invite_code? }
  响应: { user, token, subscription? }

POST /api/v1/auth/login
  请求: { email, password, device_id, device_name, platform }
  响应: { user, token, subscription, nodes }

POST /api/v1/auth/refresh
  响应: { token }

POST /api/v1/auth/logout
  请求: { device_id }

POST /api/v1/auth/forgot-password
  请求: { email }

POST /api/v1/auth/reset-password
  请求: { email, token, password }
```

### 7.3 用户接口

```
GET  /api/v1/user
  响应: { user, subscription, traffic_summary }

PUT  /api/v1/user
  请求: { name?, phone? }

POST /api/v1/user/change-password
  请求: { old_password, new_password }

PUT  /api/v1/user/push-token
  请求: { token, type: "fcm"|"apns" }
```

### 7.4 节点接口

```
GET  /api/v1/nodes
  响应: {
    nodes: [
      {
        id, name, country, flag, region,
        load, latency,
        protocols: [
          { type, port, server, password, sni, ... }
        ]
      }
    ]
  }

GET  /api/v1/nodes/recommended
  响应: { nodes: [...top3] }

POST /api/v1/nodes/{id}/latency
  请求: { latency_ms }
  说明: 客户端上报测速结果
```

### 7.5 连接接口

```
POST /api/v1/connection/connect
  请求: { node_id, protocol_type }
  响应: {
    node: { id, name, server, port, ... },
    password: "签名密码",
    config_params: { sni, ... },
    expires_at: "2026-04-29T00:00:00Z"
  }

POST /api/v1/connection/disconnect
  请求: { node_id, upload, download, duration }
  说明: 记录断开+最终流量
```

### 7.6 流量接口

```
POST /api/v1/traffic/report
  请求: { node_id, upload, download }
  响应: {
    traffic_used,
    traffic_limit,
    traffic_remaining,
    percentage,
    is_limited
  }

GET  /api/v1/traffic/summary
  响应: {
    today: { upload, download },
    month: { upload, download, limit, percentage },
    history: [ { date, upload, download } ]
  }
```

### 7.7 套餐与订阅接口

```
GET  /api/v1/plans
  响应: { plans: [{ id, name, code, prices, features, ... }] }

GET  /api/v1/subscription
  响应: { subscription, plan }

POST /api/v1/subscription/purchase
  请求: { plan_id, billing_cycle, payment_method }
  响应: { order, payment_params }

POST /api/v1/subscription/verify
  请求: { order_no }
  响应: { payment_status, subscription }

POST /api/v1/subscription/cancel
  说明: 取消自动续费
```

### 7.8 设备接口

```
GET    /api/v1/devices
  响应: { devices: [{ id, name, platform, last_ip, last_online_at, is_current }] }

DELETE /api/v1/devices/{id}
  说明: 踢下线
```

### 7.9 工单接口

```
GET  /api/v1/tickets
  响应: { tickets }

POST /api/v1/tickets
  请求: { subject, category, content }

GET  /api/v1/tickets/{id}
  响应: { ticket, replies }

POST /api/v1/tickets/{id}/reply
  请求: { content }
```

### 7.10 App 接口

```
GET  /api/v1/app/version
  请求参数: platform
  响应: {
    latest_version,
    build_number,
    download_url,
    release_notes,
    is_force_update,
    min_supported_version
  }
```

---

## 8. 定时任务

```php
// app/Console/Kernel.php

protected function schedule(Schedule $schedule)
{
    // ─── 节点相关 ───

    $schedule->command('nodes:collect-stats')
             ->everyFiveMinutes()
             ->withoutOverlapping()
             ->onOneServer();

    $schedule->command('nodes:check-health')
             ->everyFiveMinutes()
             ->withoutOverlapping()
             ->onOneServer();

    // ─── 用户相关 ───

    $schedule->command('subscriptions:check-expired')
             ->everyMinute()
             ->withoutOverlapping()
             ->onOneServer();

    $schedule->command('traffic:reset-monthly')
             ->dailyAt('00:00')
             ->onOneServer();

    $schedule->command('traffic:check-limits')
             ->everyTenMinutes()
             ->withoutOverlapping()
             ->onOneServer();

    // ─── 同步相关 ───

    $schedule->command('nodes:sync-users')
             ->hourly()
             ->withoutOverlapping()
             ->onOneServer();

    // ─── 通知相关 ───

    $schedule->command('notifications:expiry-warning')
             ->dailyAt('09:00')
             ->onOneServer();

    $schedule->command('notifications:traffic-warning')
             ->dailyAt('09:00')
             ->onOneServer();

    // ─── 清理相关 ───

    $schedule->command('traffic:clean-logs')
             ->dailyAt('03:00')
             ->onOneServer();

    $schedule->command('nodes:clean-heartbeats')
             ->dailyAt('04:00')
             ->onOneServer();
}
```

---

## 9. 管理后台

### 9.1 仪表盘

```
┌────────────────────────────────────────────────────────────┐
│  管理后台仪表盘                                             │
├──────────┬──────────┬──────────┬──────────┬────────────────┤
│ 总用户数  │ 活跃用户  │ 今日收入  │ 在线设备  │ 节点状态       │
│  12,345  │  3,456   │ ¥8,900  │  1,234  │ 8在线/1离线    │
├──────────┴──────────┴──────────┴──────────┴────────────────┤
│                                                            │
│  收入趋势图 (7天/30天/12月)                                  │
│  ┌─────────────────────────────────────────────────────┐   │
│  │  折线图                                              │   │
│  └─────────────────────────────────────────────────────┘   │
│                                                            │
│  节点实时状态                                               │
│  ┌─────────┬──────┬──────┬──────┬──────┬──────┬────────┐   │
│  │ 节点    │ 状态  │ CPU  │ 内存 │ 连接 │ 用户 │ 流量   │   │
│  │ 东京01  │ 在线  │ 23%  │ 45%  │ 156  │ 89  │ 2.3TB  │   │
│  │ 洛杉矶  │ 在线  │ 15%  │ 38%  │ 98   │ 67  │ 1.8TB  │   │
│  │ 法兰克福│ 离线  │ -    │ -    │ -    │ -   │ -      │   │
│  └─────────┴──────┴──────┴──────┴──────┴──────┴────────┘   │
│                                                            │
│  最近订单                                                   │
│  ┌──────────┬──────┬──────┬──────┬──────┬────────────────┐  │
│  │ 时间     │ 用户  │ 套餐  │ 金额  │ 状态 │ 操作          │  │
│  │ 10:23   │ u1   │ 旗舰  │ ¥99  │ 已付 │ 查看          │  │
│  └──────────┴──────┴──────┴──────┴──────┴────────────────┘  │
└────────────────────────────────────────────────────────────┘
```

### 9.2 管理后台功能清单

| 模块 | 功能 |
|------|------|
| 仪表盘 | 总览数据、收入趋势、节点状态、实时监控 |
| 用户管理 | 列表/搜索/编辑/封禁/重置流量/手动调整余额 |
| 节点管理 | 添加/编辑/删除、协议配置、手动同步、重启、查看日志 |
| 套餐管理 | 创建/编辑/上下架、关联节点、定价设置 |
| 订单管理 | 列表/搜索/退款、导出 |
| 流量统计 | 全局/按节点/按用户、日/周/月报表 |
| 工单管理 | 列表/回复/关闭 |
| 系统设置 | 站点配置、支付配置、推送配置、注册模式 |
| 客户端版本 | 发布新版本、强制更新设置 |

---

## 10. 部署架构

### 10.1 生产环境

```
                    ┌─────────────┐
                    │   CDN/WAF   │
                    │ Cloudflare  │
                    └──────┬──────┘
                           │
                    ┌──────▼──────┐
                    │ Load Balancer│
                    │   Nginx     │
                    └──────┬──────┘
                           │
              ┌────────────┼────────────┐
              │            │            │
        ┌─────▼─────┐┌────▼─────┐┌────▼─────┐
        │ Laravel   ││ Laravel  ││ Laravel  │
        │ App #1    ││ App #2   ││ App #3   │
        └─────┬─────┘└────┬─────┘└────┬─────┘
              │            │            │
        ┌─────▼────────────▼────────────▼─────┐
        │          Redis (Session/Queue/Cache) │
        └─────────────────┬───────────────────┘
                          │
        ┌─────────────────▼───────────────────┐
        │          MySQL (主从)                 │
        │          Primary + Replica           │
        └─────────────────────────────────────┘
```

### 10.2 服务器配置建议

| 组件 | 最低配置 | 推荐配置 |
|------|---------|---------|
| Laravel API | 2C4G | 4C8G (可水平扩展) |
| MySQL | 2C4G + 50GB SSD | 4C8G + 100GB SSD |
| Redis | 1C2G | 2C4G |
| Queue Worker | 1C2G | 2C4G |
| Scheduler | 1C1G | 1C2G |

### 10.3 Node Agent VPS 配置

| 用户规模 | CPU | 内存 | 带宽 | 推荐协议 |
|---------|-----|------|------|---------|
| <500 | 2C | 2G | 500Mbps | Hysteria2 |
| 500-2000 | 4C | 4G | 1Gbps | Hysteria2 |
| 2000-5000 | 8C | 8G | 1Gbps | Hysteria2 + VLESS |
| >5000 | 16C | 16G | 10Gbps | 多协议负载均衡 |

---

## 11. 开发计划

### Phase 1 — MVP（4周）

```
Week 1: 基础搭建
├── Laravel 项目初始化 + 数据库迁移
├── 用户认证系统 (JWT + 设备绑定)
├── 节点管理 CRUD + NodeAgentClient
└── 套餐管理 CRUD

Week 2: 核心业务
├── SyncService 用户同步到节点
├── 连接密码签名 (NodePasswordService)
├── 流量上报与限额检查
└── 订阅管理 (购买/续费/过期)

Week 3: 客户端对接
├── iOS 客户端框架搭建 + sing-box 集成
├── Android 客户端框架搭建 + sing-box 集成
├── 登录 + 节点列表 + 一键连接
└── 实时网速 + 流量显示

Week 4: 测试与上线
├── 端到端测试
├── 支付集成 (Stripe)
├── 管理后台基础版
└── 部署上线
```

### Phase 2 — 完善（3周）

```
Week 5-6: 支付与通知
├── 支付宝集成
├── 推送通知 (FCM/APNs)
├── 到期提醒 + 流量告警
└── 邀请返利系统

Week 7: 管理后台
├── 数据仪表盘
├── 用户管理界面
├── 节点管理界面
└── 订单与流量统计
```

### Phase 3 — 优化（3周）

```
Week 8-9: 智能化
├── 智能节点推荐
├── 自动故障切换
├── 节点健康评分
└── 流量异常检测

Week 10: 增长功能
├── 工单系统
├── 多语言支持
├── 客户端自动更新
└── 数据导出/报表
```

### Phase 4 — 规模化（持续）

```
├── 节点自动扩容
├── 多区域负载均衡
├── 高可用架构
├── 数据分析平台
└── 运维自动化
```

---

## 附录

### A. Laravel 推荐包

| 包名 | 用途 |
|------|------|
| `tymon/jwt-auth` | JWT 认证 |
| `spatie/laravel-permission` | 角色权限 |
| `laravel/horizon` | 队列监控 |
| `barryvdh/laravel-ide-helper` | IDE 支持 |
| `laravel/telescope` | 调试工具(仅开发环境) |
| `spatie/laravel-activitylog` | 操作日志 |
| `maatwebsite/excel` | Excel 导出 |
| `torann/geoip` | IP 地理定位 |

### B. Node Agent API 对照表

| Laravel 操作 | Node Agent API | 说明 |
|-------------|---------------|------|
| 同步用户配置 | POST /deploy | 部署协议+用户配置 |
| 查看节点状态 | GET /status | CPU/内存/连接/流量 |
| 查看统计 | GET /stats | 详细流量+速度 |
| 查看在线用户 | GET /online | 在线用户详情 |
| 重启 sing-box | POST /restart | 重启服务 |
| 停止 sing-box | POST /stop | 停止服务 |
| 启动 sing-box | POST /start | 启动服务 |
| 查看日志 | GET /logs | 最近日志 |
| 健康检查 | GET /health | 存活检查 |

### C. 客户端配置组装模板

#### Hysteria2 outbound

```json
{
  "type": "hysteria2",
  "tag": "proxy",
  "server": "{server}",
  "server_port": {port},
  "password": "{password}",
  "tls": {
    "enabled": true,
    "server_name": "{sni}",
    "insecure": {insecure}
  }
}
```

#### VLESS + Reality outbound

```json
{
  "type": "vless",
  "tag": "proxy",
  "server": "{server}",
  "server_port": {port},
  "uuid": "{uuid}",
  "flow": "xtls-rprx-vision",
  "tls": {
    "enabled": true,
    "server_name": "{sni}",
    "utls": { "enabled": true, "fingerprint": "chrome" },
    "reality": {
      "enabled": true,
      "public_key": "{reality_public_key}",
      "short_id": "{reality_short_id}"
    }
  }
}
```

#### VLESS + WebSocket outbound

```json
{
  "type": "vless",
  "tag": "proxy",
  "server": "{server}",
  "server_port": {port},
  "uuid": "{uuid}",
  "tls": {
    "enabled": true,
    "server_name": "{sni}"
  },
  "transport": {
    "type": "ws",
    "path": "{ws_path}",
    "headers": { "Host": "{ws_host}" }
  }
}
```

#### Trojan outbound

```json
{
  "type": "trojan",
  "tag": "proxy",
  "server": "{server}",
  "server_port": {port},
  "password": "{password}",
  "tls": {
    "enabled": true,
    "server_name": "{sni}",
    "insecure": {insecure}
  }
}
```