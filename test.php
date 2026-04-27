<?php
$apiBase = $_POST['api_base'] ?? 'http://127.0.0.1:8080';
$apiToken = $_POST['api_token'] ?? 'change-me-in-production';
$result = null;
$error = null;
$activeTab = $_POST['active_tab'] ?? 'status';

function apiRequest($url, $token, $method = 'GET', $body = null) {
    $ch = curl_init();
    curl_setopt_array($ch, [
        CURLOPT_URL            => $url,
        CURLOPT_RETURNTRANSFER => true,
        CURLOPT_TIMEOUT        => 10,
        CURLOPT_HTTPHEADER     => [
            'X-Node-Token: ' . $token,
            'Content-Type: application/json',
        ],
        CURLOPT_CUSTOMREQUEST  => $method,
    ]);
    if ($body !== null) {
        curl_setopt($ch, CURLOPT_POSTFIELDS, json_encode($body));
    }
    $resp = curl_exec($ch);
    $httpCode = curl_getinfo($ch, CURLINFO_HTTP_CODE);
    $err = curl_error($ch);
    curl_close($ch);
    if ($err) return ['error' => $err, 'http_code' => 0];
    $decoded = json_decode($resp, true);
    return [
        'http_code' => $httpCode,
        'body'      => $decoded !== null ? $decoded : $resp,
        'raw'       => $resp,
    ];
}

