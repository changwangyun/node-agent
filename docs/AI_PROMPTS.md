# AI 编程提示词 — VPN 商业平台 MVP

---

## 提示词 1: Laravel + Inertia + Vue 后端

```
你是一名资深 Laravel 全栈工程师，精通 Laravel 11 + Inertia.js + Vue 3 + TypeScript。

请帮我构建一个 VPN 商业平台的 MVP 版本，这是控制面板后端 + 管理后台。

## 项目背景

这是一个商业 VPN 平台，由三部分组成：
1. Laravel 控制面板（本项目）— 提供 API 给客户端 + 管理后台
2. Node Agent（已开发完成）— 部署在各 VPS 节点上，管理 sing-box 进程
3. 自有客户端 App — iOS/Android，内嵌 sing-box 引擎

Laravel 控制面板通过 HTTP API 与各节点 Node Agent 通信，实现用户配置同步、流量采集、节点监控。

## 技术栈

- Laravel 13
- Inertia.js + Vue 3 + TypeScript + Vite
- TailwindCSS 4
- MySQL 8
- Redis
- JWT 认证 (tymon/jwt-auth)
- Queue (Redis driver)

## 数据库设计

### users 表
```sql
CREATE TABLE users (
    id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    email VARCHAR(191) NOT NULL UNIQUE,
    password VARCHAR(255) NOT NULL,
    name VARCHAR(100) NOT NULL,
    phone VARCHAR(20) NULL,
    status ENUM('active','disabled','expired','limited') NOT NULL DEFAULT 'active',
    balance DECIMAL(10,2) NOT NULL DEFAULT 0.00,
    traffic_used BIGINT UNSIGNED NOT NULL DEFAULT 0,
    traffic_limit BIGINT UNSIGNED NOT NULL DEFAULT 0,
    speed_limit INT UNSIGNED NOT NULL DEFAULT 0,
    device_limit TINYINT UNSIGNED NOT NULL DEFAULT 3,
    uuid CHAR(36) NOT NULL UNIQUE,
    invite_code VARCHAR(50) NOT NULL UNIQUE,
    invited_by BIGINT UNSIGNED NULL,
    fcm_token VARCHAR(255) NULL,
    apns_token VARCHAR(255) NULL,
    last_login_at TIMESTAMP NULL,
    last_login_ip VARCHAR(45) NULL,
    created_at TIMESTAMP NULL,
    updated_at TIMESTAMP NULL
);
```

### user_devices 表
```sql
CREATE TABLE user_devices (
    id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    user_id BIGINT UNSIGNED NOT NULL,
    device_name VARCHAR(100) NOT NULL,
    device_id VARCHAR(255) NOT NULL,
    platform VARCHAR(50) NOT NULL,
    app_version VARCHAR(20) NULL,
    last_ip VARCHAR(45) NULL,
    last_online_at TIMESTAMP NULL,
    created_at TIMESTAMP NULL,
    updated_at TIMESTAMP NULL,
    UNIQUE KEY uk_user_device (user_id, device_id)
);
```

### nodes 表
```sql
CREATE TABLE nodes (
    id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    name VARCHAR(100) NOT NULL,
    code VARCHAR(50) NOT NULL UNIQUE,
    region VARCHAR(50) NOT NULL,
    country CHAR(2) NOT NULL,
    flag_emoji VARCHAR(10) NOT NULL DEFAULT '',
    server VARCHAR(255) NOT NULL,
    port INT UNSIGNED NOT NULL,
    protocol VARCHAR(50) NOT NULL DEFAULT 'hy2',
    api_url VARCHAR(255) NOT NULL,
    api_token VARCHAR(255) NOT NULL,
    password_secret VARCHAR(255) NOT NULL,
    status ENUM('online','offline','maintenance') NOT NULL DEFAULT 'offline',
    is_visible BOOLEAN NOT NULL DEFAULT TRUE,
    sort_order INT NOT NULL DEFAULT 0,
    max_users INT UNSIGNED NOT NULL DEFAULT 0,
    current_users INT UNSIGNED NOT NULL DEFAULT 0,
    health_score TINYINT UNSIGNED NOT NULL DEFAULT 100,
    avg_latency INT UNSIGNED NOT NULL DEFAULT 0,
    last_heartbeat_at TIMESTAMP NULL,
    created_at TIMESTAMP NULL,
    updated_at TIMESTAMP NULL
);
```

### node_protocols 表
```sql
CREATE TABLE node_protocols (
    id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    node_id BIGINT UNSIGNED NOT NULL,
    protocol VARCHAR(50) NOT NULL,
    port INT UNSIGNED NOT NULL,
    config JSON NOT NULL,
    is_default BOOLEAN NOT NULL DEFAULT FALSE,
    sort_order INT NOT NULL DEFAULT 0,
    created_at TIMESTAMP NULL,
    updated_at TIMESTAMP NULL
);
-- config 示例:
-- hy2: {"sni":"example.com","insecure":false}
-- vless-reality: {"sni":"www.microsoft.com","reality_public_key":"xxx","reality_short_id":"xxx","flow":"xtls-rprx-vision"}
```

### plans 表
```sql
CREATE TABLE plans (
    id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    name VARCHAR(100) NOT NULL,
    code VARCHAR(50) NOT NULL UNIQUE,
    description TEXT NULL,
    price_monthly DECIMAL(10,2) NOT NULL,
    price_quarterly DECIMAL(10,2) NULL,
    price_yearly DECIMAL(10,2) NULL,
    traffic_limit BIGINT UNSIGNED NOT NULL DEFAULT 0,
    speed_limit INT UNSIGNED NOT NULL DEFAULT 0,
    device_limit TINYINT UNSIGNED NOT NULL DEFAULT 3,
    features JSON NULL,
    is_visible BOOLEAN NOT NULL DEFAULT TRUE,
    sort_order INT NOT NULL DEFAULT 0,
    created_at TIMESTAMP NULL,
    updated_at TIMESTAMP NULL
);
```

### node_plan 表
```sql
CREATE TABLE node_plan (
    node_id BIGINT UNSIGNED NOT NULL,
    plan_id BIGINT UNSIGNED NOT NULL,
    PRIMARY KEY (node_id, plan_id)
);
```

### subscriptions 表
```sql
CREATE TABLE subscriptions (
    id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    user_id BIGINT UNSIGNED NOT NULL,
    plan_id BIGINT UNSIGNED NOT NULL,
    order_no VARCHAR(64) NOT NULL UNIQUE,
    type ENUM('monthly','quarterly','yearly') NOT NULL,
    price DECIMAL(10,2) NOT NULL,
    payment_method VARCHAR(50) NULL,
    payment_status ENUM('pending','paid','refunded') NOT NULL DEFAULT 'pending',
    payment_no VARCHAR(255) NULL,
    paid_at TIMESTAMP NULL,
    started_at TIMESTAMP NULL,
    expired_at TIMESTAMP NULL,
    status ENUM('active','expired','cancelled') NOT NULL DEFAULT 'active',
    traffic_used BIGINT UNSIGNED NOT NULL DEFAULT 0,
    traffic_limit BIGINT UNSIGNED NOT NULL DEFAULT 0,
    auto_renew BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMP NULL,
    updated_at TIMESTAMP NULL
);
```

### traffic_logs 表
```sql
CREATE TABLE traffic_logs (
    id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    user_id BIGINT UNSIGNED NOT NULL,
    node_id BIGINT UNSIGNED NOT NULL,
    upload BIGINT UNSIGNED NOT NULL DEFAULT 0,
    download BIGINT UNSIGNED NOT NULL DEFAULT 0,
    date DATE NOT NULL,
    created_at TIMESTAMP NULL,
    updated_at TIMESTAMP NULL,
    UNIQUE KEY uk_user_node_date (user_id, node_id, date)
);
```

### node_heartbeats 表
```sql
CREATE TABLE node_heartbeats (
    id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    node_id BIGINT UNSIGNED NOT NULL,
    cpu_percent FLOAT UNSIGNED NOT NULL DEFAULT 0,
    mem_percent FLOAT UNSIGNED NOT NULL DEFAULT 0,
    active_connections INT UNSIGNED NOT NULL DEFAULT 0,
    online_users INT UNSIGNED NOT NULL DEFAULT 0,
    upload_speed BIGINT UNSIGNED NOT NULL DEFAULT 0,
    download_speed BIGINT UNSIGNED NOT NULL DEFAULT 0,
    singbox_running BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMP NULL
);
```

### settings 表
```sql
CREATE TABLE settings (
    `key` VARCHAR(100) NOT NULL PRIMARY KEY,
    value TEXT NULL,
    updated_at TIMESTAMP NULL
);
```

## MVP 功能范围

### 客户端 API（给手机 App 调用）

1. **认证**: 注册/登录/刷新Token/登出
2. **用户**: 获取用户信息+订阅状态+流量概览
3. **节点**: 获取可用节点列表（含协议参数和签名密码）
4. **连接**: 连接/断开（记录连接日志）
5. **流量**: 上报流量/查询流量概览
6. **套餐**: 查看套餐列表/当前订阅
7. **设备**: 查看在线设备/踢下线

### 管理后台（Inertia + Vue）

1. **仪表盘**: 用户数/收入/节点状态/今日流量
2. **用户管理**: 列表/搜索/编辑/封禁/重置流量
3. **节点管理**: 列表/添加/编辑/删除/查看状态/重启/手动同步
4. **套餐管理**: 列表/添加/编辑/关联节点
5. **订阅管理**: 列表/查看/手动调整
6. **系统设置**: 站点配置/注册模式

## ⚠️ 最重要：Laravel 如何对接 Node Agent

Node Agent 是一个 Go 程序，部署在每台 VPS 节点上，监听 HTTP 端口（默认 8080）。
Laravel 通过 HTTP 请求与 Node Agent 通信，所有请求必须携带 `X-Node-Token` 请求头。

### Node Agent 完整 API 列表

| 方法 | 路径 | 认证 | 说明 |
|------|------|------|------|
| GET | /health | ❌ 不需要 | 健康检查，返回 `ok` |
| POST | /deploy | ✅ 需要 | 部署协议配置（最核心） |
| GET | /status | ✅ 需要 | 节点完整状态 |
| GET | /stats | ✅ 需要 | 流量+速度+连接+在线用户 |
| GET | /online | ✅ 需要 | 在线用户列表 |
| GET | /heartbeat | ✅ 需要 | 心跳数据（Node Agent 主动上报用） |
| POST | /restart | ✅ 需要 | 重启 sing-box |
| POST | /stop | ✅ 需要 | 停止 sing-box |
| POST | /start | ✅ 需要 | 启动 sing-box |
| POST | /device/register | ✅ 需要 | 注册设备+设备数检查 |
| POST | /session/acquire | ✅ 需要 | 获取并发会话 |
| POST | /session/release | ✅ 需要 | 释放并发会话 |
| GET | /logs | ✅ 需要 | 获取最近日志 |
| GET | /traffic/user | ✅ 需要 | 按用户查询流量(V2Ray API) |
| GET | /client-config | ✅ 需要 | 获取客户端配置 |

### 认证方式

所有需要认证的接口，请求头必须带：
```
X-Node-Token: {api_token}
```
也可以通过 URL 参数传递：`?token={api_token}`
`api_token` 在 Node Agent 的配置文件 `/etc/node-agent/config.json` 中设置。

### 核心接口详解

#### 1. POST /deploy — 部署协议配置（最核心最复杂）

Laravel 调用此接口将用户的协议配置推送到节点，Node Agent 会：
1. 生成 sing-box 服务端配置
2. 写入 /etc/sing-box/config.json
3. 自动重启 sing-box 使配置生效
4. 返回客户端连接配置

**请求体：**
```json
{
  "user_id": "user_123",
  "node_id": "node_us_01",
  "protocol": "hysteria2",
  "server": "0.0.0.0",
  "port": 443,
  "password": "base64签名密码",
  "uuid": "仅vless/reality需要",
  "sni": "example.com",
  "obfs_type": "salamander",
  "obfs_password": "obfs密码",
  "up_mbps": 100,
  "down_mbps": 200,
  "tls_cert_path": "",
  "tls_key_path": "",
  "acme_domain": "",
  "acme_email": "",
  "reality_private_key": "仅reality需要",
  "reality_public_key": "仅reality需要",
  "reality_short_id": "仅reality需要",
  "reality_dest": "www.microsoft.com",
  "reality_dest_port": 443
}
```

**各协议必填字段：**

| 字段 | hysteria2 | vless | reality |
|------|-----------|-------|---------|
| user_id | ✅ | ✅ | ✅ |
| node_id | ✅ | ✅ | ✅ |
| protocol | "hysteria2" | "vless" | "reality" |
| server | "0.0.0.0" | "0.0.0.0" | "0.0.0.0" |
| port | ✅ | ✅ | ✅ |
| password | ✅ 连接密码 | ❌ | ❌ |
| uuid | ❌ | ✅ 用户UUID | ✅ 用户UUID |
| sni | ✅ | ✅ | ✅ |
| obfs_type | 可选 | ❌ | ❌ |
| obfs_password | 可选 | ❌ | ❌ |
| up_mbps | 可选 | ❌ | ❌ |
| down_mbps | 可选 | ❌ | ❌ |
| tls_cert_path | 可选(否则自签) | 可选(否则自签) | ❌ |
| tls_key_path | 可选(否则自签) | 可选(否则自签) | ❌ |
| acme_domain | 可选 | 可选 | ❌ |
| acme_email | 可选 | 可选 | ❌ |
| reality_private_key | ❌ | ❌ | ✅ 必须 |
| reality_public_key | ❌ | ❌ | ✅ 必须 |
| reality_short_id | ❌ | ❌ | ✅ 必须 |
| reality_dest | ❌ | ❌ | 可选(默认www.microsoft.com) |
| reality_dest_port | ❌ | ❌ | 可选(默认443) |

**成功响应：**
```json
{
  "success": true,
  "message": "deployed hysteria2 config for user user_123",
  "node_id": "node_us_01",
  "client_config": {
    "singbox_config": "{ 完整的sing-box客户端JSON配置 }",
    "uri": "hysteria2://password@server:443?sni=example.com#hy2-node_us_01",
    "protocol": "hysteria2",
    "server": "1.2.3.4",
    "port": 443,
    "insecure": true
  }
}
```

**⚠️ 重要：deploy 是追加式部署，同一协议+端口的多个用户会自动合并到同一个 inbound 的 users 列表中。**
例如先部署 user-001（hysteria2, port 443），再部署 user-002（hysteria2, port 443），最终 inbound 会包含两个用户。
不同协议或不同端口的用户会生成独立的 inbound。

#### 2. GET /status — 节点完整状态

**响应：**
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
    "cpu_percent": 15.2,
    "mem_percent": 45.3,
    "mem_used_mb": 1800,
    "mem_total_mb": 4000,
    "disk_used_gb": 12,
    "disk_total_gb": 40,
    "load_1": 0.5,
    "load_5": 0.3,
    "load_15": 0.2
  },
  "traffic": {
    "upload": 1073741824,
    "download": 5368709120
  },
  "speed": {
    "upload": 1048576,
    "download": 5242880
  },
  "connections": {
    "active": 42
  },
  "devices": {
    "online_users": 5,
    "total_devices": 12
  },
  "online_details": [
    {
      "user_id": "user-001",
      "inbound": "hysteria2-in",
      "ip": "203.0.113.50",
      "upload": 1048576,
      "download": 5242880
    }
  ]
}
```

