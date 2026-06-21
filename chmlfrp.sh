#!/bin/bash
set -o pipefail

###############################################################################
# Unraid + ChmlFrp 全自动控制脚本（单文件入口）
#
# 设计目标（科学化）：
# - 可靠判定：健康检查独立产出状态 JSON（online/offline + reason）
# - 智能决策：离线才 failover；fastest 带冷却避免抖动；失败节点 ban；多节点回退
# - 单行可用：适配 Unraid CA User Scripts（一个计划任务一行命令）
# - 不改 ChmlFrp API：仍使用既有 endpoints（login/node/nodeinfo/tunnel_config 等由 new_fix_flow 执行）
#
# 你只需要管理 2 个脚本：
# - chmlfrp.sh（本控制器）
# - new_fix_flow.sh（执行"全量同步到指定节点 + 重建 frpc 容器"）
#
# ─── API 文档与版本说明 ─────────────────────────────────────────────────────
# 官方 API 文档: https://docs.chmlfrp.net/API/v2/
# Apifox 在线文档: https://s.apifox.cn/24b31bd1-e48b-44ab-a486-81cf5f964422
#
# V2 API（cf-v2.uapis.cn）— 主力使用：
#   GET  /login?username=&password=     → 登录，获取 token
#   GET  /node                          → 节点列表（无参数，返回所有节点基本信息）
#   GET  /nodeinfo?token=&node=         → 节点详情（含 state/realIp/ip/port 等）
#   GET  /tunnel?token=                 → 隧道列表
#   POST /create_tunnel                 → 创建隧道
#   GET  /tunnel_config?token=&node=    → 生成 frpc 配置
#
# ⚠️  V2 删除隧道接口 /delete_tunnel 官方标注"还在开发中，暂时无法使用"
#     文档: https://docs.chmlfrp.net/API/v2/Tunnel_operations/delete_tunnel.html
#
# V1 API（cf-v1.uapis.cn）— 仅用于删除隧道：
#   GET  /api/deletetl.php?token=&userid=&nodeid=  → 删除隧道
#     文档: https://docs.chmlfrp.net/API/v1/Tunnel_operations/deletetl.html
#     注意: V1 官方已标注"即将被放弃使用"，但删除隧道目前只能用 V1
###############################################################################
# Unraid + ChmlFrp 全自动控制脚本（单文件入口）
#
# 设计目标（科学化）：
# - 可靠判定：健康检查独立产出状态 JSON（online/offline + reason）
# - 智能决策：离线才 failover；fastest 带冷却避免抖动；失败节点 ban；多节点回退
# - 单行可用：适配 Unraid CA User Scripts（一个计划任务一行命令）
# - 不改 ChmlFrp API：仍使用既有 endpoints（login/node/nodeinfo/tunnel_config 等由 new_fix_flow 执行）
#
# 你只需要管理 2 个脚本：
# - chmlfrp.sh（历史控制器脚本）
# - new_fix_flow.sh（历史执行脚本）
###############################################################################

# --- 基本路径：默认以脚本所在目录作为 LOG_DIR（历史脚本通常放在同目录便于归档） ---
BASE_DIR="$(cd "$(dirname "$0")" >/dev/null 2>&1 && pwd)"
LOG_DIR="${LOG_DIR:-$BASE_DIR}"

# --- 配置文件（建议与脚本同目录） ---
SETTINGS_FILE="${SETTINGS_FILE:-$LOG_DIR/settings.env}"
USERDATA_FILE="${USERDATA_FILE:-$LOG_DIR/userdata.txt}"
FIXED_TUNNEL_FILE="${FIXED_TUNNEL_FILE:-$LOG_DIR/fixed_tunnels.txt}"
EXEMPT_NAMES_FILE="${EXEMPT_NAMES_FILE:-$LOG_DIR/exempt_names.txt}"

# --- 输出/状态文件（沿用既有命名） ---
STATUS_FILE="${STATUS_FILE:-$LOG_DIR/chmlfrp-frpc在线测试.txt}"
NODE_FILE="${NODE_FILE:-$LOG_DIR/chmlfrp节点筛选.txt}"
USERINFO_FILE="${USERINFO_FILE:-$LOG_DIR/chmlfrp用户详情.txt}"

# --- frpc ---
FRPC_DOCKER_NAME="${FRPC_DOCKER_NAME:-frpc}"
FRPC_CONFIG_PATH="${FRPC_CONFIG_PATH:-/mnt/user/appdata/frpc/frpc.toml}"

# --- 节点筛选策略（可在 settings.env 覆盖） ---
FILTER_CHINA="${FILTER_CHINA:-all}"        # yes/no/all
FILTER_TYPE="${FILTER_TYPE:-all}"          # vip/user/all
FILTER_BUILD_SITE="${FILTER_BUILD_SITE:-all}" # yes/no/all
FILTER_NOTES="${FILTER_NOTES:-}"           # 关键字，逗号分隔；空=不过滤

# --- 自动化策略（可在 settings.env 覆盖） ---
NODE_REFRESH_SECONDS="${NODE_REFRESH_SECONDS:-3600}"
COOLDOWN_SECONDS="${COOLDOWN_SECONDS:-900}"
BAN_SECONDS="${BAN_SECONDS:-3600}"
MAX_TRIES="${MAX_TRIES:-3}"

PING_ATTEMPTS="${PING_ATTEMPTS:-5}"
PING_TIMEOUT="${PING_TIMEOUT:-2}"
MIN_SUCCESS="${MIN_SUCCESS:-3}"
SLEEP_BETWEEN="${SLEEP_BETWEEN:-0.2}"

# 可选：端到端健康检查 URL（例如 https://xxx/health）
HEALTHCHECK_URL="${HEALTHCHECK_URL:-}"

