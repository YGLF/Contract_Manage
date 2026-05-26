#!/usr/bin/env bash

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"
ENV_FILE="${PROJECT_ROOT}/.env"
COMPOSE_FILE="${COMPOSE_FILE:-${PROJECT_ROOT}/docker-compose.microservices.yml}"
COMPOSE_BASENAME="$(basename "${COMPOSE_FILE}")"
DOCKER_COMPOSE=()
DOCKER_COMPOSE_LABEL=""
TMP_BODY_FILE=""
# 生产验证可显式执行:
#   COMPOSE_FILE="${PROJECT_ROOT}/docker-compose.production.yml" bash scripts/health-check.sh

cd "${PROJECT_ROOT}"

FAILED=0
GATEWAY_PORT="${GATEWAY_SERVICE_PORT:-8100}"
GATEWAY_BASE_URL="http://127.0.0.1:${GATEWAY_PORT}"

pass() {
  printf '[PASS] %s\n' "$1"
}

warn() {
  printf '[WARN] %s\n' "$1"
}

fail() {
  printf '[FAIL] %s\n' "$1"
  FAILED=1
}

load_env_file() {
  local file="$1"
  local line=""
  local key=""
  local value=""
  while IFS= read -r line || [[ -n "${line}" ]]; do
    line="${line%$'\r'}"
    [[ -z "${line}" || "${line}" == \#* ]] && continue
    [[ "${line}" == export\ * ]] && line="${line#export }"
    if [[ "${line}" != *=* ]]; then
      warn ".env 存在无法解析的行，已忽略: ${line}"
      continue
    fi
    key="${line%%=*}"
    value="${line#*=}"
    key="${key//[[:space:]]/}"
    if [[ ! "${key}" =~ ^[A-Za-z_][A-Za-z0-9_]*$ ]]; then
      warn ".env 存在非法键名，已忽略: ${key}"
      continue
    fi
    if [[ "${value}" == \"*\" && "${value}" == *\" ]]; then
      value="${value:1:${#value}-2}"
    elif [[ "${value}" == \'*\' && "${value}" == *\' ]]; then
      value="${value:1:${#value}-2}"
    fi
    printf -v "${key}" '%s' "${value}"
    export "${key}"
  done < "${file}"
}

resolve_docker_compose() {
  if docker compose version >/dev/null 2>&1; then
    DOCKER_COMPOSE=(docker compose)
    DOCKER_COMPOSE_LABEL="docker compose"
    pass "Docker Compose 可用 (${DOCKER_COMPOSE_LABEL})"
    return
  fi
  if command -v docker-compose >/dev/null 2>&1 && docker-compose version >/dev/null 2>&1; then
    DOCKER_COMPOSE=(docker-compose)
    DOCKER_COMPOSE_LABEL="docker-compose"
    warn "检测到 legacy Compose 命令 docker-compose。CentOS 7 仅建议用于部署测试，生产麒麟环境应优先使用受控的 Compose 方案。"
    return
  fi
  fail "未找到可用的 Compose 命令。需要 docker compose 或 docker-compose。"
}

check_cmd() {
  local cmd="$1"
  local tip="$2"
  if command -v "${cmd}" >/dev/null 2>&1; then
    pass "已找到命令 ${cmd}"
  else
    fail "未找到命令 ${cmd}。${tip}"
  fi
}

check_status() {
  local url="$1"
  local expected="$2"
  local name="$3"
  local actual
  actual="$(curl -sS -o "${TMP_BODY_FILE}" -w '%{http_code}' --connect-timeout 5 --max-time 10 "${url}" || true)"
  if [[ "${actual}" == "${expected}" ]]; then
    pass "${name} 返回 ${expected}"
  else
    fail "${name} 返回 ${actual}，期望 ${expected}。URL=${url}"
  fi
}

cleanup() {
  if [[ -n "${TMP_BODY_FILE}" && -f "${TMP_BODY_FILE}" ]]; then
    rm -f "${TMP_BODY_FILE}"
  fi
}

check_cmd docker "请先安装 Docker。"
check_cmd bash "请使用 Bash 执行本脚本。"
check_cmd curl "请先安装 curl。"
check_cmd grep "请先安装 grep。"
check_cmd mktemp "请先安装 mktemp，以安全缓存 HTTP 响应。"
resolve_docker_compose

if [[ -f "${ENV_FILE}" ]]; then
  load_env_file "${ENV_FILE}"
fi

TMP_BODY_FILE="$(mktemp "${TMPDIR:-/tmp}/contract-health-check.XXXXXX")"
trap cleanup EXIT

printf '网关地址: %s\n\n' "${GATEWAY_BASE_URL}"
printf 'Compose 文件: %s\n' "${COMPOSE_BASENAME}"

printf '容器状态:\n'
compose_ps_output="$("${DOCKER_COMPOSE[@]}" -f "${COMPOSE_FILE}" ps 2>&1)" || {
  printf '%s\n' "${compose_ps_output}"
  fail "Compose 状态查询失败，请先修复容器编排或 Docker 守护进程异常。"
  exit 1
}
printf '%s\n' "${compose_ps_output}"
printf '\n'

check_status "${GATEWAY_BASE_URL}/health" "200" "网关健康检查"
check_status "${GATEWAY_BASE_URL}/gateway/routes" "200" "网关路由表"
check_status "${GATEWAY_BASE_URL}/api/contracts/contracts" "401" "未登录访问拦截"

if grep -qi "unhealthy" <<<"${compose_ps_output}"; then
  fail "发现 unhealthy 容器，请先修复容器健康检查或运行时依赖。"
fi

if grep -qi "exited" <<<"${compose_ps_output}"; then
  warn "发现 Exited 状态容器，请执行 ${DOCKER_COMPOSE_LABEL} -f ${COMPOSE_BASENAME} ps 查看详情"
fi

if grep -qi "restarting" <<<"${compose_ps_output}"; then
  warn "发现 Restarting 状态容器，请优先查看对应服务日志"
fi

if [[ ${FAILED} -ne 0 ]]; then
  printf '\n健康检查未通过。建议先看:\n'
  printf '1. %s -f %s ps\n' "${DOCKER_COMPOSE_LABEL}" "${COMPOSE_BASENAME}"
  printf '2. %s -f %s logs --tail=200 gateway-service\n' "${DOCKER_COMPOSE_LABEL}" "${COMPOSE_BASENAME}"
  printf '3. %s -f %s logs --tail=200 <异常服务名>\n' "${DOCKER_COMPOSE_LABEL}" "${COMPOSE_BASENAME}"
  exit 1
fi

printf '\n基础健康检查通过。\n'