if ($_SERVER['REQUEST_METHOD'] === 'POST' && isset($_POST['action'])) {
    $action = $_POST['action'];
    switch ($action) {
        case 'health':
            $result = apiRequest("{$apiBase}/health", $apiToken, 'GET');
            break;
        case 'status':
            $result = apiRequest("{$apiBase}/status", $apiToken, 'GET');
            break;
        case 'stats':
            $result = apiRequest("{$apiBase}/stats", $apiToken, 'GET');
            break;
        case 'heartbeat':
            $result = apiRequest("{$apiBase}/heartbeat", $apiToken, 'GET');
            break;
        case 'start':
            $result = apiRequest("{$apiBase}/start", $apiToken, 'POST');
            break;
        case 'stop':
            $result = apiRequest("{$apiBase}/stop", $apiToken, 'POST');
            break;
        case 'restart':
            $result = apiRequest("{$apiBase}/restart", $apiToken, 'POST');
            break;
        case 'deploy':
            $deployBody = array_filter([
                'user_id'            => $_POST['deploy_user_id'] ?? '',
                'node_id'            => $_POST['deploy_node_id'] ?? '',
                'protocol'           => $_POST['deploy_protocol'] ?? 'hysteria2',
                'server'             => $_POST['deploy_server'] ?? '',
                'port'               => intval($_POST['deploy_port'] ?? 0),
                'password'           => $_POST['deploy_password'] ?? '',
                'uuid'               => $_POST['deploy_uuid'] ?? '',
                'sni'                => $_POST['deploy_sni'] ?? '',
                'reality_public_key' => $_POST['deploy_reality_pubkey'] ?? '',
                'reality_short_id'   => $_POST['deploy_reality_shortid'] ?? '',
                'inbound_type'       => $_POST['deploy_inbound_type'] ?? 'tun',
            ], function($v) { return $v !== '' && $v !== 0; });
            $result = apiRequest("{$apiBase}/deploy", $apiToken, 'POST', $deployBody);
            $activeTab = 'deploy';
            break;
        case 'device_register':
            $result = apiRequest("{$apiBase}/device/register", $apiToken, 'POST', [
                'user_id'   => $_POST['device_user_id'] ?? '',
                'device_id' => $_POST['device_device_id'] ?? '',
                'ip'        => $_POST['device_ip'] ?? '',
            ]);
            $activeTab = 'device';
            break;
        case 'session_acquire':
            $result = apiRequest("{$apiBase}/session/acquire", $apiToken, 'POST', [
                'user_id' => $_POST['session_user_id'] ?? '',
            ]);
            $activeTab = 'device';
            break;
        case 'session_release':
            $result = apiRequest("{$apiBase}/session/release", $apiToken, 'POST', [
                'user_id' => $_POST['session_user_id'] ?? '',
            ]);
            $activeTab = 'device';
            break;
    }
}
?>
<!DOCTYPE html>
<html lang="zh-CN">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<title>Node Agent API 测试面板</title>
<style>
*{margin:0;padding:0;box-sizing:border-box}
body{font-family:-apple-system,BlinkMacSystemFont,"Segoe UI",Roboto,sans-serif;background:#0f172a;color:#e2e8f0;min-height:100vh}
.header{background:linear-gradient(135deg,#1e293b 0%,#0f172a 100%);border-bottom:1px solid #334155;padding:20px 32px;display:flex;align-items:center;justify-content:space-between}
.header h1{font-size:22px;font-weight:700;background:linear-gradient(90deg,#38bdf8,#818cf8);-webkit-background-clip:text;-webkit-text-fill-color:transparent}
.header .badge{font-size:11px;background:#22c55e;color:#fff;padding:3px 10px;border-radius:20px;font-weight:600}
.config-bar{background:#1e293b;border-bottom:1px solid #334155;padding:14px 32px;display:flex;gap:16px;align-items:center;flex-wrap:wrap}
.config-bar label{font-size:13px;color:#94a3b8;font-weight:500}
.config-bar input{background:#0f172a;border:1px solid #475569;color:#e2e8f0;padding:7px 12px;border-radius:6px;font-size:13px;width:280px}
.config-bar input:focus{outline:none;border-color:#38bdf8;box-shadow:0 0 0 2px rgba(56,189,248,.2)}
.container{display:flex;min-height:calc(100vh - 120px)}
.sidebar{width:220px;background:#1e293b;border-right:1px solid #334155;padding:16px 0;flex-shrink:0}
.sidebar .group{padding:8px 20px;font-size:11px;color:#64748b;text-transform:uppercase;letter-spacing:1px;font-weight:600;margin-top:8px}
.sidebar button{display:block;width:100%;text-align:left;padding:10px 24px;background:none;border:none;color:#cbd5e1;font-size:13px;cursor:pointer;transition:all .15s}
.sidebar button:hover{background:#334155;color:#f1f5f9}
.sidebar button.active{background:linear-gradient(90deg,rgba(56,189,248,.15),transparent);color:#38bdf8;border-right:3px solid #38bdf8;font-weight:600}
.main{flex:1;padding:28px 32px;overflow-y:auto}
.tab-content{display:none}
.tab-content.active{display:block}
.card{background:#1e293b;border:1px solid #334155;border-radius:10px;padding:24px;margin-bottom:20px}
.card h2{font-size:16px;font-weight:600;margin-bottom:16px;color:#f1f5f9;display:flex;align-items:center;gap:8px}
.card h2 .icon{width:20px;height:20px;display:inline-flex;align-items:center;justify-content:center;font-size:14px}
.form-row{display:flex;gap:12px;margin-bottom:12px;align-items:flex-start;flex-wrap:wrap}
.form-group{display:flex;flex-direction:column;gap:4px;flex:1;min-width:160px}
.form-group label{font-size:12px;color:#94a3b8;font-weight:500}
.form-group input,.form-group select{background:#0f172a;border:1px solid #475569;color:#e2e8f0;padding:8px 12px;border-radius:6px;font-size:13px}
.form-group input:focus,.form-group select:focus{outline:none;border-color:#38bdf8}
.form-group select{cursor:pointer}
.btn{padding:8px 20px;border:none;border-radius:6px;font-size:13px;font-weight:600;cursor:pointer;transition:all .15s;display:inline-flex;align-items:center;gap:6px}
.btn-primary{background:#2563eb;color:#fff}.btn-primary:hover{background:#1d4ed8}
.btn-green{background:#16a34a;color:#fff}.btn-green:hover{background:#15803d}
.btn-red{background:#dc2626;color:#fff}.btn-red:hover{background:#b91c1c}
.btn-orange{background:#ea580c;color:#fff}.btn-orange:hover{background:#c2410c}
.btn-purple{background:#7c3aed;color:#fff}.btn-purple:hover{background:#6d28d9}
.btn-sm{padding:6px 14px;font-size:12px}
.result-box{background:#0f172a;border:1px solid #334155;border-radius:8px;padding:16px;margin-top:16px;font-family:"SF Mono",Monaco,"Cascadia Code",monospace;font-size:12px;line-height:1.7;overflow-x:auto;white-space:pre-wrap;word-break:break-all}
.result-box .status{font-weight:700;margin-bottom:8px}
.status-ok{color:#22c55e}.status-err{color:#ef4444}.status-warn{color:#f59e0b}
.quick-grid{display:grid;grid-template-columns:repeat(auto-fill,minmax(200px,1fr));gap:12px;margin-bottom:20px}
.quick-card{background:#0f172a;border:1px solid #334155;border-radius:8px;padding:16px;cursor:pointer;transition:all .15s;text-align:center}
.quick-card:hover{border-color:#38bdf8;transform:translateY(-1px)}
.quick-card .label{font-size:12px;color:#94a3b8;margin-bottom:4px}
.quick-card .value{font-size:20px;font-weight:700;color:#f1f5f9}
.quick-card .value.green{color:#22c55e}.quick-card .value.blue{color:#38bdf8}.quick-card .value.yellow{color:#f59e0b}
.actions-grid{display:grid;grid-template-columns:repeat(auto-fill,minmax(180px,1fr));gap:12px}
.action-btn{padding:14px;background:#0f172a;border:1px solid #334155;border-radius:8px;cursor:pointer;text-align:center;transition:all .15s}
.action-btn:hover{border-color:#38bdf8;transform:translateY(-1px)}
.action-btn .action-icon{font-size:24px;margin-bottom:6px}
.action-btn .action-label{font-size:13px;font-weight:600;color:#e2e8f0}
.action-btn .action-desc{font-size:11px;color:#64748b;margin-top:2px}
.protocol-hint{font-size:11px;color:#64748b;margin-top:4px;padding:8px;background:#0f172a;border-radius:4px;line-height:1.6}
.protocol-hint code{background:#334155;padding:1px 5px;border-radius:3px;color:#38bdf8;font-size:11px}
.empty-state{text-align:center;padding:40px;color:#64748b}
.empty-state .empty-icon{font-size:48px;margin-bottom:12px;opacity:.5}
.empty-state p{font-size:14px}
</style>
</head>
<body>

<div class="header">
    <h1>⚡ Node Agent API 测试面板</h1>
    <span class="badge">v1.0</span>
</div>

<form method="POST" id="mainForm">
<input type="hidden" name="action" id="actionInput">
<input type="hidden" name="active_tab" id="activeTabInput" value="<?= htmlspecialchars($activeTab) ?>">

<div class="config-bar">
    <label>API 地址</label>
    <input type="text" name="api_base" value="<?= htmlspecialchars($apiBase) ?>" placeholder="http://127.0.0.1:8080">
    <label>Token</label>
    <input type="text" name="api_token" value="<?= htmlspecialchars($apiToken) ?>" placeholder="X-Node-Token" style="width:240px">
    <button type="button" class="btn btn-primary btn-sm" onclick="quickAction('health')">🔗 测试连接</button>
</div>

<div class="container">
    <div class="sidebar">
        <div class="group">监控</div>
        <button type="button" onclick="switchTab('status')" class="<?= $activeTab==='status'?'active':'' ?>">📊 节点状态</button>
        <button type="button" onclick="switchTab('stats')" class="<?= $activeTab==='stats'?'active':'' ?>">📈 流量统计</button>
        <button type="button" onclick="switchTab('heartbeat')" class="<?= $activeTab==='heartbeat'?'active':'' ?>">💓 心跳数据</button>
        <div class="group">部署</div>
        <button type="button" onclick="switchTab('deploy')" class="<?= $activeTab==='deploy'?'active':'' ?>">🚀 配置部署</button>
        <div class="group">控制</div>
        <button type="button" onclick="switchTab('control')" class="<?= $activeTab==='control'?'active':'' ?>">🎮 进程控制</button>
        <div class="group">设备</div>
        <button type="button" onclick="switchTab('device')" class="<?= $activeTab==='device'?'active':'' ?>">📱 设备管理</button>
    </div>

    <div class="main">

        <!-- 节点状态 -->
        <div class="tab-content <?= $activeTab==='status'?'active':'' ?>" id="tab-status">
            <div class="card">
                <h2><span class="icon">📊</span> 节点状态</h2>
                <p style="font-size:13px;color:#94a3b8;margin-bottom:16px">获取节点运行状态、系统资源、连接数和设备信息</p>
                <button type="button" class="btn btn-primary" onclick="quickAction('status')">🔍 查询状态</button>
            </div>
            <?php if ($result && ($_POST['action'] ?? '') === 'status'): ?>
            <div class="card">
                <h2>返回结果 <span class="status <?= $result['http_code']===200?'status-ok':'status-err' ?>">HTTP <?= $result['http_code'] ?></span></h2>
                <?php if ($result['http_code'] === 200 && is_array($result['body'])): ?>
                <div class="quick-grid">
                    <div class="quick-card">
                        <div class="label">sing-box 状态</div>
                        <div class="value <?= ($result['body']['node']['singbox_running']??false)?'green':'yellow' ?>"><?= ($result['body']['node']['singbox_running']??false)?'运行中':'已停止' ?></div>
                    </div>
                    <div class="quick-card">
                        <div class="label">进程状态</div>
                        <div class="value blue"><?= htmlspecialchars($result['body']['node']['singbox_state'] ?? 'N/A') ?></div>
                    </div>
                    <div class="quick-card">
                        <div class="label">PID</div>
                        <div class="value"><?= $result['body']['node']['pid'] ?? 'N/A' ?></div>
                    </div>
                    <div class="quick-card">
                        <div class="label">运行时间</div>
                        <div class="value"><?= isset($result['body']['node']['uptime_seconds'])?gmdate('H:i:s',$result['body']['node']['uptime_seconds']):'N/A' ?></div>
                    </div>
                    <div class="quick-card">
                        <div class="label">CPU</div>
                        <div class="value"><?= round($result['body']['system']['cpu_percent']??0,1) ?>%</div>
                    </div>
                    <div class="quick-card">
                        <div class="label">内存</div>
                        <div class="value"><?= round($result['body']['system']['mem_percent']??0,1) ?>%</div>
                    </div>
                    <div class="quick-card">
                        <div class="label">活跃连接</div>
                        <div class="value blue"><?= $result['body']['connections']['active'] ?? 0 ?></div>
                    </div>
                    <div class="quick-card">
                        <div class="label">在线用户</div>
                        <div class="value green"><?= $result['body']['devices']['online_users'] ?? 0 ?></div>
                    </div>
                </div>
                <?php endif; ?>
                <div class="result-box"><span class="status <?= $result['http_code']===200?'status-ok':'status-err' ?>">HTTP <?= $result['http_code'] ?></span>
<?= htmlspecialchars(json_encode($result['body'], JSON_PRETTY_PRINT | JSON_UNESCAPED_UNICODE)) ?></div>
            </div>
            <?php endif; ?>
        </div>

        <!-- 流量统计 -->
        <div class="tab-content <?= $activeTab==='stats'?'active':'' ?>" id="tab-stats">
            <div class="card">
                <h2><span class="icon">📈</span> 流量统计</h2>
                <p style="font-size:13px;color:#94a3b8;margin-bottom:16px">获取节点上传/下载流量和活跃连接数</p>
                <button type="button" class="btn btn-primary" onclick="quickAction('stats')">📊 查询流量</button>
            </div>
            <?php if ($result && ($_POST['action'] ?? '') === 'stats'): ?>
            <div class="card">
                <h2>返回结果 <span class="status <?= $result['http_code']===200?'status-ok':'status-err' ?>">HTTP <?= $result['http_code'] ?></span></h2>
                <?php if ($result['http_code'] === 200 && is_array($result['body'])): ?>
                <div class="quick-grid">
                    <div class="quick-card">
                        <div class="label">上传流量</div>
                        <div class="value blue"><?= number_format(($result['body']['traffic']['upload']??0)/1024/1024,2) ?> MB</div>
                    </div>
                    <div class="quick-card">
                        <div class="label">下载流量</div>
                        <div class="value green"><?= number_format(($result['body']['traffic']['download']??0)/1024/1024,2) ?> MB</div>
                    </div>
                    <div class="quick-card">
                        <div class="label">活跃连接</div>
                        <div class="value"><?= $result['body']['connections']['active'] ?? 0 ?></div>
                    </div>
                </div>
                <?php endif; ?>
                <div class="result-box"><span class="status <?= $result['http_code']===200?'status-ok':'status-err' ?>">HTTP <?= $result['http_code'] ?></span>
<?= htmlspecialchars(json_encode($result['body'], JSON_PRETTY_PRINT | JSON_UNESCAPED_UNICODE)) ?></div>
            </div>
            <?php endif; ?>
        </div>

        <!-- 心跳数据 -->
        <div class="tab-content <?= $activeTab==='heartbeat'?'active':'' ?>" id="tab-heartbeat">
            <div class="card">
                <h2><span class="icon">💓</span> 心跳数据</h2>
                <p style="font-size:13px;color:#94a3b8;margin-bottom:16px">获取节点心跳上报的完整 payload 数据</p>
                <button type="button" class="btn btn-primary" onclick="quickAction('heartbeat')">💓 获取心跳</button>
            </div>
            <?php if ($result && ($_POST['action'] ?? '') === 'heartbeat'): ?>
            <div class="card">
                <h2>返回结果 <span class="status <?= $result['http_code']===200?'status-ok':'status-err' ?>">HTTP <?= $result['http_code'] ?></span></h2>
                <div class="result-box"><span class="status <?= $result['http_code']===200?'status-ok':'status-err' ?>">HTTP <?= $result['http_code'] ?></span>
<?= htmlspecialchars(json_encode($result['body'], JSON_PRETTY_PRINT | JSON_UNESCAPED_UNICODE)) ?></div>
            </div>
            <?php endif; ?>
        </div>

        <!-- 配置部署 -->
        <div class="tab-content <?= $activeTab==='deploy'?'active':'' ?>" id="tab-deploy">
            <div class="card">
                <h2><span class="icon">🚀</span> 配置部署</h2>
                <p style="font-size:13px;color:#94a3b8;margin-bottom:16px">生成 sing-box 配置并部署到节点，支持 Hysteria2 / VLESS / Reality 协议</p>

                <div class="form-row">
                    <div class="form-group">
                        <label>协议类型</label>
                        <select name="deploy_protocol" id="deploy_protocol" onchange="toggleDeployFields()">
                            <option value="hysteria2" <?= (($_POST['deploy_protocol']??'hysteria2')==='hysteria2')?'selected':'' ?>>Hysteria2</option>
                            <option value="vless" <?= (($_POST['deploy_protocol']??'')==='vless')?'selected':'' ?>>VLESS</option>
                            <option value="reality" <?= (($_POST['deploy_protocol']??'')==='reality')?'selected':'' ?>>Reality</option>
                        </select>
                    </div>
                    <div class="form-group">
                        <label>入站类型</label>
                        <select name="deploy_inbound_type">
                            <option value="tun" <?= (($_POST['deploy_inbound_type']??'tun')==='tun')?'selected':'' ?>>TUN (全局代理)</option>
                            <option value="mixed" <?= (($_POST['deploy_inbound_type']??'')==='mixed')?'selected':'' ?>>Mixed (SOCKS+HTTP)</option>
                        </select>
                    </div>
                </div>

                <div class="form-row">
                    <div class="form-group">
                        <label>User ID *</label>
                        <input type="text" name="deploy_user_id" value="<?= htmlspecialchars($_POST['deploy_user_id']??'user-001') ?>" required>
                    </div>
                    <div class="form-group">
                        <label>Node ID *</label>
                        <input type="text" name="deploy_node_id" value="<?= htmlspecialchars($_POST['deploy_node_id']??'node-001') ?>" required>
                    </div>
                </div>

                <div class="form-row">
                    <div class="form-group">
                        <label>服务器地址 *</label>
                        <input type="text" name="deploy_server" value="<?= htmlspecialchars($_POST['deploy_server']??'hk1.example.com') ?>" required>
                    </div>
                    <div class="form-group">
                        <label>端口 *</label>
                        <input type="number" name="deploy_port" value="<?= htmlspecialchars($_POST['deploy_port']??'443') ?>" required>
                    </div>
                </div>

                <div class="form-row">
                    <div class="form-group">
                        <label>密码 *</label>
                        <input type="text" name="deploy_password" value="<?= htmlspecialchars($_POST['deploy_password']??'my-password') ?>">
                    </div>
                    <div class="form-group" id="uuid-group">
                        <label>UUID (VLESS/Reality)</label>
                        <input type="text" name="deploy_uuid" value="<?= htmlspecialchars($_POST['deploy_uuid']??'') ?>" placeholder="留空则使用密码字段">
                    </div>
                </div>

                <div class="form-row">
                    <div class="form-group">
                        <label>SNI</label>
                        <input type="text" name="deploy_sni" value="<?= htmlspecialchars($_POST['deploy_sni']??'') ?>" placeholder="服务器域名">
                    </div>
                </div>

                <div class="form-row" id="reality-fields" style="display:none">
                    <div class="form-group">
                        <label>Reality Public Key</label>
                        <input type="text" name="deploy_reality_pubkey" value="<?= htmlspecialchars($_POST['deploy_reality_pubkey']??'') ?>" placeholder="Reality 公钥">
                    </div>
                    <div class="form-group">
                        <label>Reality Short ID</label>
                        <input type="text" name="deploy_reality_shortid" value="<?= htmlspecialchars($_POST['deploy_reality_shortid']??'') ?>" placeholder="6ba85179930d344f">
                    </div>
                </div>

                <div class="protocol-hint" id="protocol-hint">
                    <strong>Hysteria2</strong>：需要 <code>password</code>，TLS 自动启用。SNI 留空则 insecure=true。
                </div>

                <div style="margin-top:16px">
                    <button type="button" class="btn btn-green" onclick="submitAction('deploy')">🚀 部署配置</button>
                </div>
            </div>
            <?php if ($result && ($_POST['action'] ?? '') === 'deploy'): ?>
            <div class="card">
                <h2>部署结果 <span class="status <?= $result['http_code']===200?'status-ok':'status-err' ?>">HTTP <?= $result['http_code'] ?></span></h2>
                <div class="result-box"><span class="status <?= $result['http_code']===200?'status-ok':'status-err' ?>">HTTP <?= $result['http_code'] ?></span>
<?= htmlspecialchars(json_encode($result['body'], JSON_PRETTY_PRINT | JSON_UNESCAPED_UNICODE)) ?></div>
            </div>
            <?php endif; ?>
        </div>

        <!-- 进程控制 -->
        <div class="tab-content <?= $activeTab==='control'?'active':'' ?>" id="tab-control">
            <div class="card">
                <h2><span class="icon">🎮</span> 进程控制</h2>
                <p style="font-size:13px;color:#94a3b8;margin-bottom:16px">启动、停止、重启 sing-box 进程</p>
                <div class="actions-grid">
                    <div class="action-btn" onclick="confirmAction('start','确认启动 sing-box？')">
                        <div class="action-icon">▶️</div>
                        <div class="action-label">启动</div>
                        <div class="action-desc">启动 sing-box 进程</div>
                    </div>
                    <div class="action-btn" onclick="confirmAction('stop','确认停止 sing-box？')">
                        <div class="action-icon">⏹️</div>
                        <div class="action-label">停止</div>
                        <div class="action-desc">停止 sing-box 进程</div>
                    </div>
                    <div class="action-btn" onclick="confirmAction('restart','确认重启 sing-box？')">
                        <div class="action-icon">🔄</div>
                        <div class="action-label">重启</div>
                        <div class="action-desc">重启 sing-box 进程</div>
                    </div>
                </div>
            </div>
            <?php if ($result && in_array($_POST['action']??'', ['start','stop','restart'])): ?>
            <div class="card">
                <h2>操作结果 <span class="status <?= $result['http_code']===200?'status-ok':'status-err' ?>">HTTP <?= $result['http_code'] ?></span></h2>
                <div class="result-box"><span class="status <?= $result['http_code']===200?'status-ok':'status-err' ?>">HTTP <?= $result['http_code'] ?></span>
<?= htmlspecialchars(json_encode($result['body'], JSON_PRETTY_PRINT | JSON_UNESCAPED_UNICODE)) ?></div>
            </div>
            <?php endif; ?>
        </div>

        <!-- 设备管理 -->
        <div class="tab-content <?= $activeTab==='device'?'active':'' ?>" id="tab-device">
            <div class="card">
                <h2><span class="icon">📱</span> 设备注册</h2>
                <div class="form-row">
                    <div class="form-group">
                        <label>User ID</label>
                        <input type="text" name="device_user_id" value="<?= htmlspecialchars($_POST['device_user_id']??'user-001') ?>">
                    </div>
                    <div class="form-group">
                        <label>Device ID</label>
                        <input type="text" name="device_device_id" value="<?= htmlspecialchars($_POST['device_device_id']??'device-001') ?>">
                    </div>
                    <div class="form-group">
                        <label>IP 地址</label>
                        <input type="text" name="device_ip" value="<?= htmlspecialchars($_POST['device_ip']??'10.0.0.1') ?>">
                    </div>
                </div>
                <button type="button" class="btn btn-purple" onclick="submitAction('device_register')">📱 注册设备</button>
            </div>

            <div class="card">
                <h2><span class="icon">🔐</span> 会话管理</h2>
                <div class="form-row">
                    <div class="form-group">
                        <label>User ID</label>
                        <input type="text" name="session_user_id" value="<?= htmlspecialchars($_POST['session_user_id']??'user-001') ?>">
                    </div>
                </div>
                <div style="display:flex;gap:10px">
                    <button type="button" class="btn btn-green" onclick="submitAction('session_acquire')">➕ 获取会话</button>
                    <button type="button" class="btn btn-red" onclick="submitAction('session_release')">➖ 释放会话</button>
                </div>
            </div>

            <?php if ($result && in_array($_POST['action']??'', ['device_register','session_acquire','session_release'])): ?>
            <div class="card">
                <h2>操作结果 <span class="status <?= $result['http_code']===200?'status-ok':'status-err' ?>">HTTP <?= $result['http_code'] ?></span></h2>
                <div class="result-box"><span class="status <?= $result['http_code']===200?'status-ok':'status-err' ?>">HTTP <?= $result['http_code'] ?></span>
<?= htmlspecialchars(json_encode($result['body'], JSON_PRETTY_PRINT | JSON_UNESCAPED_UNICODE)) ?></div>
            </div>
            <?php endif; ?>
        </div>

        <!-- 无结果时的空状态 -->
        <?php if (!$result): ?>
        <div class="tab-content active" id="tab-empty" style="<?= $result?'display:none':'' ?>">
            <div class="empty-state">
                <div class="empty-icon">🧪</div>
                <p>选择左侧菜单开始测试 API</p>
            </div>
        </div>
        <?php endif; ?>

    </div>
</div>
</form>

<script>
function switchTab(tab) {
    document.querySelectorAll('.tab-content').forEach(el => el.classList.remove('active'));
    document.querySelectorAll('.sidebar button').forEach(el => el.classList.remove('active'));
    var tabEl = document.getElementById('tab-' + tab);
    if (tabEl) tabEl.classList.add('active');
    document.getElementById('activeTabInput').value = tab;
    var btns = document.querySelectorAll('.sidebar button');
    btns.forEach(b => { if (b.textContent.trim().includes(getTabLabel(tab))) b.classList.add('active'); });
    var emptyEl = document.getElementById('tab-empty');
    if (emptyEl) emptyEl.style.display = 'none';
}
function getTabLabel(tab) {
    var map = {status:'节点状态',stats:'流量统计',heartbeat:'心跳数据',deploy:'配置部署',control:'进程控制',device:'设备管理'};
    return map[tab] || tab;
}
function submitAction(action) {
    document.getElementById('actionInput').value = action;
    document.getElementById('mainForm').submit();
}
function quickAction(action) {
    document.getElementById('actionInput').value = action;
    document.getElementById('mainForm').submit();
}
function confirmAction(action, msg) {
    if (confirm(msg)) {
        document.getElementById('actionInput').value = action;
        document.getElementById('mainForm').submit();
    }
}
function toggleDeployFields() {
    var proto = document.getElementById('deploy_protocol').value;
    var realityFields = document.getElementById('reality-fields');
    var uuidGroup = document.getElementById('uuid-group');
    var hint = document.getElementById('protocol-hint');
    realityFields.style.display = proto === 'reality' ? 'flex' : 'none';
    uuidGroup.style.display = (proto === 'vless' || proto === 'reality') ? 'flex' : 'none';
    if (proto === 'hysteria2') {
        hint.innerHTML = '<strong>Hysteria2</strong>：需要 <code>password</code>，TLS 自动启用。SNI 留空则 insecure=true。';
    } else if (proto === 'vless') {
        hint.innerHTML = '<strong>VLESS</strong>：需要 <code>uuid</code>（留空则使用密码字段），Flow 固定为 <code>xtls-rprx-vision</code>，TLS 自动启用。';
    } else if (proto === 'reality') {
        hint.innerHTML = '<strong>Reality</strong>：需要 <code>uuid</code> + <code>reality_public_key</code> + <code>reality_short_id</code>，SNI 建议填写伪装域名。';
    }
}
toggleDeployFields();
</script>
</body>
</html>
