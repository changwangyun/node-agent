#!/bin/bash
set -e

REPO="changwangyun/node-agent"
INSTALL_DIR="/usr/local/bin"
CONFIG_DIR="/etc/node-agent"
SINGBOX_DIR="/etc/sing-box"
DATA_DIR="/var/lib/node-agent"
LOG_DIR="/var/log/node-agent"
BINARY="node-agent"

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
CYAN='\033[0;36m'
NC='\033[0m'

info()  { echo -e "${CYAN}[INFO]${NC} $*"; }
ok()    { echo -e "${GREEN}[OK]${NC} $*"; }
warn()  { echo -e "${YELLOW}[WARN]${NC} $*"; }
err()   { echo -e "${RED}[ERROR]${NC} $*"; exit 1; }

banner() {
    local ver
    ver=$(detect_latest_version)
    echo ""
    echo -e "${CYAN}╔══════════════════════════════════════════╗${NC}"
    echo -e "${CYAN}║${NC}     Node Agent VPN 节点 - 一键安装      ${CYAN}║${NC}"
    echo -e "${CYAN}║${NC}              版本: ${ver}                  ${CYAN}║${NC}"
    echo -e "${CYAN}╚══════════════════════════════════════════╝${NC}"
    echo ""
}

check_root() {
    if [ "$(id -u)" -ne 0 ]; then
        err "请使用 root 用户运行此脚本: sudo bash install.sh"
    fi
}

detect_arch() {
    local os arch
    os=$(uname -s | tr '[:upper:]' '[:lower:]')
    case "$(uname -m)" in
        x86_64|amd64)   arch="amd64" ;;
        aarch64|arm64)  arch="arm64" ;;
        armv7l|armv7)   arch="armv7" ;;
        *)              err "不支持的架构: $(uname -m)" ;;
    esac
    echo "${os}-${arch}"
}

detect_latest_version() {
    local url="https://api.github.com/repos/${REPO}/releases/latest"
    local ver
    ver=$(curl -fsSL --connect-timeout 10 "$url" 2>/dev/null | grep '"tag_name"' | head -1 | sed -E 's/.*"tag_name":\s*"([^"]+)".*/\1/')
    if [ -z "$ver" ]; then
        ver="v1.0.0"
    fi
    echo "$ver"
}

install_binary_local() {
    local src="$1"
    if [ ! -f "$src" ]; then
        return 1
    fi
    cp "$src" "${INSTALL_DIR}/${BINARY}"
    chmod +x "${INSTALL_DIR}/${BINARY}"
    ok "二进制已安装到 ${INSTALL_DIR}/${BINARY} (来自: $src)"
    return 0
}

install_binary_from_tar() {
    local tarfile="$1"
    if [ ! -f "$tarfile" ]; then
        return 1
    fi
    local tmpdir
    tmpdir=$(mktemp -d)
    tar xzf "$tarfile" -C "$tmpdir"
    if [ ! -f "${tmpdir}/${BINARY}" ]; then
        rm -rf "$tmpdir"
        return 1
    fi
    cp "${tmpdir}/${BINARY}" "${INSTALL_DIR}/${BINARY}"
    chmod +x "${INSTALL_DIR}/${BINARY}"
    rm -rf "$tmpdir"
    ok "二进制已安装到 ${INSTALL_DIR}/${BINARY} (来自: $tarfile)"
    return 0
}

install_binary_from_github() {
    local arch="$1"
    local ver="$2"
    local filename="${BINARY}-${arch}.tar.gz"
    local url

    if [ "$ver" = "latest" ]; then
        ver=$(detect_latest_version)
    fi

    url="https://github.com/${REPO}/releases/download/${ver}/${filename}"

    info "从 GitHub 下载 ${BINARY} ${ver} (${arch})..."
    local tmpdir
    tmpdir=$(mktemp -d)

    if curl -fsSL --progress-bar --connect-timeout 30 -o "${tmpdir}/${filename}" "$url" 2>/dev/null; then
        tar xzf "${tmpdir}/${filename}" -C "${tmpdir}"
        if [ -f "${tmpdir}/${BINARY}" ]; then
            cp "${tmpdir}/${BINARY}" "${INSTALL_DIR}/${BINARY}"
            chmod +x "${INSTALL_DIR}/${BINARY}"
            rm -rf "$tmpdir"
            ok "二进制已安装到 ${INSTALL_DIR}/${BINARY} (来自: GitHub ${ver})"
            return 0
        fi
    fi

    rm -rf "$tmpdir"
    return 1
}