#### 3. GET /stats — 流量统计

**响应：**
```json
{
  "traffic": { "upload": 1073741824, "download": 5368709120 },
  "speed": { "upload": 1048576, "download": 5242880 },
  "connections": { "active": 42 },
  "online_users": 5,
  "online_details": [...],
  "user_traffic": [
    { "user_id": "user_123", "upload": 524288, "download": 2097152 }
  ]
}
```

#### 4. GET /online — 在线用户

**响应：**
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

> **工作原理**：通过 Clash API `/connections` 接口获取活跃连接，解析 `inboundUser` 字段（即 Deploy 时设置的 `name`），按用户聚合。前提：sing-box 需使用 `-tags with_v2ray_api,with_quic,with_clash_api` 编译，且用户配置中必须包含 `name` 字段。

#### 5. GET /traffic/user — 按用户查询流量（V2Ray API）

查询参数：
- `user_id=xxx` — 查询单个用户
- 不带参数 — 查询所有用户

**响应（单个用户）：**
```json
{
  "user_id": "user_123",
  "upload": 524288,
  "download": 2097152
}
```

**响应（所有用户）：**
```json
{
  "users": [
    { "user_id": "user_123", "upload": 524288, "download": 2097152 },
    { "user_id": "user_456", "upload": 1024, "download": 4096 }
  ],
  "count": 2
}
```