# --- QZhua OAuth2 配置 ---
QZHUA_API_BASE="${QZHUA_API_BASE:-https://account-api.qzhua.net}"
QZHUA_TOKEN_ENDPOINT="${QZHUA_TOKEN_ENDPOINT:-${QZHUA_API_BASE}/oauth2/token}"
QZHUA_DEVICE_CODE_ENDPOINT="${QZHUA_DEVICE_CODE_ENDPOINT:-${QZHUA_API_BASE}/oauth2/device_authorization}"
QZHUA_SCOPE="${QZHUA_SCOPE:-chmlfrp_api}"
TOKEN_EXPIRE_BUFFER="${TOKEN_EXPIRE_BUFFER:-60}"

# --- 内部文件 ---
LOCK_FILE="$LOG_DIR/.controller.lock"
COOLDOWN_FILE="$LOG_DIR/.controller.last_switch_ts"
BAN_FILE="$LOG_DIR/.controller.ban_nodes.json"
NODE_REFRESH_FILE="$LOG_DIR/.controller.last_node_refresh_ts"
NODES_METRICS_FILE="$LOG_DIR/chmlfrp节点测速.txt"

now_ts() { date +%s; }
log() { echo "[$(date '+%F %T')][$1] $2"; }
info() { log INFO "$1"; }
warn() { log WARNING "$1"; }
err() { log ERROR "$1"; }
success() { log SUCCESS "$1"; }

require_cmd() { command -v "$1" >/dev/null 2>&1; }

load_settings() {
  if [ -f "$SETTINGS_FILE" ]; then
    # shellcheck disable=SC1090
    set -a
    . "$SETTINGS_FILE"
    set +a
  fi
}

with_lock() {
  mkdir -p "$LOG_DIR" >/dev/null 2>&1 || true
  if require_cmd flock; then
    exec 200>"$LOCK_FILE"
    if ! flock -n 200; then
      warn "已有任务在运行，退出（lock=$LOCK_FILE）"
      exit 0
    fi
  else
    warn "系统未找到 flock，无法加锁（可能发生并发执行）"
  fi
}

json_ok() {
  local f="$1"
  [ -f "$f" ] || return 1
  require_cmd jq || return 1
  jq empty "$f" >/dev/null 2>&1
}

json_get() {
  local f="$1"; local q="$2"
  jq -r "$q" "$f" 2>/dev/null
}

write_status() {
  local status="$1" reason="$2" details="$3"
  if require_cmd jq; then
    jq -n --arg status "$status" --arg reason "$reason" --arg details "$details" --argjson ts "$(now_ts)" \
      '{status:$status, reason:$reason, details:$details, ts:$ts}' > "$STATUS_FILE"
  else
    printf '{"status":"%s","reason":"%s","details":"%s","ts":%s}\n' \
      "$status" "$reason" "${details//"/\\"}" "$(now_ts)" > "$STATUS_FILE"
  fi
}

parse_toml_kv() {
  local file="$1" key_regex="$2"
  grep -E "$key_regex" "$file" 2>/dev/null | head -n 1 | sed -E 's/^[[:space:]]*[^=]+=[[:space:]]*"?([^"#]+)"?.*$/\1/'
}

tcp_probe() {
  local host="$1" port="$2" timeout_secs="${3:-2}"
  require_cmd timeout || return 2
  timeout "$timeout_secs" bash -c "cat < /dev/null > /dev/tcp/${host}/${port}" >/dev/null 2>&1
}

http_probe() {
  local url="$1" timeout_secs="${2:-3}"
  [ -z "$url" ] && return 2
  require_cmd curl || return 2
  curl -fsS --max-time "$timeout_secs" "$url" >/dev/null 2>&1
}

health_check() {
  # 1) docker
  if ! require_cmd docker; then
    write_status offline docker_missing "docker 命令不存在"
    return 0
  fi
  if ! docker ps -a --format '{{.Names}}' | grep -wq "$FRPC_DOCKER_NAME"; then
    write_status offline container_missing "frpc 容器不存在"
    return 0
  fi
  local running
  running=$(docker inspect -f '{{.State.Running}}' "$FRPC_DOCKER_NAME" 2>/dev/null || echo false)
  if [ "$running" != "true" ]; then
    write_status offline container_not_running "frpc 容器未运行"
    return 0
  fi

  # 2) frpc.toml
  if [ ! -f "$FRPC_CONFIG_PATH" ]; then
    write_status offline frpc_config_missing "frpc.toml 不存在: $FRPC_CONFIG_PATH"
    return 0
  fi
  local server_addr server_port
  server_addr=$(parse_toml_kv "$FRPC_CONFIG_PATH" '^[[:space:]]*(server_addr|serverAddr)[[:space:]]*=')
  server_port=$(parse_toml_kv "$FRPC_CONFIG_PATH" '^[[:space:]]*(server_port|serverPort)[[:space:]]*=')
  if [ -z "$server_addr" ]; then
    write_status offline frpc_config_invalid "frpc.toml 未找到 server_addr"
    return 0
  fi
  if [ -n "$server_port" ]; then
    if ! tcp_probe "$server_addr" "$server_port" 2; then
      write_status offline tcp_connect_fail "TCP 连接失败: ${server_addr}:${server_port}"
      return 0
    fi
  fi

  # 3) 可选端到端
  if [ -n "$HEALTHCHECK_URL" ]; then
    if ! http_probe "$HEALTHCHECK_URL" 3; then
      write_status offline http_healthcheck_fail "HTTP 健康检查失败: $HEALTHCHECK_URL"
      return 0
    fi
  fi

  # 4) 日志关键错误
  local logs
  logs=$(docker logs --tail 80 "$FRPC_DOCKER_NAME" 2>/dev/null || true)
  if echo "$logs" | grep -q "start error: 客户端代理参数错误"; then
    write_status offline config_mismatch "检测到客户端代理参数错误"
    return 0
  fi
  if echo "$logs" | grep -Eqi "(connect to server.*failed|login to server failed|i/o timeout|connection refused|no route to host)"; then
    write_status offline server_connect_fail "检测到疑似服务端连接失败日志"
    return 0
  fi

  write_status online ok "container running; config ok; probes ok"
  return 0
}