install_binary_from_source() {
    info "从源码编译安装..."
    info "安装 Go 编译环境..."

    if ! command -v go &>/dev/null; then
        local go_ver
        go_ver=$(curl -fsSL 'https://go.dev/VERSION?m=text' 2>/dev/null | head -1 || echo "go1.21.0")
        local go_os go_arch
        go_os=$(uname -s | tr '[:upper:]' '[:lower:]')
        case "$(uname -m)" in
            x86_64|amd64)  go_arch="amd64" ;;
            aarch64|arm64) go_arch="arm64" ;;
            *)             go_arch="armv6l" ;;
        esac

        local go_url="https://dl.google.com/go/${go_ver}.${go_os}-${go_arch}.tar.gz"
        info "下载 ${go_ver}..."
        if ! curl -fsSL --progress-bar -o /tmp/go.tar.gz "$go_url"; then
            err "Go 下载失败，请手动安装 Go 1.21+ 后重新运行"
        fi
        rm -rf /usr/local/go
        tar -C /usr/local -xzf /tmp/go.tar.gz
        rm -f /tmp/go.tar.gz
        export PATH="/usr/local/go/bin:$PATH"
        ok "Go ${go_ver} 已安装"
    else
        ok "Go 已安装: $(go version)"
    fi

    local src_dir="${SCRIPT_DIR}"
    if [ ! -f "${src_dir}/go.mod" ]; then
        info "克隆源码..."
        src_dir="/tmp/node-agent-build"
        rm -rf "$src_dir"
        git clone https://github.com/${REPO}.git "$src_dir" 2>/dev/null || {
            err "无法克隆仓库。请将源码上传到 VPS 后在源码目录内运行: bash deploy/install.sh --local"
        }
    fi

    info "编译..."
    cd "$src_dir"
    CGO_ENABLED=0 go build -ldflags="-s -w" -o "${BINARY}" .
    cp "${BINARY}" "${INSTALL_DIR}/${BINARY}"
    chmod +x "${INSTALL_DIR}/${BINARY}"
    ok "二进制已安装到 ${INSTALL_DIR}/${BINARY} (从源码编译)"
}

install_binary() {
    local arch="$1"
    local mode="$2"

    case "$mode" in
        local)
            info "使用本地二进制..."
            if install_binary_local "${SCRIPT_DIR}/${BINARY}"; then
                return 0
            fi
            if install_binary_local "${SCRIPT_DIR}/dist/${BINARY}"; then
                return 0
            fi
            local tarfile
            for f in "${SCRIPT_DIR}"/${BINARY}-${arch}.tar.gz "${SCRIPT_DIR}"/dist/${BINARY}-${arch}.tar.gz "${SCRIPT_DIR}"/${BINARY}-*.tar.gz; do
                if [ -f "$f" ]; then
                    tarfile="$f"
                    break
                fi
            done
            if [ -n "$tarfile" ] && install_binary_from_tar "$tarfile"; then
                return 0
            fi
            err "未找到本地二进制文件。请先编译: make build 或 make release"
            ;;
        source)
            install_binary_from_source
            ;;
        github|*)
            info "尝试从 GitHub Release 下载..."
            if install_binary_from_github "$arch" "latest"; then
                return 0
            fi
            warn "GitHub Release 下载失败，尝试从源码编译..."
            install_binary_from_source
            ;;
    esac
}