#### 6. POST /device/register — 设备注册+限制检查

**请求：**
```json
{
  "user_id": "user_123",
  "device_id": "device_unique_id",
  "ip": "1.2.3.4"
}
```

**成功响应：**
```json
{ "allowed": true, "reason": "OK", "device_count": 2 }
```

**超限响应（HTTP 403）：**
```json
{ "allowed": false, "reason": "DEVICE_LIMIT_EXCEEDED", "error": "device limit exceeded: 3" }
```

#### 7. POST /session/acquire — 并发会话获取

**请求：**
```json
{ "user_id": "user_123" }
```

**成功响应：**
```json
{ "allowed": true, "reason": "OK", "concurrent_count": 1 }
```

**超限响应（HTTP 403）：**
```json
{ "allowed": false, "reason": "CONCURRENT_LIMIT_EXCEEDED", "error": "concurrent limit exceeded: 2" }
```

#### 8. POST /session/release — 释放并发会话

**请求：**
```json
{ "user_id": "user_123" }
```

**响应：**
```json
{ "success": true }
```

#### 9. GET /logs — 获取日志

查询参数：`lines=50`（默认50，最大200）

**响应：**
```json
{
  "lines": ["[singbox:err] INFO[0000] started", "..."],
  "count": 50
}
```

#### 10. POST /restart | /stop | /start — 服务控制