read_status() {
  if [ ! -f "$STATUS_FILE" ]; then
    echo unknown
    return 0
  fi
  if json_ok "$STATUS_FILE"; then
    json_get "$STATUS_FILE" '.status // "unknown"'
  else
    grep -oE '"status"\s*:\s*"[^"]+"' "$STATUS_FILE" | head -n 1 | sed -E 's/.*"status"\s*:\s*"([^"]+)".*/\1/'
  fi
}

cooldown_ok() {
  [ -f "$COOLDOWN_FILE" ] || return 0
  local last now diff
  last=$(cat "$COOLDOWN_FILE" 2>/dev/null || echo 0)
  now=$(now_ts)
  diff=$(( now - last ))
  [ "$diff" -ge "$COOLDOWN_SECONDS" ]
}

mark_switched() { echo "$(now_ts)" > "$COOLDOWN_FILE"; }

init_ban_file() {
  if json_ok "$BAN_FILE"; then return 0; fi
  require_cmd jq || return 0
  jq -n '{banned:[]}' > "$BAN_FILE"
}

is_banned() {
  local name="$1"
  require_cmd jq || return 1
  json_ok "$BAN_FILE" || return 1
  local now
  now=$(now_ts)
  jq -e --arg n "$name" --argjson now "$now" '(.banned//[])|map(select(.name==$n and (.until//0)>$now))|length>0' "$BAN_FILE" >/dev/null 2>&1
}