install_singbox() {
    local need_upgrade=false

    if command -v sing-box &>/dev/null; then
        local current_ver
        current_ver=$(sing-box version 2>/dev/null | head -1 | grep -oE '[0-9]+\.[0-9]+\.[0-9]+' | head -1)
        if [ -n "$current_ver" ]; then
            local major minor
            major=$(echo "$current_ver" | cut -d. -f1)
            minor=$(echo "$current_ver" | cut -d. -f2)
            if [ "$major" -lt 1 ] || { [ "$major" -eq 1 ] && [ "$minor" -lt 10 ]; }; then
                warn "sing-box 版本过低 ($current_ver)，需要 >= v1.10.0 以支持性能优化特性"
                need_upgrade=true
            elif sing-box version 2>/dev/null | grep -q "with_v2ray_api" && sing-box version 2>/dev/null | grep -q "with_quic" && sing-box version 2>/dev/null | grep -q "with_clash_api"; then
                ok "sing-box 已安装(含 v2ray_api + quic + clash_api): $(sing-box version 2>/dev/null | head -1)"
                return 0
            else
                warn "当前 sing-box 缺少必要编译标签，需要重新编译以支持按用户流量统计、Hysteria2 和 Clash API"
                need_upgrade=true
            fi
        else
            warn "无法检测 sing-box 版本"
            need_upgrade=true
        fi
    else
        need_upgrade=true
    fi

    if [ "$need_upgrade" = true ]; then
        info "从源码编译 sing-box (含 with_v2ray_api,with_quic,with_clash_api 标签)..."
    fi

    if ! command -v git &>/dev/null; then
        info "安装 git..."
        if command -v apt-get &>/dev/null; then
            apt-get update -qq && apt-get install -y -qq git >/dev/null 2>&1
        elif command -v yum &>/dev/null; then
            yum install -y -q git >/dev/null 2>&1
        elif command -v apk &>/dev/null; then
            apk add --no-cache git >/dev/null 2>&1
        else
            warn "无法自动安装 git，请手动安装后重试"
        fi
        if command -v git &>/dev/null; then
            ok "git 已安装"
        fi
    fi

    if ! command -v go &>/dev/null; then
        info "安装 Go 编译环境..."
        local go_ver
        go_ver=$(curl -fsSL 'https://go.dev/VERSION?m=text' | head -1)
        if [ -z "$go_ver" ]; then go_ver="go1.23.4"; fi
        local pkg
        case "$(uname -m)" in
            x86_64|amd64)   pkg="amd64" ;;
            aarch64|arm64)  pkg="arm64" ;;
            armv7l|armv7)   pkg="armv6l" ;;
            *)              err "不支持的架构" ;;
        esac
        curl -fsSL "https://go.dev/dl/${go_ver}.linux-${pkg}.tar.gz" | tar -C /usr/local -xzf -
        export PATH=$PATH:/usr/local/go/bin
        echo "export PATH=\$PATH:/usr/local/go/bin" >> /etc/profile.d/go.sh
        ok "Go ${go_ver} 已安装"
    fi

    local sb_ver
    sb_ver=$(curl -fsSL https://api.github.com/repos/SagerNet/sing-box/releases/latest | grep '"tag_name"' | head -1 | sed -E 's/.*"([^"]+)".*/\1/')
    if [ -z "$sb_ver" ]; then sb_ver="v1.14.0"; fi

    info "编译 sing-box ${sb_ver} (with_v2ray_api,with_quic,with_clash_api)..."
    local tmpdir
    tmpdir=$(mktemp -d)
    cd "${tmpdir}"

    if ! go install -ldflags "-X 'github.com/sagernet/sing-box/constant.Version=${sb_ver}'" -tags "with_v2ray_api,with_quic,with_clash_api" "github.com/sagernet/sing-box/cmd/sing-box@${sb_ver}"; then
        warn "编译失败，尝试下载预编译版本..."
        local pkg2
        case "$(uname -m)" in
            x86_64|amd64)   pkg2="amd64" ;;
            aarch64|arm64)  pkg2="arm64" ;;
            armv7l|armv7)   pkg2="armv7" ;;
            *)              pkg2="amd64" ;;
        esac
        local sb_url="https://github.com/SagerNet/sing-box/releases/download/${sb_ver}/sing-box-${sb_ver#v}-linux-${pkg2}.tar.gz"
        info "下载: $sb_url"
        if curl -fsSL --connect-timeout 10 --progress-bar -o "${tmpdir}/sing-box.tar.gz" "$sb_url"; then
            tar xzf "${tmpdir}/sing-box.tar.gz" -C "${tmpdir}"
            local sb_bin
            sb_bin=$(find "${tmpdir}" -name "sing-box" -type f | head -1)
            if [ -n "$sb_bin" ]; then
                cp "$sb_bin" "${INSTALL_DIR}/sing-box"
                chmod +x "${INSTALL_DIR}/sing-box"
                ok "sing-box 已安装(预编译版)"
            fi
        else
            warn "下载预编译版本失败，请检查网络连接或手动安装 sing-box"
        fi
    else
        cp "$(go env GOPATH)/bin/sing-box" "${INSTALL_DIR}/sing-box"
        chmod +x "${INSTALL_DIR}/sing-box"
        ok "sing-box ${sb_ver} 已编译安装(含 with_v2ray_api,with_quic,with_clash_api)"
    fi

    cd -
    rm -rf "${tmpdir}"
}