**响应：**
```json
{ "success": true, "message": "sing-box restarted" }
```

#### 11. GET /client-config — 获取客户端配置

查询参数：`user_id=xxx`

**响应：** 同 deploy 返回的 client_config 结构

#### 12. GET /health — 健康检查（无需认证）

**响应：** `ok`（纯文本，HTTP 200）

### Node Agent 心跳上报机制

Node Agent 会主动向 Laravel 发送心跳，Laravel 需要实现接收端点。

**Node Agent 发送：** POST `{control_plane.url}{control_plane.heartbeat_path}`（默认路径 `/api/node/heartbeat`，可通过 `heartbeat_path` 配置项修改）
**请求头：** `X-Node-Token: {control_plane.token}`, `X-Node-ID: {control_plane.node_id}`

**心跳请求体：**
```json
{
  "node_id": "node_us_01",
  "timestamp": 1706140800,
  "status": {
    "singbox_running": true,
    "singbox_state": "running",
    "uptime_seconds": 3600,
    "pid": 12345
  },
  "traffic": { "upload": 1073741824, "download": 5368709120 },
  "online": { "user_count": 5, "device_count": 12, "active_sessions": 5 },
  "system": {
    "cpu_percent": 15.2,
    "mem_percent": 45.3,
    "mem_used_mb": 1800,
    "mem_total_mb": 4000,
    "disk_used_gb": 12,
    "disk_total_gb": 40,
    "load_1": 0.5,
    "load_5": 0.3,
    "load_15": 0.2
  }
}
```

Laravel 接收心跳后应该：
1. 更新 nodes 表的 status、last_heartbeat_at、health_score
2. 写入 node_heartbeats 表
3. 更新 nodes 表的 current_users 字段

### Laravel NodeAgentClient 服务完整实现参考