ban_node() {
  local name="$1" reason="$2"
  require_cmd jq || return 0
  init_ban_file
  local until
  until=$(( $(now_ts) + BAN_SECONDS ))
  jq --arg n "$name" --arg r "$reason" --argjson u "$until" '
    .banned = ((.banned//[]) | map(select(.name!=$n)) + [{name:$n, reason:$r, until:$u}])
  ' "$BAN_FILE" > "${BAN_FILE}.tmp" && mv "${BAN_FILE}.tmp" "$BAN_FILE"
}

urlencode() {
  local s="$1"
  curl -s -o /dev/null -w '%{url_effective}' --get --data-urlencode "name=$s" 'http://dummy' | sed 's/.*name=//g'
}

userinfo_sync() {
  require_cmd curl || { err "缺少 curl"; return 1; }
  require_cmd jq || { err "缺少 jq"; return 1; }

  if ! json_ok "$USERDATA_FILE"; then
    err "userdata.txt（内部为 JSON）不存在或非法：$USERDATA_FILE"
    return 1
  fi
  local u p
  u=$(json_get "$USERDATA_FILE" '.chmlfrp.username // empty')
  p=$(json_get "$USERDATA_FILE" '.chmlfrp.password // empty')
  if [ -z "$u" ] || [ -z "$p" ]; then
    err "userdata.txt（JSON）缺少 chmlfrp.username/chmlfrp.password（用于 login）"
    return 1
  fi
  local url resp code
  url="http://cf-v2.uapis.cn/login?username=${u}&password=${p}"
  info "同步用户详情（login）"
  resp=$(curl -sS -L --connect-timeout 5 --max-time 15 "$url" || true)
  if [ -z "$resp" ] || ! echo "$resp" | jq empty >/dev/null 2>&1; then
    err "login API 返回空或非 JSON"
    return 1
  fi
  code=$(echo "$resp" | jq -r '.code // 0')
  if [ "$code" -ne 200 ]; then
    err "login 失败：code=$code msg=$(echo "$resp" | jq -r '.msg // ""')"
    return 1
  fi
  echo "$resp" | jq '.' > "$USERINFO_FILE"
  success "已写入用户详情：$USERINFO_FILE"
  return 0
}

ensure_userinfo() {
  # 确保 USERINFO_FILE 存在且合法，否则尝试同步一次
  if json_ok "$USERINFO_FILE"; then
    return 0
  fi
  info "用户详情缺失或非法，尝试自动同步：$USERINFO_FILE"
  userinfo_sync || true
  json_ok "$USERINFO_FILE"
}

node_refresh_needed() {
  local now last diff
  now=$(now_ts)
  last=$(cat "$NODE_REFRESH_FILE" 2>/dev/null || echo 0)
  diff=$(( now - last ))
  if [ "$diff" -ge "$NODE_REFRESH_SECONDS" ]; then return 0; fi
  if ! json_ok "$NODE_FILE"; then return 0; fi
  return 1
}

node_list_refresh() {
  # 生成 NODE_FILE（JSON）：{nodes:[{节点名称,节点本地IPv4,...}]}
  require_cmd curl || { err "缺少 curl"; return 1; }
  require_cmd jq || { err "缺少 jq"; return 1; }

  # token：优先 userdata.txt（JSON）的 chmlfrp.token
  if ! json_ok "$USERDATA_FILE"; then
    err "userdata.txt（内部为 JSON）不存在或非法：$USERDATA_FILE"
    return 1
  fi
  local token
  token=$(json_get "$USERDATA_FILE" '.chmlfrp.token // empty')
  if [ -z "$token" ]; then
    err "userdata.txt（JSON）缺少 chmlfrp.token"
    return 1
  fi

  info "刷新节点列表：/node"
  local node_api_url="http://cf-v2.uapis.cn/node"
  info "调用 API: GET $node_api_url"
  local resp
  resp=$(curl -sS -L --connect-timeout 5 --max-time 15 "$node_api_url" || true)
  if [ -z "$resp" ] || ! echo "$resp" | jq empty >/dev/null 2>&1; then
    err "node API 返回空或非 JSON"
    return 1
  fi
  if [ "$(echo "$resp" | jq -r '.code // 0')" != "200" ]; then
    err "node API 失败：code=$(echo "$resp" | jq -r '.code // 0')"
    return 1
  fi

  # --- 输出 API 返回的全部原始节点 ---
  local total_raw
  total_raw=$(echo "$resp" | jq '.data | length')
  info "API 返回全部节点（共 $total_raw 个）："
  echo "$resp" | jq -r '.data[] | "  [\(.nodegroup)] \(.name) | 地区:\(.area) | 中国:\(.china) | 建站:\(.web)"' | while IFS= read -r line; do
    info "$line"
  done

  # 过滤策略：基于列表字段（china/nodegroup/web/notes）
  local notes_filter_json
  if [ -n "$FILTER_NOTES" ]; then
    # 转成数组
    notes_filter_json=$(printf '%s' "$FILTER_NOTES" | jq -R 'split(",") | map(gsub("^\\s+|\\s+$";"")) | map(select(length>0))')
  else
    notes_filter_json='[]'
  fi

  echo "$resp" | jq --arg fc "$FILTER_CHINA" --arg ft "$FILTER_TYPE" --arg fw "$FILTER_BUILD_SITE" --argjson fn "$notes_filter_json" '
    def match_yes_no_all(v; req):
      if req=="all" then true
      elif req=="yes" then (v=="yes" or v==true)
      elif req=="no" then (v=="no" or v==false)
      else true end;
    def match_group(v; req):
      if req=="all" then true else (v==req) end;
    def match_notes(n; arr):
      if (arr|length)==0 then true
      else any(arr[]; (n//"") | tostring | contains(.)) end;

    {nodes: [
      .data[]
      | select(match_yes_no_all(.china; $fc))
      | select(match_group(.nodegroup; $ft))
      | select(match_yes_no_all(.web; $fw))
      | select(match_notes(.notes; $fn))
      | {
          "节点名称": .name,
          "节点本地IPv4": (.realIp // ""),
          "节点地区": .area,
          "权限组": .nodegroup,
          "是否中国节点": .china,
          "是否支持建站": .web,
          "节点介绍": (.notes // "")
        }
    ]}
  ' > "$NODE_FILE"

  # --- 输出筛选详情 ---
  local total_before total_after
  total_before=$(echo "$resp" | jq '.data | length')
  total_after=$(jq '.nodes | length' "$NODE_FILE")

  # 构建筛选条件描述
  local filters=""
  [ "$FILTER_CHINA" != "all" ] && filters+=" 中国=$FILTER_CHINA"
  [ "$FILTER_TYPE" != "all" ] && filters+=" 权限组=$FILTER_TYPE"
  [ "$FILTER_BUILD_SITE" != "all" ] && filters+=" 建站=$FILTER_BUILD_SITE"
  [ -n "$FILTER_NOTES" ] && filters+=" 关键字=$FILTER_NOTES"
  [ -z "$filters" ] && filters="无（全部节点）"

  info "节点筛选完成：原始 $total_before 个 → 筛选后 $total_after 个（条件:$filters）"

  # 列出筛选后的节点名称
  if [ "$total_after" -gt 0 ]; then
    local node_names
    node_names=$(jq -r '.nodes[].节点名称' "$NODE_FILE" | tr '\n' '、')
    info "筛选后节点列表：${node_names%、}"
  else
    warn "筛选后节点列表为空！请检查筛选条件是否过于严格"
  fi

  echo "$(now_ts)" > "$NODE_REFRESH_FILE"
  success "节点列表已写入：$NODE_FILE"
  return 0
}

nodeinfo_get() {
  local token="$1" name="$2"
  local enc url
  enc=$(urlencode "$name")
  url="http://cf-v2.uapis.cn/nodeinfo?token=${token}&node=${enc}"
  curl -sS -L --connect-timeout 3 --max-time 6 "$url"
}

# 网络连通性诊断：测试多个外网地址，判断是 token 问题还是网络问题
network_diagnosis() {
  info "=== 网络连通性诊断 ==="
  local all_ok=true

  # 测试 ChmlFrp API 服务器
  if curl -fsS --max-time 5 -o /dev/null "http://cf-v2.uapis.cn/" 2>/dev/null; then
    info "  ✅ ChmlFrp API (cf-v2.uapis.cn) 可达"
  else
    warn "  ❌ ChmlFrp API (cf-v2.uapis.cn) 不可达"
    all_ok=false
  fi

  # 测试国内常用站点
  local test_sites=(
    "https://www.baidu.com|百度"
    "https://www.bing.com|Bing"
    "https://www.weibo.com|微博"
    "https://www.cctv.com|CCTV"
  )

  for site_entry in "${test_sites[@]}"; do
    local url="${site_entry%%|*}"
    local label="${site_entry##*|}"
    if curl -fsS --max-time 5 -o /dev/null "$url" 2>/dev/null; then
      info "  ✅ $label ($url) 可达"
    else
      warn "  ❌ $label ($url) 不可达"
      all_ok=false
    fi
  done

  if $all_ok; then
    info "=== 网络连通性正常，问题出在 token 认证（可能已过期或无效）==="
  else
    warn "=== 部分外网不可达，请检查本机网络/DNS 设置 ==="
  fi
}

ping_quality() {
  # 输出："avg_ms\tloss_pct\treceived"
  local target="$1"
  local out
  out=$(ping -c "$PING_ATTEMPTS" -W "$PING_TIMEOUT" "$target" 2>/dev/null || true)
  [ -n "$out" ] || return 1

  # 解析收包
  local received
  received=$(echo "$out" | awk -F',' '/packets transmitted/ {gsub(/^[[:space:]]+/,"",$2); print $2}' | awk '{print $1}' | head -n 1)
  [ -n "$received" ] || received=0
  if [ "$received" -lt "$MIN_SUCCESS" ]; then
    return 1
  fi

  # 解析丢包百分比
  local loss
  loss=$(echo "$out" | awk -F',' '/packet loss/ {print $3}' | sed -E 's/[^0-9.]+//g' | head -n 1)
  [ -n "$loss" ] || loss=100

  # 解析 avg RTT
  local avg
  avg=$(echo "$out" | sed -nE 's/^(rtt|round-trip)[^=]*= ([0-9.]+)\/([0-9.]+)\/.*/\3/p' | head -n 1)
  if [ -z "$avg" ]; then
    # 退化：从 time= 的行做粗略平均
    avg=$(echo "$out" | awk -F'time=' '/time=/{print $2}' | awk '{print $1}' | awk '{s+=$1; c++} END{if(c>0) printf("%.0f", s/c); else print ""}')
  fi
  [ -n "$avg" ] || return 1

  printf '%.0f\t%.0f\t%s\n' "$avg" "$loss" "$received"
  return 0
}

select_best_node() {
  # 注意：本函数会被 auto_failover/auto_fastest 通过 $() 调用，
  # 所有日志必须走 stderr（>&2），否则会污染命令替换捕获的输出。
  # 唯一走 stdout 的是最终的候选行（score<TAB>name<TAB>...）。
  _sbn_log() { echo "[$(date '+%F %T')][$1] $2" >&2; }

  require_cmd jq || { _sbn_log ERROR "缺少 jq"; return 1; }
  require_cmd curl || { _sbn_log ERROR "缺少 curl"; return 1; }

  if ! json_ok "$USERDATA_FILE"; then
    _sbn_log ERROR "userdata.txt（内部为 JSON）不存在或非法：$USERDATA_FILE"
    return 1
  fi
  local token
  token=$(get_access_token)
  if [ -z "$token" ]; then
    _sbn_log ERROR "无法获取有效的 access_token"
    return 1
  fi
  if ! json_ok "$NODE_FILE"; then
    _sbn_log ERROR "节点文件不存在或非法：$NODE_FILE"
    return 1
  fi

  local total
  total=$(jq '.nodes | length' "$NODE_FILE")
  [ "$total" -gt 0 ] || { _sbn_log ERROR "节点文件 nodes 为空"; return 1; }

  local candidates=()
  local metrics_tmp
  metrics_tmp="${NODES_METRICS_FILE}.tmp"
  printf "#ts\tscore\tname\tavg_ms\tloss_pct\tping_target\tip_for_dns\n" > "$metrics_tmp"
  local i
  _sbn_log INFO "开始遍历 $total 个节点进行测速选优..."
  local auth_error_count=0
  for ((i=0; i<total; i++)); do
    local name ip_from_file
    name=$(jq -r ".nodes[$i][\"节点名称\"] // \"\"" "$NODE_FILE")
    ip_from_file=$(jq -r ".nodes[$i][\"节点本地IPv4\"] // \"\"" "$NODE_FILE")
    [ -z "$name" ] && continue

    if is_banned "$name"; then
      _sbn_log INFO "跳过 ban 节点：$name"
      continue
    fi

    local resp
    local name_encoded
    name_encoded=$(urlencode "$name")
    local nodeinfo_url="http://cf-v2.uapis.cn/nodeinfo?token=${token}&node=${name_encoded}"
    _sbn_log INFO "调用 nodeinfo: $nodeinfo_url"
    resp=$(nodeinfo_get "$token" "$name")
    if [ -z "$resp" ] || ! echo "$resp" | jq empty >/dev/null 2>&1; then
      _sbn_log INFO "节点 [$name] 被跳过：nodeinfo API 返回空或非 JSON"
      continue
    fi
    local code state real_ip domain_ip
    code=$(echo "$resp" | jq -r '.code // 0')
    if [ "$code" -ne 200 ]; then
      # 401 = 未授权（token 无效/过期），连续出现说明不是单个节点问题
      if [ "$code" -eq 401 ]; then
        auth_error_count=$((auth_error_count + 1))
        if [ "$auth_error_count" -eq 1 ]; then
          _sbn_log WARNING "⚠️  nodeinfo 返回 code=401（未授权），token 可能已过期或无效"
          _sbn_log INFO "触发网络连通性诊断（判断是 token 问题还是本地网络问题）..."
          network_diagnosis >&2
        fi
        if [ "$auth_error_count" -ge 3 ]; then
          _sbn_log ERROR "连续 $auth_error_count 个节点返回 401，确认 token 无效，终止测速"
          _sbn_log ERROR "请检查 userdata.txt 中的 chmlfrp.token 是否已过期（可重新登录获取）"
          return 1
        fi
      fi
      _sbn_log INFO "节点 [$name] 被跳过：nodeinfo code=$code"
      continue
    fi
    state=$(echo "$resp" | jq -r '.data.state // "unknown"')
    if [ "$state" != "online" ]; then
      _sbn_log INFO "节点 [$name] 被跳过：state=$state（非 online）"
      continue
    fi
    real_ip=$(echo "$resp" | jq -r '.data.realIp // ""')
    domain_ip=$(echo "$resp" | jq -r '.data.ip // ""')

    local ping_target
    ping_target="$real_ip"
    if [ -z "$ping_target" ] || [ "$ping_target" = "null" ]; then ping_target="$ip_from_file"; fi
    if [ -z "$ping_target" ] || [ "$ping_target" = "null" ]; then ping_target="$domain_ip"; fi
    if [ -z "$ping_target" ] || [ "$ping_target" = "null" ]; then
      _sbn_log INFO "节点 [$name] 被跳过：realIp='$real_ip', 节点IPv4='$ip_from_file', domainIp='$domain_ip' 全部为空"
      continue
    fi

    local pq avg loss
    pq=$(ping_quality "$ping_target" || true)
    if [ -z "$pq" ]; then
      _sbn_log INFO "节点 [$name] 被跳过：ping 失败（ping_target=$ping_target）"
      continue
    fi
    avg=$(printf '%s' "$pq" | cut -f1)
    loss=$(printf '%s' "$pq" | cut -f2)

    local ip_for_dns
    ip_for_dns="$real_ip"
    if [ -z "$ip_for_dns" ] || [ "$ip_for_dns" = "null" ]; then ip_for_dns="$ping_target"; fi

    # 评分：score = avg_ms + loss_pct*30
    local score
    score=$(( avg + loss * 30 ))
    _sbn_log INFO "节点 [$name] 入选候选：score=$score (avg=${avg}ms, loss=${loss}%, ping_target=$ping_target)"
    candidates+=("${score}"$'\t'"${name}"$'\t'"${ip_for_dns}"$'\t'"${ping_target}"$'\t'"avg=${avg}ms loss=${loss}%")
    printf "%s\t%s\t%s\t%s\t%s\t%s\t%s\n" "$(now_ts)" "$score" "$name" "$avg" "$loss" "$ping_target" "$ip_for_dns" >> "$metrics_tmp"
  done

  # 写出测速结果，便于你在 Unraid 里直接查看
  mv "$metrics_tmp" "$NODES_METRICS_FILE" 2>/dev/null || true

  [ ${#candidates[@]} -gt 0 ] || { _sbn_log ERROR "没有可用在线节点（nodeinfo online + ping 稳定）"; return 1; }

  # 按 score 升序排序，输出全部候选行（供 auto_failover 多轮回退使用）
  printf '%s\n' "${candidates[@]}" | sort -n -t $'\t' -k1,1
  return 0
}

current_node_name() {
  # 通过 frpc.toml 的 server_addr 反查 NODE_FILE
  [ -f "$FRPC_CONFIG_PATH" ] || { echo ""; return 0; }
  local ip
  ip=$(parse_toml_kv "$FRPC_CONFIG_PATH" '^[[:space:]]*(server_addr|serverAddr)[[:space:]]*=')
  [ -n "$ip" ] || { echo ""; return 0; }
  if json_ok "$NODE_FILE"; then
    jq -r --arg ip "$ip" '.nodes[] | select(."节点本地IPv4"==$ip) | ."节点名称"' "$NODE_FILE" | head -n 1
  else
    echo ""
  fi
}

apply_switch_to_node() {
  local node_name="$1"
  local fix="$LOG_DIR/new_fix_flow.sh"
  if [ ! -f "$fix" ]; then
    err "找不到 $fix"
    exit 1
  fi
    err "找不到 new_fix_flow.sh（请把它放到同目录）"
    return 1
  fi

  # 删除隧道需要 userid；尽量保证 userinfo 存在
  ensure_userinfo || warn "用户详情仍不可用：删除隧道可能失败（但仍继续尝试修复）"

  bash "$fix" --force-run --node "$node_name"
}

# 固定隧道文件变更必重建：做一次“同步检查”（不带 --force-run；未变更时会快速退出）
sync_fix_flow_to_node() {
  local node_name="$1"
  [ -n "$node_name" ] || return 0

  local fix="$LOG_DIR/new_fix_flow.sh"
  if [ ! -f "$fix" ]; then
    fix="$BASE_DIR/new_fix_flow.sh"
  fi
  if [ ! -f "$fix" ]; then
    warn "找不到 new_fix_flow.sh，跳过同步检查"
    return 0
  fi

  ensure_userinfo || warn "用户详情仍不可用：同步过程中删除隧道可能失败（但仍继续尝试）"
  bash "$fix" --node "$node_name"
}

auto_failover() {
  with_lock
  load_settings

  if node_refresh_needed; then
    node_list_refresh || true
  fi

  # 即使在线也要做"固定隧道变更同步检查"（满足：改了 fixed_tunnels.txt 必重建）
  local cur
  cur=$(current_node_name)
  if [ -n "$cur" ]; then
    info "failover：同步检查当前节点固定隧道变更（node=$cur）"
    sync_fix_flow_to_node "$cur" || true
  else
    warn "failover：当前节点未知（无法从 frpc.toml 反查），跳过固定隧道同步检查"
  fi

  health_check
  local st
  st=$(read_status)
  info "当前状态：$st"
  if [ "$st" = "online" ]; then
    info "在线，failover 不动作"
    return 0
  fi

  init_ban_file

  # 一次性选出所有候选节点（按 score 升序排列）
  # select_best_node 内部日志走 stderr，stdout 只输出候选行
  local all_candidates
  all_candidates=$(select_best_node || true)
  if [ -z "$all_candidates" ]; then
    err "无法选出任何候选节点（所有节点均不可用），failover 退出"
    return 1
  fi

  local total_candidates
  total_candidates=$(echo "$all_candidates" | wc -l | tr -d ' ')
  info "选出 $total_candidates 个候选节点，将依次尝试切换..."

  local tried=0
  while IFS=$'\t' read -r score name ip_for_dns ping_target dbg; do
    [ -z "$name" ] && continue

    # 防御性校验：score 必须为纯数字
    if ! [[ "$score" =~ ^[0-9]+$ ]]; then
      warn "候选行格式异常（score 非数字）: [$score\t$name]，跳过"
      continue
    fi

    tried=$((tried + 1))
    info "尝试 #$tried/$total_candidates：切换到节点=$name (score=$score, $dbg)"

    if ! apply_switch_to_node "$name"; then
      warn "切换执行失败：$name"
      ban_node "$name" fix_flow_failed
      continue
    fi
    mark_switched

    health_check
    st=$(read_status)
    info "切换后状态：$st"
    if [ "$st" = "online" ]; then
      success "自愈成功：$name"
      return 0
    fi
    warn "切换后仍离线，ban 并回退：$name"
    ban_node "$name" post_switch_offline
  done <<< "$all_candidates"

  if [ "$tried" -eq 0 ]; then
    err "候选列表为空，无可用节点"
  else
    err "已回退尝试 $tried 次仍未恢复在线"
  fi
  return 1
}

auto_fastest() {
  with_lock
  load_settings

  if ! cooldown_ok; then
    info "冷却中（${COOLDOWN_SECONDS}s），fastest 不动作"
    return 0
  fi
  if node_refresh_needed; then
    node_list_refresh || true
  fi

  init_ban_file

  local cur
  cur=$(current_node_name)
  [ -n "$cur" ] && info "当前节点：$cur" || info "当前节点：未知（无法从 frpc.toml + node 文件反查）"

  local all_candidates
  all_candidates=$(select_best_node || true)
  [ -n "$all_candidates" ] || { err "无法选出任何候选节点"; return 1; }

  # select_best_node 现在输出多行（按 score 升序），取第一行即最优
  local best_line
  best_line=$(echo "$all_candidates" | head -n 1)

  local score name dbg
  score=$(printf '%s' "$best_line" | cut -f1)
  name=$(printf '%s' "$best_line" | cut -f2)
  dbg=$(printf '%s' "$best_line" | cut -f5)

  # 防御性校验
  if ! [[ "$score" =~ ^[0-9]+$ ]]; then
    err "select_best_node 输出格式异常（非数字 score）: [$best_line]"
    return 1
  fi
  [ -n "$name" ] || { err "选出节点名称为空"; return 1; }

  info "最优节点：$name (score=$score, $dbg)"

  if [ -n "$cur" ] && [ "$cur" = "$name" ]; then
    info "最优节点与当前一致，不切换"
    return 0
  fi

  info "fastest：执行切换到 $name"
  if apply_switch_to_node "$name"; then
    mark_switched
    health_check
    local st
    st=$(read_status)
    info "切换后状态：$st"
    [ "$st" = "online" ] && success "fastest 切换完成：$name" || warn "fastest 切换后未在线，请观察日志"
    return 0
  fi
  err "fastest 切换执行失败：$name"
  return 1
}

manual_switch() {
  with_lock
  load_settings
  # CA User Scripts 的参数输入不支持“一个参数里包含空格”。
  # 因此：
  # - 若传了参数：把剩余参数拼成节点名称
  # - 若没传参数：从 $LOG_DIR/manual_node.txt 读取节点名称（历史方式）
  local node_name="$*"
  if [ -z "$node_name" ]; then
    local f="$LOG_DIR/manual_node.txt"
    if [ -f "$f" ]; then
      node_name=$(head -n 1 "$f" 2>/dev/null | sed -E 's/^[[:space:]]+//; s/[[:space:]]+$//')
    fi
  fi
  [ -n "$node_name" ] || {
    err "manual 缺少节点名称：请传参，或在 $LOG_DIR/manual_node.txt 第一行写入节点名称";
    return 2;
  }
  info "手动切换到节点：$node_name"
  apply_switch_to_node "$node_name"
}

usage() {
  cat <<EOF
用法：
  $0 health                      # 健康检查
  $0 failover                   # 故障自愈
  $0 fastest                    # 主动选最优
  $0 manual "节点名称"          # 手动切换节点
  $0 userinfo                   # 同步用户详情
  $0 nodes                      # 刷新节点列表
  $0 oauth_refresh               # 刷新 OAuth2 token
  $0 oauth_reauth               # 重新进行 OAuth2 授权

给 Unraid CA User Scripts 的示例命令：
  bash "$LOG_DIR/chmlfrp.sh" health
  bash "$LOG_DIR/chmlfrp.sh" failover
  bash "$LOG_DIR/chmlfrp.sh" fastest
EOF
}

###############################################################################
# QZhua OAuth2 Token 管理
###############################################################################

# 检查是否启用 OAuth2
is_oauth2_enabled() {
  if ! json_ok "$USERDATA_FILE"; then
    return 1
  fi
  local enabled
  enabled=$(json_get "$USERDATA_FILE" '.chmlfrp.oauth2.enabled // false')
  [ "$enabled" = "true" ]
}

# 获取 token 过期时间戳
get_token_expires_at() {
  json_get "$USERDATA_FILE" '.chmlfrp.oauth2.token_expires_at // 0'
}

# 检查 token 是否过期（预留 60 秒缓冲）
is_token_expired() {
  local expires_at now
  expires_at=$(get_token_expires_at)
  now=$(now_ts)
  [ "$expires_at" -gt 0 ] && [ $((now + TOKEN_EXPIRE_BUFFER)) -ge "$expires_at" ]
}

# 更新 OAuth2 token 到配置文件
update_oauth2_token() {
  local access_token="$1"
  local refresh_token="$2"
  local expires_in="$3"
  
  local now expires_at
  now=$(now_ts)
  expires_at=$((now + expires_in))
  
  if json_ok "$USERDATA_FILE"; then
    local tmp_file="${USERDATA_FILE}.tmp"
    jq --arg at "$access_token" \
       --arg rt "$refresh_token" \
       --argjson ea "$expires_at" \
       '.chmlfrp.oauth2.access_token = $at | .chmlfrp.oauth2.refresh_token = $rt | .chmlfrp.oauth2.token_expires_at = $ea' \
       "$USERDATA_FILE" > "$tmp_file" && mv "$tmp_file" "$USERDATA_FILE"
    info "Token 已保存到 $USERDATA_FILE"
  fi
}

# 用 refresh_token 刷新 access_token
refresh_access_token() {
  require_cmd curl || { err "缺少 curl"; return 1; }
  
  local refresh_token
  refresh_token=$(json_get "$USERDATA_FILE" '.chmlfrp.oauth2.refresh_token // empty')
  
  if [ -z "$refresh_token" ]; then
    err "没有 refresh_token，需要重新授权"
    return 1
  fi
  
  local client_id client_secret
  client_id=$(json_get "$USERDATA_FILE" '.chmlfrp.oauth2.client_id // empty')
  client_secret=$(json_get "$USERDATA_FILE" '.chmlfrp.oauth2.client_secret // empty')
  
  if [ -z "$client_id" ] || [ -z "$client_secret" ]; then
    err "缺少 client_id 或 client_secret"
    return 1
  fi
  
  info "正在刷新 access_token..."
  
  local resp
  resp=$(curl -sS -X POST "$QZHUA_TOKEN_ENDPOINT" \
    -u "${client_id}:${client_secret}" \
    -H "Content-Type: application/x-www-form-urlencoded" \
    -d "grant_type=refresh_token" \
    -d "refresh_token=${refresh_token}")
  
  if echo "$resp" | jq -e '.access_token' >/dev/null 2>&1; then
    local new_access_token new_refresh_token expires_in
    new_access_token=$(echo "$resp" | jq -r '.access_token')
    new_refresh_token=$(echo "$resp" | jq -r '.refresh_token // "'"$refresh_token"'"')
    expires_in=$(echo "$resp" | jq -r '.expires_in // 3600')
    
    update_oauth2_token "$new_access_token" "$new_refresh_token" "$expires_in"
    success "access_token 刷新成功"
    return 0
  else
    err "refresh_token 刷新失败: $(echo "$resp" | jq -r '.error // "未知错误"')"
    return 1
  fi
}

# 设备码授权流程（首次授权或 token 全部失效）
device_code_auth() {
  require_cmd curl || { err "缺少 curl"; return 1; }
  
  local client_id client_secret
  client_id=$(json_get "$USERDATA_FILE" '.chmlfrp.oauth2.client_id // empty')
  client_secret=$(json_get "$USERDATA_FILE" '.chmlfrp.oauth2.client_secret // empty')
  
  if [ -z "$client_id" ] || [ -z "$client_secret" ]; then
    err "缺少 client_id 或 client_secret"
    return 1
  fi
  
  info "获取设备码..."
  
  local resp
  resp=$(curl -sS -X POST "$QZHUA_DEVICE_CODE_ENDPOINT" \
    -u "${client_id}:${client_secret}" \
    -H "Content-Type: application/x-www-form-urlencoded" \
    -d "scope=${QZHUA_SCOPE}")
  
  if ! echo "$resp" | jq empty >/dev/null 2>&1; then
    err "获取设备码失败: $resp"
    return 1
  fi
  
  local device_code user_code verification_uri_complete
  device_code=$(echo "$resp" | jq -r '.device_code')
  user_code=$(echo "$resp" | jq -r '.user_code')
  verification_uri_complete=$(echo "$resp" | jq -r '.verification_uri_complete')
  
  if [ -z "$device_code" ] || [ "$device_code" = "null" ]; then
    err "获取 device_code 失败"
    return 1
  fi
  
  echo ""
  echo "========================================"
  echo "请在 5 分钟内完成以下授权操作："
  echo "========================================"
  echo ""
  echo "1. 在浏览器打开以下链接："
  echo "   $verification_uri_complete"
  echo ""
  echo "2. 或访问: https://account-api.qzhua.net/oauth-device-verify"
  echo ""
  echo "3. 输入用户代码: $user_code"
  echo ""
  echo "4. 使用 QZhua 账号登录并授权"
  echo ""
  echo "========================================"
  echo ""
  
  info "轮询获取 access_token（请在浏览器完成授权后）..."
  
  local interval=5
  local max_attempts=60
  
  for ((i=0; i<max_attempts; i++)); do
    sleep "$interval"
    
    resp=$(curl -sS -X POST "$QZHUA_TOKEN_ENDPOINT" \
      -u "${client_id}:${client_secret}" \
      -H "Content-Type: application/x-www-form-urlencoded" \
      -d "grant_type=urn:ietf:params:oauth:grant-type:device_code" \
      -d "device_code=${device_code}")
    
    if echo "$resp" | jq -e '.access_token' >/dev/null 2>&1; then
      local access_token refresh_token expires_in
      access_token=$(echo "$resp" | jq -r '.access_token')
      refresh_token=$(echo "$resp" | jq -r '.refresh_token // ""')
      expires_in=$(echo "$resp" | jq -r '.expires_in // 3600')
      
      update_oauth2_token "$access_token" "$refresh_token" "$expires_in"
      success "授权成功！access_token 已保存"
      return 0
    fi
    
    local error
    error=$(echo "$resp" | jq -r '.error // "unknown"')
    
    if [ "$error" = "authorization_pending" ]; then
      info "等待授权... ($((i+1))/${max_attempts})"
      continue
    elif [ "$error" = "slow_down" ]; then
      info "请求过于频繁，等待..."
      continue
    else
      err "授权失败: $error"
      return 1
    fi
  done
  
  err "授权超时，请重试"
  return 1
}

# 获取有效的 access_token（主入口）
get_access_token() {
  if ! is_oauth2_enabled; then
    err "OAuth2 未启用，请在 userdata.txt 中启用 oauth2"
    return 1
  fi
  
  local access_token
  access_token=$(json_get "$USERDATA_FILE" '.chmlfrp.oauth2.access_token // empty')
  
  if [ -z "$access_token" ] || [ "$access_token" = "null" ]; then
    info "未找到 access_token，尝试刷新..."
    if refresh_access_token; then
      access_token=$(json_get "$USERDATA_FILE" '.chmlfrp.oauth2.access_token // empty')
    else
      error "========================================"
      error "OAuth2 Token 完全失效，需要重新授权！"
      error "请运行以下命令完成授权："
      error "  ./chmlfrp.sh oauth_reauth"
      error "或："
      error "  bash /mnt/user/Hdd_Disk_Share/脚本日志/chmlfrp/chmlfrp.sh oauth_reauth"
      error "========================================"
      return 1
    fi
  elif is_token_expired; then
    info "access_token 已过期，尝试刷新..."
    if ! refresh_access_token; then
      error "========================================"
      error "OAuth2 Token 刷新失败，需要重新授权！"
      error "请运行以下命令完成授权："
      error "  ./chmlfrp.sh oauth_reauth"
      error "========================================"
      return 1
    else
      access_token=$(json_get "$USERDATA_FILE" '.chmlfrp.oauth2.access_token // empty')
    fi
  fi
  
  echo "$access_token"
}

# 新增命令：刷新 token
oauth_token_refresh() {
  with_lock
  load_settings
  
  if ! is_oauth2_enabled; then
    err "OAuth2 未启用，请先在 userdata.txt 中配置 oauth2"
    return 1
  fi
  
  if ! refresh_access_token; then
    err "Token 刷新失败"
    return 1
  fi
  
  success "Token 刷新成功"
  return 0
}

# 新增命令：重新授权
oauth_reauth() {
  with_lock
  load_settings
  
  if ! is_oauth2_enabled; then
    err "OAuth2 未启用，请先在 userdata.txt 中配置 oauth2"
    return 1
  fi
  
  if ! device_code_auth; then
    err "授权失败"
    return 1
  fi
  
  success "授权成功"
  return 0
}

main() {
  load_settings
  case "${1:-}" in
    health)        health_check ;;
    failover)      auto_failover ;;
    fastest)       auto_fastest ;;
    manual)        shift; manual_switch "$*" ;;
    userinfo)      with_lock; load_settings; userinfo_sync ;;
    nodes)          with_lock; load_settings; node_list_refresh ;;
    oauth_refresh)  oauth_token_refresh ;;
    oauth_reauth)   oauth_reauth ;;
    -h|--help|help|"") usage ;;
    *) err "未知命令: $1"; usage; exit 2 ;;
  esac
}

main "$@"