create_dirs() {
    info "创建目录..."
    mkdir -p "${CONFIG_DIR}" "${SINGBOX_DIR}" "${DATA_DIR}" "${LOG_DIR}"

    if [ ! -f "${SINGBOX_DIR}/config.json" ]; then
        cat > "${SINGBOX_DIR}/config.json" << 'SBEOF'
{
  "log": { "level": "info" },
  "dns": {
    "servers": [
      { "tag": "google", "type": "tls", "server": "8.8.8.8" },
      { "tag": "local", "type": "udp", "server": "223.5.5.5" }
    ]
  },
  "inbounds": [],
  "outbounds": [
    { "tag": "direct", "type": "direct" }
  ],
  "route": {
    "rules": [
      { "action": "sniff" },
      { "protocol": "dns", "action": "hijack-dns" }
    ],
    "default_domain_resolver": "google",
    "final": "direct"
  }
}
SBEOF
        ok "sing-box 占位配置已创建: ${SINGBOX_DIR}/config.json"
        warn "此为占位配置，请通过 POST /deploy 部署实际协议配置"
    fi

    ok "目录已创建"
}

install_config() {
    if [ -f "${CONFIG_DIR}/config.json" ]; then
        warn "配置文件已存在，跳过: ${CONFIG_DIR}/config.json"
        warn "如需重置: mv ${CONFIG_DIR}/config.json ${CONFIG_DIR}/config.json.bak"
        return 0
    fi

    local panel_type

    echo ""
    echo -e "  ${CYAN}请选择面板类型:${NC}"
    echo "  1) Laravel 自有面板 (面板推送配置到节点)"
    echo "  2) Xboard 面板 (节点从面板拉取配置)"
    read -rp "  请选择 [1]: " panel_type
    panel_type="${panel_type:-1}"

    if [ "$panel_type" = "2" ]; then
        install_config_xboard
    else
        install_config_laravel
    fi
}