```php
class NodeAgentClient
{
    public function __construct(
        private HttpClient $http,
    ) {}

    public function deploy(Node $node, array $payload): array
    {
        return $this->post($node, '/deploy', $payload);
    }

    public function status(Node $node): array
    {
        return $this->get($node, '/status');
    }

    public function stats(Node $node): array
    {
        return $this->get($node, '/stats');
    }

    public function online(Node $node): array
    {
        return $this->get($node, '/online');
    }

    public function restart(Node $node): array
    {
        return $this->post($node, '/restart');
    }

    public function stop(Node $node): array
    {
        return $this->post($node, '/stop');
    }

    public function start(Node $node): array
    {
        return $this->post($node, '/start');
    }

    public function registerDevice(Node $node, string $userId, string $deviceId, string $ip): array
    {
        return $this->post($node, '/device/register', [
            'user_id' => $userId,
            'device_id' => $deviceId,
            'ip' => $ip,
        ]);
    }

    public function acquireSession(Node $node, string $userId): array
    {
        return $this->post($node, '/session/acquire', ['user_id' => $userId]);
    }

    public function releaseSession(Node $node, string $userId): array
    {
        return $this->post($node, '/session/release', ['user_id' => $userId]);
    }

    public function logs(Node $node, int $lines = 50): array
    {
        return $this->get($node, '/logs?lines=' . $lines);
    }

    public function userTraffic(Node $node, ?string $userId = null): array
    {
        $path = '/traffic/user';
        if ($userId) {
            $path .= '?user_id=' . $userId;
        }
        return $this->get($node, $path);
    }

    public function clientConfig(Node $node, string $userId): array
    {
        return $this->get($node, '/client-config?user_id=' . $userId);
    }

    public function health(Node $node): bool
    {
        try {
            $response = $this->http->get($node->api_url . '/health');
            return $response->successful();
        } catch (\Throwable) {
            return false;
        }
    }

    private function get(Node $node, string $path): array
    {
        $response = $this->http->withHeaders([
            'X-Node-Token' => $node->api_token,
        ])->get(rtrim($node->api_url, '/') . $path);

        return $response->json();
    }

    private function post(Node $node, string $path, array $data = []): array
    {
        $response = $this->http->withHeaders([
            'X-Node-Token' => $node->api_token,
        ])->post(rtrim($node->api_url, '/') . $path, $data);

        return $response->json();
    }
}
```

### SyncService 核心逻辑

```php
class SyncService
{
    public function __construct(
        private NodeAgentClient $client,
    ) {}

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

        if (isset($protocol->config['acme_domain'])) {
            $payload['acme_domain'] = $protocol->config['acme_domain'];
            $payload['acme_email'] = $protocol->config['acme_email'] ?? '';
        }

        return $this->client->deploy($node, $payload);
    }

    public function removeUserFromNode(User $user, Node $node): void
    {
        // Node Agent 当前是覆盖式部署，移除用户需要重新部署不含该用户的配置
        // MVP 阶段可以简单调用 stop，后续实现批量部署后改为重新部署
        $this->client->stop($node);
    }

    public function restartNode(Node $node): array
    {
        return $this->client->restart($node);
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

### Laravel 接收心跳的路由和控制器

```php
// routes/api.php
Route::post('/node/heartbeat', [NodeHeartbeatController::class, 'receive']);

// NodeHeartbeatController.php
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
            'avg_latency' => 0,
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

        return response()->json(['success' => true]);
    }
}
```

### Node Agent 配置文件参考

Node Agent 安装在 VPS 后，配置文件位于 `/etc/node-agent/config.json`：

```json
{
  "node_id": "node_us_01",
  "api_port": 8080,
  "api_token": "与Laravel nodes表api_token一致",
  "log_level": "info",
  "data_dir": "/var/lib/node-agent",
  "singbox": {
    "binary_path": "/usr/local/bin/sing-box",
    "config_path": "/etc/sing-box/config.json",
    "work_dir": "/var/lib/sing-box",
    "clash_api_addr": "0.0.0.0:9090",
    "clash_api_secret": "node-agent-stats",
    "v2ray_api_addr": "127.0.0.1:10001"
  },
  "control_plane": {
    "url": "https://api.your-domain.com",
    "token": "与Laravel nodes表api_token一致",
    "node_id": "node_us_01",
    "timeout": 10
  },
  "device_limit": {
    "max_devices": 3,
    "max_concurrent": 2
  },
  "ip_whitelist": [],
  "cors": {
    "enabled": true,
    "allowed_origins": ["*"]
  },
  "heartbeat_interval": 60,
  "watchdog_interval": 30
}
```

### 完整业务流程图

```
用户注册 → 创建 user 记录(含uuid)
    ↓
用户购买套餐 → 创建 subscription 记录
    ↓
用户打开App → GET /nodes 获取节点列表
    ↓
用户选择节点 → POST /connection/connect
    ↓
Laravel 调用 Node Agent POST /deploy
    ├─ 生成签名密码
    ├─ 构建 deploy payload
    └─ Node Agent 生成配置+重启sing-box
    ↓
Laravel 返回节点参数+签名密码给App
    ↓
App 本地组装 sing-box 客户端配置
    ↓
App 启动 VPN 隧道连接
    ↓
App 每30秒 POST /traffic/report 上报流量
    ↓
Laravel 记录流量+检查限额
    ↓
超限 → Laravel 调用 Node Agent POST /stop 断开用户
    ↓
