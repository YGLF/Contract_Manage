#!/usr/bin/env bash

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"
ENV_FILE="${PROJECT_ROOT}/.env"
ENV_TEMPLATE="${PROJECT_ROOT}/.env.microservices.example"
OFFLINE_BUNDLE_DIR="${1:-${PROJECT_ROOT}/offline-bundle}"
APP_VERSION="${APP_VERSION:-1.1}"
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

check_cmd() {
  local cmd="$1"
  local tip="$2"
  if command -v "${cmd}" >/dev/null 2>&1; then
    pass "已找到命令 ${cmd}"
  else
    fail "未找到命令 ${cmd}。${tip}"
  fi
}

printf '项目目录: %s\n' "${PROJECT_ROOT}"
printf '离线包目录: %s\n\n' "${OFFLINE_BUNDLE_DIR}"

check_cmd docker "请先安装 Docker。"
if docker compose version >/dev/null 2>&1; then
  pass "Docker Compose 可用"
else
  fail "Docker Compose 不可用。请先安装支持 compose 子命令的 Docker。"
fi
check_cmd curl "请先安装 curl。"
check_cmd tar "请先安装 tar。"

mkdir -p "${PROJECT_ROOT}/uploads"
pass "uploads 目录已就绪"

if [[ ! -f "${ENV_FILE}" ]]; then
  if [[ -f "${ENV_TEMPLATE}" ]]; then
    cp "${ENV_TEMPLATE}" "${ENV_FILE}"
    pass "已从模板生成 .env，请继续手工修改数据库、JWT、910B 配置"
  else
    fail "未找到 .env 和 .env.microservices.example"
  fi
else
  pass ".env 已存在"
fi

if [[ -d "${OFFLINE_BUNDLE_DIR}/vendor" ]]; then
  rm -rf "${PROJECT_ROOT}/vendor"
  cp -R "${OFFLINE_BUNDLE_DIR}/vendor" "${PROJECT_ROOT}/vendor"
  pass "已同步 vendor 目录"
elif [[ -d "${PROJECT_ROOT}/vendor" ]]; then
  pass "项目内已有 vendor 目录"
else
  warn "未找到 vendor 目录。离线启动时 Go 依赖可能无法下载"
fi

if [[ -d "${OFFLINE_BUNDLE_DIR}/images" ]]; then
  found_tar=0
  for image_tar in "${OFFLINE_BUNDLE_DIR}"/images/*.tar; do
    if [[ -f "${image_tar}" ]]; then
      found_tar=1
      printf '导入镜像: %s\n' "${image_tar}"
      docker load -i "${image_tar}"
    fi
  done
  if [[ ${found_tar} -eq 1 ]]; then
    pass "离线镜像导入完成"
  else
    warn "离线包 images 目录存在，但未找到 tar 镜像文件"
  fi
else
  warn "未找到离线镜像目录 ${OFFLINE_BUNDLE_DIR}/images"
fi

if docker image inspect "anxin-contract/gateway-service:${APP_VERSION}" >/dev/null 2>&1; then
  pass "生产网关镜像 anxin-contract/gateway-service:${APP_VERSION} 已就绪"
else
  fail "未找到生产网关镜像 anxin-contract/gateway-service:${APP_VERSION}。生产 compose 无法在离线服务器直接启动"
fi

if docker image inspect golang:1.21 >/dev/null 2>&1; then
  pass "golang:1.21 镜像已就绪，可支持开发/联调 compose 路径"
else
  warn "未找到 golang:1.21 镜像。生产镜像路径不依赖该镜像，开发/联调 compose 路径将不可用"
fi

if [[ ${FAILED} -ne 0 ]]; then
  printf '\n运行环境补齐未完成。请先修复 FAIL 项。\n'
  exit 1
fi

printf '\n运行环境补齐完成。\n'
printf '下一步执行:\n'
printf '1. 编辑 %s\n' "${ENV_FILE}"
printf '2. COMPOSE_FILE=./docker-compose.production.yml bash scripts/check-env.sh\n'
printf '3. docker compose --env-file .env -f docker-compose.production.yml up -d\n'
printf '4. COMPOSE_FILE=./docker-compose.production.yml bash scripts/health-check.sh\n'