install_config_laravel() {
    local node_id api_token cp_url cp_token

    read -rp "请输入 Node ID [node-001]: " node_id
    node_id="${node_id:-node-001}"

    read -rp "请输入 API Token [自动生成]: " api_token
    if [ -z "$api_token" ]; then
        api_token=$(openssl rand -hex 16 2>/dev/null || cat /proc/sys/kernel/random/uuid 2>/dev/null || echo "change-me-$(date +%s)")
    fi

    read -rp "请输入控制面板 URL [http://127.0.0.1:8000]: " cp_url
    cp_url="${cp_url:-http://127.0.0.1:8000}"

    read -rp "请输入控制面板 Token: " cp_token

    cat > "${CONFIG_DIR}/config.json" << EOF
{
  "node_id": "${node_id}",
  "api_port": 8080,
  "api_token": "${api_token}",
  "log_level": "info",
  "data_dir": "/var/lib/node-agent",
  "singbox": {
    "binary_path": "/usr/local/bin/sing-box",
    "config_path": "/etc/sing-box/config.json",
    "work_dir": "/etc/sing-box"
  },
  "control_plane": {
    "url": "${cp_url}",
    "token": "${cp_token}",
    "node_id": "${node_id}",
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
    chmod 600 "${CONFIG_DIR}/config.json"
    ok "配置文件已创建: ${CONFIG_DIR}/config.json (Laravel 面板模式)"
    echo ""
    echo -e "  ${YELLOW}API Token: ${api_token}${NC}"
    echo -e "  ${YELLOW}请妥善保管此 Token！${NC}"
    echo ""
}

install_config_xboard() {
    local xb_host xb_key xb_node_id xb_node_type xb_interval

    read -rp "请输入 Xboard 面板地址 (例: https://panel.example.com): " xb_host
    if [ -z "$xb_host" ]; then
        err "Xboard 面板地址不能为空"
    fi

    read -rp "请输入 Xboard 节点 Token (在 Xboard 后台节点设置中获取): " xb_key
    if [ -z "$xb_key" ]; then
        err "Xboard 节点 Token 不能为空"
    fi

    read -rp "请输入 Xboard 节点 ID [1]: " xb_node_id
    xb_node_id="${xb_node_id:-1}"

    echo ""
    echo -e "  ${CYAN}请选择节点协议类型:${NC}"
    echo "  1) hysteria2"
    echo "  2) vless"
    echo "  3) trojan"
    echo "  4) reality"
    read -rp "  请选择 [1]: " xb_node_type_choice
    xb_node_type_choice="${xb_node_type_choice:-1}"
    case "$xb_node_type_choice" in
        2) xb_node_type="vless" ;;
        3) xb_node_type="trojan" ;;
        4) xb_node_type="reality" ;;
        *) xb_node_type="hysteria2" ;;
    esac

    read -rp "请输入同步间隔（秒）[60]: " xb_interval
    xb_interval="${xb_interval:-60}"

    local xb_api_token
    xb_api_token=$(openssl rand -hex 16 2>/dev/null || echo "xboard-mode-$(date +%s)")

    cat > "${CONFIG_DIR}/config.json" << EOF
{
  "node_id": "${xb_node_id}",
  "api_port": 8080,
  "api_token": "${xb_api_token}",
  "log_level": "info",
  "data_dir": "/var/lib/node-agent",
  "panel_type": "xboard",
  "singbox": {
    "binary_path": "/usr/local/bin/sing-box",
    "config_path": "/etc/sing-box/config.json",
    "work_dir": "/etc/sing-box"
  },
  "control_plane": {
    "url": "",
    "token": "",
    "node_id": "${xb_node_id}",
    "timeout": 10
  },
  "xboard": {
    "api_host": "${xb_host}",
    "api_key": "${xb_key}",
    "node_id": ${xb_node_id},
    "node_type": "${xb_node_type}",
    "sync_interval": ${xb_interval},
    "timeout": 30
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
    chmod 600 "${CONFIG_DIR}/config.json"
    ok "配置文件已创建: ${CONFIG_DIR}/config.json (Xboard 面板模式)"
    echo ""
    echo -e "  ${YELLOW}面板地址:  ${xb_host}${NC}"
    echo -e "  ${YELLOW}节点 ID:   ${xb_node_id}${NC}"
    echo -e "  ${YELLOW}协议类型:  ${xb_node_type}${NC}"
    echo -e "  ${YELLOW}同步间隔:  ${xb_interval}s${NC}"
    echo ""
}

install_service() {
    info "安装 systemd 服务..."
    cat > /etc/systemd/system/${BINARY}.service << 'EOF'
[Unit]
Description=Node Agent - VPN Node Control Daemon
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
User=root
Group=root
ExecStart=/usr/local/bin/node-agent -config /etc/node-agent/config.json
ExecReload=/bin/kill -HUP $MAINPID
Restart=on-failure
RestartSec=5
LimitNOFILE=65535

CPUSchedulingPolicy=rr
CPUSchedulingPriority=99

WorkingDirectory=/var/lib/node-agent

Environment=HOME=/var/lib/node-agent
Environment=PATH=/usr/local/bin:/usr/bin:/bin

StandardOutput=journal
StandardError=journal
SyslogIdentifier=node-agent

ProtectSystem=strict
ProtectHome=yes
PrivateTmp=yes
NoNewPrivileges=no

ReadWritePaths=/etc/node-agent
ReadWritePaths=/etc/sing-box
ReadWritePaths=/var/lib/node-agent
ReadWritePaths=/var/log/node-agent

[Install]
WantedBy=multi-user.target
EOF

    systemctl daemon-reload
    systemctl enable ${BINARY}
    ok "systemd 服务已安装并启用"
}

optimize_system() {
    info "优化系统网络参数..."

    local sysctl_file="/etc/sysctl.d/99-node-agent.conf"
    local changed=false

    if ! grep -q "net.core.rmem_max" "$sysctl_file" 2>/dev/null || \
       ! grep -q "16777216" "$sysctl_file" 2>/dev/null; then
        cat > "$sysctl_file" << 'SYSCTL'
# Node Agent - Network Performance Optimization
# UDP Buffer (Hysteria2/QUIC critical)
net.core.rmem_max=16777216
net.core.wmem_max=16777216
net.core.rmem_default=16777216
net.core.wmem_default=16777216
net.core.netdev_max_backlog=65536
net.ipv4.udp_mem=65536 131072 262144
net.ipv4.udp_rmem_min=16384
net.ipv4.udp_wmem_min=16384
# TCP Buffer
net.ipv4.tcp_rmem=4096 87380 16777216
net.ipv4.tcp_wmem=4096 65536 16777216
# TCP Performance
net.core.somaxconn=65535
net.ipv4.tcp_fastopen=3
net.ipv4.tcp_slow_start_after_idle=0
net.ipv4.tcp_mtu_probing=1
net.ipv4.tcp_window_scaling=1
net.ipv4.tcp_sack=1
net.ipv4.tcp_fack=1
net.ipv4.tcp_low_latency=1
net.ipv4.tcp_no_metrics_save=1
net.ipv4.tcp_tw_reuse=1
net.ipv4.tcp_max_syn_backlog=65535
net.ipv4.tcp_max_tw_buckets=65535
net.ipv4.tcp_syncookies=1
net.ipv4.ip_local_port_range=1024 65535
# Connection Tracking
net.netfilter.nf_conntrack_max=1048576
net.netfilter.nf_conntrack_tcp_timeout_established=7200
SYSCTL
        changed=true
    fi

    if [ "$changed" = true ]; then
        sysctl -p "$sysctl_file" 2>/dev/null || true
        ok "网络参数已优化 (UDP 16MB + TCP BBR)"
    else
        info "系统网络参数已优化，跳过"
    fi

    if [ -f /proc/sys/net/ipv4/tcp_congestion_control ]; then
        local cc
        cc=$(cat /proc/sys/net/ipv4/tcp_congestion_control 2>/dev/null || echo "")
        if [ "$cc" != "bbr" ]; then
            if modprobe tcp_bbr 2>/dev/null; then
                if ! grep -q "tcp_bbr" /etc/modules-load.d/node-agent.conf 2>/dev/null; then
                    echo "tcp_bbr" > /etc/modules-load.d/node-agent.conf
                fi
                if ! grep -q "tcp_congestion_control" "$sysctl_file" 2>/dev/null; then
                    cat >> "$sysctl_file" << 'BBR'
# BBR Congestion Control
net.ipv4.tcp_congestion_control=bbr
net.core.default_qdisc=fq
BBR
                    sysctl -p "$sysctl_file" 2>/dev/null || true
                fi
                ok "BBR 拥塞控制已启用"
            else
                if ! grep -q "default_qdisc" "$sysctl_file" 2>/dev/null; then
                    cat >> "$sysctl_file" << 'FQCODEL'
net.core.default_qdisc=fq_codel
FQCODEL
                    sysctl -p "$sysctl_file" 2>/dev/null || true
                fi
                warn "无法加载 tcp_bbr 模块，已启用 fq_codel 队列调度作为备选"
            fi
        else
            info "BBR 已启用，跳过"
        fi
    fi

    local kernel_ver
    kernel_ver=$(uname -r | cut -d. -f1-2 2>/dev/null || echo "0")
    local major=$(echo "$kernel_ver" | cut -d. -f1)
    local minor=$(echo "$kernel_ver" | cut -d. -f2)
    if [ "$major" -gt 5 ] || { [ "$major" -eq 5 ] && [ "$minor" -ge 9 ]; }; then
        info "内核 $(uname -r) 支持 BBR，TCP 性能优化已就绪"
    else
        warn "内核 $(uname -r) 较旧 (< 5.9)，建议升级内核以获得更好 BBR 支持"
    fi
}

configure_firewall() {
    if command -v ufw &>/dev/null; then
        info "检测到 UFW 防火墙，放行 API 端口 8080..."
        ufw allow 8080/tcp &>/dev/null || true
        ok "UFW 规则已添加"
    elif command -v firewall-cmd &>/dev/null; then
        info "检测到 Firewalld，放行 API 端口 8080..."
        firewall-cmd --permanent --add-port=8080/tcp &>/dev/null || true
        firewall-cmd --reload &>/dev/null || true
        ok "Firewalld 规则已添加"
    else
        warn "未检测到防火墙管理工具，请手动放行 8080 端口"
    fi
}

start_service() {
    info "启动 Node Agent..."
    systemctl start ${BINARY}
    sleep 1
    if systemctl is-active --quiet ${BINARY}; then
        ok "Node Agent 已启动"
    else
        warn "Node Agent 启动失败，请检查日志: journalctl -u ${BINARY} -n 50"
    fi
}

show_result() {
    local token ver
    token=$(grep '"api_token"' "${CONFIG_DIR}/config.json" 2>/dev/null | sed -E 's/.*"api_token":\s*"([^"]+)".*/\1/' || echo "见配置文件")
    ver=$(${INSTALL_DIR}/${BINARY} -version 2>/dev/null || echo "unknown")

    echo ""
    echo -e "${GREEN}╔══════════════════════════════════════════╗${NC}"
    echo -e "${GREEN}║${NC}          ✅ 安装完成！                   ${GREEN}║${NC}"
    echo -e "${GREEN}╚══════════════════════════════════════════╝${NC}"
    echo ""
    echo -e "  ${CYAN}版本:${NC}        ${ver}"
    echo -e "  ${CYAN}API 地址:${NC}    http://$(hostname -I 2>/dev/null | awk '{print $1}' || echo 'SERVER_IP'):8080"
    echo -e "  ${CYAN}API Token:${NC}   ${token}"
    echo -e "  ${CYAN}配置文件:${NC}    ${CONFIG_DIR}/config.json"
    echo -e "  ${CYAN}数据目录:${NC}    ${DATA_DIR}"
    echo ""
    echo -e "  ${YELLOW}常用命令:${NC}"
    echo "    查看状态:  systemctl status ${BINARY}"
    echo "    查看日志:  journalctl -u ${BINARY} -f"
    echo "    重启服务:  systemctl restart ${BINARY}"
    echo "    停止服务:  systemctl stop ${BINARY}"
    echo "    修改配置:  vi ${CONFIG_DIR}/config.json"
    echo ""
    echo -e "  ${YELLOW}测试 API:${NC}"
    echo "    curl -H 'X-Node-Token: ${token}' http://localhost:8080/status"
    echo ""
}

do_install() {
    local mode="github"

    while [ $# -gt 0 ]; do
        case "$1" in
            --local)  mode="local" ;;
            --source) mode="source" ;;
            --github) mode="github" ;;
            *) ;;
        esac
        shift
    done

    banner
    check_root

    local arch
    arch=$(detect_arch)
    info "系统架构: ${arch}"

    if [ "$mode" = "local" ]; then
        info "安装模式: 本地二进制"
    elif [ "$mode" = "source" ]; then
        info "安装模式: 源码编译"
    else
        info "安装模式: GitHub Release (失败则回退源码编译)"
    fi

    if [ -f "${INSTALL_DIR}/${BINARY}" ]; then
        warn "检测到已安装的 ${BINARY}，将进行升级..."
        systemctl stop ${BINARY} 2>/dev/null || true
    fi

    echo ""
    info "===== 步骤 1/6: 安装 Node Agent 二进制 ====="
    install_binary "$arch" "$mode"

    echo ""
    info "===== 步骤 2/6: 安装 sing-box ====="
    install_singbox

    echo ""
    info "===== 步骤 3/6: 创建目录 ====="
    create_dirs

    echo ""
    info "===== 步骤 4/6: 生成配置 ====="
    install_config

    echo ""
    info "===== 步骤 5/7: 安装服务 ====="
    install_service

    echo ""
    info "===== 步骤 6/7: 优化系统网络 ====="
    optimize_system

    echo ""
    info "===== 步骤 7/7: 配置防火墙 & 启动 ====="
    configure_firewall
    start_service

    show_result
}