过期 → Laravel 调用 Node Agent 移除用户配置
```

## 核心业务逻辑

### NodeAgentClient 服务
与 Node Agent 通信的 HTTP 客户端，所有请求带 X-Node-Token 头，完整实现见上方参考代码。

### SyncService 服务（最核心）
- syncUserToNode(User, Node, protocolType): 将用户同步到指定节点，构建 deploy payload
- removeUserFromNode(User, Node): 从节点移除用户（当前MVP调用stop）
- restartNode(Node): 重启节点 sing-box
- generatePassword(User, Node): 生成签名密码 = base64(uuid.timestamp.hmac_sha256)

### NodePasswordService 服务
- generatePassword(User, Node): 生成签名密码 = base64(uuid.timestamp.hmac_sha256)
- 每个节点有独立的 password_secret（存在 nodes 表）
- 密码24小时有效
- 客户端连接时由 Laravel 生成，传入 sing-box 配置

### TrafficService 服务
- recordUserTraffic(User, nodeId, upload, download): 记录流量+检查限额
- checkTrafficLimit(User): 超限则标记limited+调用Node Agent stop+推送通知
- collectFromNodes(): 从所有节点 GET /stats 采集统计数据

## API 路由设计

```
# 客户端 API (api/v1/)
POST   /api/v1/auth/register
POST   /api/v1/auth/login          { email, password, device_id, device_name, platform }
POST   /api/v1/auth/refresh
POST   /api/v1/auth/logout
GET    /api/v1/user
PUT    /api/v1/user
POST   /api/v1/user/change-password
GET    /api/v1/nodes                返回可用节点+协议参数+签名密码
GET    /api/v1/nodes/recommended    智能推荐3个最优节点
POST   /api/v1/connection/connect   { node_id, protocol_type }
POST   /api/v1/connection/disconnect { node_id, upload, download, duration }
POST   /api/v1/traffic/report       { node_id, upload, download }
GET    /api/v1/traffic/summary
GET    /api/v1/plans
GET    /api/v1/subscription
GET    /api/v1/devices
DELETE /api/v1/devices/{id}

# 管理后台 (Inertia pages)
GET    /admin/dashboard
GET    /admin/users
GET    /admin/nodes
GET    /admin/plans
GET    /admin/subscriptions
GET    /admin/settings
```

## 响应格式

```json
{
  "code": 0,
  "message": "success",
  "data": { ... }
}
```

错误码: 0=成功, 1001=参数错误, 1002=未认证, 2001=订阅过期, 2002=流量超限, 2003=设备数超限, 3001=节点不可用

## 定时任务

```php
// 每5分钟: 采集节点统计
$schedule->command('nodes:collect-stats')->everyFiveMinutes();
// 每分钟: 检查过期订阅
$schedule->command('subscriptions:check-expired')->everyMinute();
// 每天0点: 重置月流量
$schedule->command('traffic:reset-monthly')->dailyAt('00:00');
// 每5分钟: 检查节点健康
$schedule->command('nodes:check-health')->everyFiveMinutes();
```

## 管理后台 UI 要求

- 使用 TailwindCSS，简洁专业风格
- 侧边栏导航: 仪表盘/用户/节点/套餐/订阅/设置
- 仪表盘: 4个统计卡片(总用户/活跃用户/今日收入/在线设备) + 节点状态表格
- 用户管理: 表格+搜索+编辑弹窗
- 节点管理: 表格+添加/编辑表单+状态指示灯+操作按钮(同步/重启)
- 套餐管理: 表格+添加/编辑表单+节点关联多选
- 设置: 表单，分站点/注册/支付分组

## 开发步骤

1. 创建 Laravel 项目，安装依赖 (inertia, vue, tailwind, jwt-auth)
2. 创建所有 Migration 和 Model（含关联关系）
3. 实现 NodeAgentClient 服务
4. 实现 SyncService 服务
5. 实现 NodePasswordService 服务
6. 实现 TrafficService 服务
7. 实现客户端 API Controllers + Routes
8. 实现管理后台 Inertia Pages
9. 实现定时任务 Commands
10. 编写 Seeder（测试数据）

请按步骤逐一实现，每步完成后确认再继续。代码不要加注释。
```

---

## 提示词 2: iOS 客户端 MVP

```
你是一名资深 iOS 开发工程师，精通 Swift + SwiftUI + NetworkExtension。

请帮我构建一个 VPN 客户端 App 的 MVP 版本，内嵌 sing-box 引擎实现代理。

## 项目背景

这是一个商业 VPN 平台的客户端，用户通过 App 一键连接 VPN。
后端 Laravel API 已开发完成，提供用户认证、节点列表、流量上报等接口。
sing-box 官方提供 iOS Framework：https://github.com/SagerNet/sing-box/tree/dev/platform

## 技术栈

- Swift 5.9+
- SwiftUI
- NetworkExtension (NEPacketTunnelProvider)
- sing-box iOS Framework (Mobile Library)
- KeychainAccess (安全存储)
- Alamofire (HTTP 请求)

## MVP 功能范围

1. **登录/注册**: 邮箱+密码登录，JWT Token 存 Keychain
2. **首页**: 一键连接按钮 + 连接状态 + 实时网速 + 流量用量环形图
3. **节点列表**: 国旗+名称+延迟+负载，点击选择
4. **我的**: 用户信息+订阅状态+设备管理+退出登录

## API 接口

```
Base URL: https://api.your-domain.com/api/v1

POST /auth/register    { email, password, name }
POST /auth/login       { email, password, device_id, device_name, platform }
                       → { user, token }
POST /auth/refresh     → { token }

GET  /user             → { user, subscription, traffic_summary }
GET  /nodes            → { nodes: [{ id, name, country, flag, protocols: [...], load, latency }] }
GET  /nodes/recommended → { nodes: [...top3] }

POST /connection/connect   { node_id, protocol_type }
     → { node, password, config_params, expires_at }
POST /connection/disconnect { node_id, upload, download, duration }

POST /traffic/report   { node_id, upload, download }
     → { traffic_used, traffic_limit, is_limited }
GET  /traffic/summary  → { today, month, history }

GET  /devices          → { devices: [...] }
DELETE /devices/{id}

