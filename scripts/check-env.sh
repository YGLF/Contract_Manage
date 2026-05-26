#!/usr/bin/env bash

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"
ENV_FILE="${PROJECT_ROOT}/.env"
ENV_TEMPLATE="${PROJECT_ROOT}/.env.microservices.example"
COMPOSE_FILE="${COMPOSE_FILE:-${PROJECT_ROOT}/docker-compose.microservices.yml}"
COMPOSE_BASENAME="$(basename "${COMPOSE_FILE}")"
DOCKER_COMPOSE=()
DOCKER_COMPOSE_LABEL=""
# 生产验证可显式执行:
#   COMPOSE_FILE="${PROJECT_ROOT}/docker-compose.production.yml" bash scripts/check-env.sh

FAILED=0

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

check_cmd() {
  local cmd="$1"
  local tip="$2"
  if command -v "${cmd}" >/dev/null 2>&1; then
    pass "已找到命令 ${cmd}"
  else
    fail "未找到命令 ${cmd}。${tip}"
  fi
}

check_file() {
  local file="$1"
  local name="$2"
  if [[ -f "${file}" ]]; then
    pass "${name} 存在"
  else
    fail "${name} 不存在: ${file}"
  fi
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

check_not_placeholder() {
  local key="$1"
  local value="${2:-}"
  if [[ -z "${value}" ]]; then
    fail "${key} 未配置"
    return
  fi

  case "${value}" in
    replace-*|changeme|change-me|your-*|example*|localhost)
      fail "${key} 仍是默认占位值: ${value}"
      ;;
    *)
      pass "${key} 已配置"
      ;;
  esac
}

check_port_open() {
  local host="$1"
  local port="$2"
  local name="$3"
  if timeout 3 bash -c "</dev/tcp/${host}/${port}" >/dev/null 2>&1; then
    pass "${name} 端口可连接 (${host}:${port})"
  else
    warn "${name} 端口未连通 (${host}:${port})，请确认网络、防火墙或服务状态"
  fi
}

check_http() {
  local url="$1"
  local name="$2"
  if curl -fsS --connect-timeout 5 --max-time 10 "${url}" >/dev/null 2>&1; then
    pass "${name} 可访问 (${url})"
  else
    warn "${name} 暂不可访问 (${url})，请确认地址、DNS 或目标服务状态"
  fi
}

printf '项目目录: %s\n' "${PROJECT_ROOT}"
printf 'Compose 文件: %s\n' "${COMPOSE_BASENAME}"

check_cmd docker "请先安装 Docker。"
check_cmd timeout "请先安装 coreutils timeout，以避免端口探测长时间阻塞。"
check_cmd curl "请先安装 curl。"
check_cmd bash "请使用 Bash 执行本脚本。"
resolve_docker_compose

check_file "${COMPOSE_FILE}" "${COMPOSE_BASENAME}"
if [[ "${COMPOSE_BASENAME}" == "docker-compose.production.yml" ]]; then
  check_file "${PROJECT_ROOT}/Dockerfile.microservice" "Dockerfile.microservice"
fi

if [[ -f "${ENV_FILE}" ]]; then
  pass ".env 存在"
else
  warn ".env 不存在，将以模板为基础检查"
  check_file "${ENV_TEMPLATE}" ".env.microservices.example"
fi

if [[ -f "${ENV_FILE}" ]]; then
  load_env_file "${ENV_FILE}"
fi

check_not_placeholder "JWT_SECRET" "${JWT_SECRET:-}"
check_not_placeholder "DB_HOST" "${DB_HOST:-}"
check_not_placeholder "DB_PORT" "${DB_PORT:-}"
check_not_placeholder "DB_USER" "${DB_USER:-}"
check_not_placeholder "DB_PASSWORD" "${DB_PASSWORD:-}"
check_not_placeholder "DB_NAME" "${DB_NAME:-}"
check_not_placeholder "AI_MODEL_ENDPOINT" "${AI_MODEL_ENDPOINT:-}"

if [[ -n "${UPLOAD_DIR:-}" ]]; then
  pass "UPLOAD_DIR 已配置: ${UPLOAD_DIR}"
else
  warn "UPLOAD_DIR 未配置，将使用服务默认值"
fi

if [[ "${COMPOSE_BASENAME}" == "docker-compose.production.yml" ]]; then
  if [[ -d "${PROJECT_ROOT}/vendor" ]]; then
    pass "vendor 目录存在，可支持离线打包脚本生成依赖快照"
  else
    warn "vendor 目录不存在。生产 compose 本身不依赖 vendor 运行，但离线打包前应执行 scripts/prepare-offline-bundle.sh"
  fi
else
  if [[ -d "${PROJECT_ROOT}/vendor" ]]; then
    pass "vendor 目录存在，可支持离线 Go 依赖构建"
  else
    warn "vendor 目录不存在。当前 compose 使用 go run，离线服务器可能因无法下载 Go 依赖而启动失败"
  fi
fi

if docker image inspect golang:1.21 >/dev/null 2>&1; then
  pass "本机已存在 golang:1.21 镜像"
else
  if [[ "${COMPOSE_BASENAME}" == "docker-compose.production.yml" ]]; then
    warn "本机不存在 golang:1.21 镜像。若当前机器负责执行 production build，需要保证 Docker 能拉取基础镜像或预先导入离线镜像。"
  else
    warn "本机不存在 golang:1.21 镜像。离线服务器必须先执行离线镜像导入"
  fi
fi

if [[ -n "${DB_HOST:-}" && -n "${DB_PORT:-}" ]]; then
  check_port_open "${DB_HOST}" "${DB_PORT}" "数据库"
fi

if [[ -n "${AI_MODEL_ENDPOINT:-}" ]]; then
  check_http "${AI_MODEL_ENDPOINT}" "910B 推理接口"
fi

if [[ ${FAILED} -ne 0 ]]; then
  printf '\n环境检查未通过。请先修复上面的 FAIL 项，再启动微服务。\n'
  exit 1
fi

if [[ "${COMPOSE_BASENAME}" == "docker-compose.production.yml" ]]; then
  printf '\n环境检查已通过。可以继续执行 %s --env-file .env -f docker-compose.production.yml build && %s --env-file .env -f docker-compose.production.yml up -d\n' "${DOCKER_COMPOSE_LABEL}" "${DOCKER_COMPOSE_LABEL}"
else
  printf '\n环境检查已通过。可以继续执行 scripts/start-microservices.sh\n'
fi