do_uninstall() {
    banner
    check_root

    info "停止服务..."
    systemctl stop ${BINARY} 2>/dev/null || true
    systemctl disable ${BINARY} 2>/dev/null || true

    info "删除文件..."
    rm -f "${INSTALL_DIR}/${BINARY}"
    rm -f /etc/systemd/system/${BINARY}.service
    systemctl daemon-reload

    echo ""
    ok "Node Agent 已卸载"
    warn "配置和数据目录保留: ${CONFIG_DIR} ${DATA_DIR}"
    warn "如需彻底删除: rm -rf ${CONFIG_DIR} ${DATA_DIR} ${LOG_DIR}"
}

show_usage() {
    echo ""
    echo -e "${CYAN}Node Agent 安装脚本${NC}"
    echo ""
    echo "用法: bash install.sh [命令] [选项]"
    echo ""
    echo "命令:"
    echo "  install     安装 (默认)"
    echo "  uninstall   卸载"
    echo ""
    echo "安装选项:"
    echo "  --local     使用本地二进制 (当前目录或 dist/ 目录)"
    echo "  --source    从源码编译安装 (自动安装 Go)"
    echo "  --github    从 GitHub Release 下载 (默认，失败回退源码编译)"
    echo ""
    echo -e "${YELLOW}推荐部署方式:${NC}"
    echo ""
    echo "  方式一: 上传编译好的二进制到 VPS"
    echo "    # 本地编译"
    echo "    make release"
    echo "    # 上传到 VPS"
    echo "    scp dist/node-agent-linux-amd64.tar.gz root@VPS_IP:/tmp/"
    echo "    # SSH 到 VPS 执行"
    echo "    cd /tmp && tar xzf node-agent-linux-amd64.tar.gz && bash install.sh --local"
    echo ""
    echo "  方式二: VPS 上直接从源码编译"
    echo "    bash install.sh --source"
    echo ""
    echo "  方式三: GitHub Release 已发布后"
    echo "    bash <(curl -fsSL https://raw.githubusercontent.com/${REPO}/main/deploy/install.sh)"
    echo ""
}

case "${1:-}" in
    uninstall|remove)
        do_uninstall
        ;;
    install|"")
        shift 2>/dev/null || true
        do_install "$@"
        ;;
    -h|--help|help)
        show_usage
        ;;
    *)
        do_install "$@"
        ;;
esac