GET  /plans            → { plans: [...] }
GET  /subscription     → { subscription, plan }
```

## 核心架构

### App 结构

```
VPNApp
├── App/VPNApp.swift              -- @main entry
├── Views/
│   ├── LoginView.swift           -- 登录/注册
│   ├── HomeView.swift            -- 首页(一键连接)
│   ├── NodesView.swift           -- 节点列表
│   ├── ProfileView.swift         -- 我的
│   └── Components/
│       ├── ConnectButton.swift   -- 连接按钮(大圆按钮+动画)
│       ├── SpeedIndicator.swift  -- 实时网速显示
│       ├── TrafficRing.swift     -- 流量环形图
│       └── NodeRow.swift         -- 节点行
├── ViewModels/
│   ├── AuthViewModel.swift       -- 认证逻辑
│   ├── HomeViewModel.swift       -- 首页逻辑
│   ├── NodesViewModel.swift      -- 节点逻辑
│   └── ProfileViewModel.swift    -- 个人逻辑
├── Services/
│   ├── APIService.swift          -- HTTP 请求封装
│   ├── VPNManager.swift          -- VPN 连接管理
│   ├── ConfigBuilder.swift       -- sing-box 配置组装
│   └── TrafficMonitor.swift      -- 流量监控
├── Models/
│   ├── User.swift
│   ├── Node.swift
│   ├── Subscription.swift
│   └── TrafficSummary.swift
└── PacketTunnel/                  -- NetworkExtension Target
    └── PacketTunnelProvider.swift  -- VPN 隧道实现
```

### 连接流程

```
1. 用户选择节点 → 点击连接
2. 调用 POST /connection/connect 获取签名密码+节点参数
3. ConfigBuilder 本地组装 sing-box 配置:
   - inbounds: [{ type: "tun", ... }]
   - outbounds: [{ type: "hy2"/"vless", server, port, password, ... }]
   - route: { rules: [...], final: "proxy" }
4. 将配置传给 VPNManager → 启动 NEPacketTunnelProvider
5. PacketTunnelProvider 使用 sing-box Mobile Library 启动代理
6. TrafficMonitor 每2秒读取本地流量统计
7. 每30秒调用 POST /traffic/report 上报流量
8. 如果返回 is_limited=true → 自动断开
```

### sing-box 配置组装

Laravel API 返回节点参数，客户端本地组装完整配置：

```swift
func buildConfig(node: Node, protocol: NodeProtocol, password: String) -> [String: Any] {
    return [
        "log": ["level": "warn"],
        "dns": [
            "servers": [
                ["tag": "google", "type": "tls", "server": "8.8.8.8"],
                ["tag": "local", "type": "udp", "server": "223.5.5.5"]
            ]
        ],
        "inbounds": [[
            "type": "tun",
            "tag": "tun-in",
            "address": ["172.19.0.1/30", "fdfe:dcba:9876::1/126"],
            "mtu": 9000,
            "auto_route": true,
            "strict_route": true
        ]],
        "outbounds": [
            buildOutbound(protocol: protocol, password: password),
            ["type": "direct", "tag": "direct"],
            ["type": "block", "tag": "block"],
            ["type": "dns", "tag": "dns-out"]
        ],
        "route": [
            "rules": [
                ["action": "sniff"],
                ["protocol": ["dns"], "action": "hijack-dns"]
            ],
            "default_domain_resolver": "google",
            "final": "proxy"
        ]
    ]
}
```

### Hysteria2 outbound 组装

```swift
func buildHysteria2Outbound(server: String, port: Int, password: String, sni: String) -> [String: Any] {
    return [
        "type": "hysteria2",
        "tag": "proxy",
        "server": server,
        "server_port": port,
        "password": password,
        "tls": [
            "enabled": true,
            "server_name": sni
        ]
    ]
}
```

### VLESS+Reality outbound 组装

```swift
func buildVlessRealityOutbound(server: String, port: Int, uuid: String, sni: String, publicKey: String, shortId: String) -> [String: Any] {
    return [
        "type": "vless",
        "tag": "proxy",
        "server": server,
        "server_port": port,
        "uuid": uuid,
        "flow": "xtls-rprx-vision",
        "tls": [
            "enabled": true,
            "server_name": sni,
            "utls": ["enabled": true, "fingerprint": "chrome"],
            "reality": [
                "enabled": true,
                "public_key": publicKey,
                "short_id": shortId
            ]
        ]
    ]
}
```

## UI 设计要求

### 首页 (HomeView)
- 顶部: 用户头像+名称+订阅状态标签
- 中间: 大圆连接按钮（未连接灰色/连接中旋转动画/已连接绿色）
- 连接按钮下方: 当前节点名称+国旗
- 下方左: 上传速度 ↑ XX MB/s
- 下方右: 下载速度 ↓ XX MB/s
- 底部: 流量环形图（已用/总量）+ 百分比

### 节点列表 (NodesView)
- 顶部: 搜索栏
- 推荐: 智能推荐3个节点（星标）
- 全部: 按地区分组
- 每行: 国旗emoji + 名称 + 延迟ms + 负载条 + 选中指示

### 登录页 (LoginView)
- Logo + App名称
- 邮箱输入框
- 密码输入框
- 登录按钮
- 注册链接

## 开发步骤

1. 创建 Xcode 项目 (SwiftUI App + NetworkExtension Target)
2. 集成 sing-box Mobile Library (SPM)
3. 实现 APIService (HTTP 请求 + JWT 管理)
4. 实现 Models
5. 实现 AuthViewModel + LoginView
6. 实现 VPNManager + ConfigBuilder + PacketTunnelProvider
7. 实现 HomeViewModel + HomeView (连接按钮+网速+流量)
8. 实现 NodesViewModel + NodesView
9. 实现 ProfileViewModel + ProfileView
10. 实现 TrafficMonitor (流量上报)

请按步骤逐一实现，每步完成后确认再继续。代码不要加注释。
```

---

## 提示词 3: Android 客户端 MVP

```
你是一名资深 Android 开发工程师，精通 Kotlin + Jetpack Compose + VpnService。

请帮我构建一个 VPN 客户端 App 的 MVP 版本，内嵌 sing-box 引擎实现代理。

## 项目背景

这是一个商业 VPN 平台的客户端，用户通过 App 一键连接 VPN。
后端 Laravel API 已开发完成，提供用户认证、节点列表、流量上报等接口。
sing-box 官方提供 Android AAR Library：https://github.com/SagerNet/sing-box/tree/dev/platform

## 技术栈

- Kotlin 2.0+
- Jetpack Compose + Material 3
- sing-box AAR Library (Mobile Library)
- Retrofit + OkHttp (HTTP 请求)
- DataStore (安全存储)
- Hilt (依赖注入)
- VpnService (Android VPN 接口)

## MVP 功能范围

同 iOS 客户端：登录/注册、首页一键连接、节点列表、我的

## API 接口

同 iOS 客户端，完全一致

## 核心架构

### App 结构

```
app/
├── data/
│   ├── api/ApiService.kt           -- Retrofit 接口定义
│   ├── model/                      -- 数据模型
│   │   ├── User.kt
│   │   ├── Node.kt
│   │   ├── Subscription.kt
│   │   └── TrafficSummary.kt
│   ├── repository/
│   │   ├── AuthRepository.kt
│   │   ├── NodeRepository.kt
│   │   └── TrafficRepository.kt
│   └── local/TokenManager.kt       -- DataStore JWT管理
├── ui/
│   ├── navigation/NavGraph.kt
│   ├── screen/
│   │   ├── login/LoginScreen.kt
│   │   ├── home/HomeScreen.kt
│   │   ├── nodes/NodesScreen.kt
│   │   └── profile/ProfileScreen.kt
│   ├── component/
│   │   ├── ConnectButton.kt
│   │   ├── SpeedIndicator.kt
│   │   ├── TrafficRing.kt
│   │   └── NodeItem.kt
│   └── viewmodel/
│       ├── AuthViewModel.kt
│       ├── HomeViewModel.kt
│       ├── NodesViewModel.kt
│       └── ProfileViewModel.kt
├── service/
│   ├── VpnService.kt               -- Android VpnService 实现
│   ├── SingboxManager.kt           -- sing-box 生命周期管理
│   └── TrafficMonitor.kt           -- 流量监控+上报
├── util/
│   └── ConfigBuilder.kt            -- sing-box 配置组装
├── di/AppModule.kt                 -- Hilt 模块
└── MainActivity.kt
```

### 连接流程

同 iOS 客户端，完全一致：
1. 选择节点 → 调 API 获取签名密码
2. ConfigBuilder 组装 sing-box 配置
3. 启动 VpnService → sing-box AAR 启动代理
4. TrafficMonitor 每30秒上报流量
5. is_limited=true → 自动断开

### sing-box 配置组装

同 iOS 客户端，配置 JSON 格式完全一致

### VpnService 实现

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
            .establish()?.fd ?: return START_NOT_STICKY

        // sing-box AAR 启动
        singboxProcess = Singbox.start(config, fd)

        return START_STICKY
    }

    override fun onDestroy() {
        Singbox.stop(singboxProcess)
        super.onDestroy()
    }
}
```

## UI 设计要求

同 iOS 客户端，Material 3 风格：
- 首页: 大圆连接按钮 + 网速 + 流量环形图
- 节点列表: 搜索+推荐+分组
- 登录: 简洁表单

## 开发步骤

1. 创建 Android 项目 (Kotlin + Compose + Hilt)
2. 集成 sing-box AAR Library
3. 实现 Retrofit ApiService + TokenManager
4. 实现 Models + Repository
5. 实现 AuthViewModel + LoginScreen
6. 实现 VpnService + SingboxManager + ConfigBuilder
7. 实现 HomeViewModel + HomeScreen
8. 实现 NodesViewModel + NodesScreen
9. 实现 ProfileViewModel + ProfileScreen
10. 实现 TrafficMonitor

请按步骤逐一实现，每步完成后确认再继续。代码不要加注释。
```

---

## 使用建议

1. **先执行提示词 1**（Laravel 后端），完成后端后再开发客户端
2. 每个提示词都是分步骤的，AI 每完成一步你确认后再继续下一步
3. 如果 AI 某步输出不完整，发送「继续」让它补全
4. Laravel 后端完成后，用 Postman 测试 API 确认正常，再开始客户端开发
5. iOS 和 Android 客户端可以并行开发，API 完全一致
6. 开发过程中遇到问题，把错误信息发给 AI 让它修复
