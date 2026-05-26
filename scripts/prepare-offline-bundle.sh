#!/usr/bin/env bash

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"
BUNDLE_DIR="${PROJECT_ROOT}/offline-bundle"
IMAGES_DIR="${BUNDLE_DIR}/images"
VENDOR_DIR="${BUNDLE_DIR}/vendor"
APP_VERSION="${APP_VERSION:-1.1}"
PACKAGE_NAME="AnXin_Contract_Manage_microservices_offline_package.tar.gz"
PACKAGE_PATH="${PROJECT_ROOT}/${PACKAGE_NAME}"
created_build_env=0

cleanup() {
  if [[ ${created_build_env} -eq 1 ]]; then
    rm -f "${PROJECT_ROOT}/.env"
  fi
}
trap cleanup EXIT

pass() {
  printf '[PASS] %s\n' "$1"
}

fail() {
  printf '[FAIL] %s\n' "$1"
  exit 1
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

check_cmd docker "请在联网机器上安装 Docker。"
check_cmd go "请在联网机器上安装 Go 1.21。"
check_cmd tar "请先安装 tar。"

rm -rf "${BUNDLE_DIR}"
mkdir -p "${IMAGES_DIR}"

printf '准备 vendor 目录...\n'
cd "${PROJECT_ROOT}"
go mod vendor
rm -rf "${VENDOR_DIR}"
cp -R "${PROJECT_ROOT}/vendor" "${VENDOR_DIR}"
pass "vendor 目录已准备完成"

printf '拉取并导出基础镜像...\n'
docker pull golang:1.21
docker save -o "${IMAGES_DIR}/golang_1.21.tar" golang:1.21
pass "基础镜像已导出"

printf '构建并导出生产业务镜像...\n'
export APP_VERSION
if [[ ! -f "${PROJECT_ROOT}/.env" ]]; then
  cp "${PROJECT_ROOT}/.env.microservices.example" "${PROJECT_ROOT}/.env"
  created_build_env=1
fi
docker compose --env-file .env.microservices.example -f docker-compose.production.yml build

service_images=()
while IFS= read -r -d '' service_dir; do
  service_name="$(basename "${service_dir}")"
  service_images+=("anxin-contract/${service_name}:${APP_VERSION}")
done < <(find "${PROJECT_ROOT}/cmd" -mindepth 1 -maxdepth 1 -type d -print0 | sort -z)

if [[ ${#service_images[@]} -eq 0 ]]; then
  fail "未找到 cmd/* 微服务目录，无法导出生产业务镜像"
fi

docker save -o "${IMAGES_DIR}/anxin_contract_services_${APP_VERSION}.tar" "${service_images[@]}"
pass "生产业务镜像已导出"

printf '生成离线说明...\n'
cat > "${BUNDLE_DIR}/README.txt" <<'EOF'
1. 将整个 offline-bundle 目录复制到内网服务器项目根目录。
2. 在服务器项目根目录执行:
   bash scripts/bootstrap-runtime.sh ./offline-bundle
3. 修改 .env 后先执行:
   bash scripts/check-env.sh
4. 生产镜像路径启动:
   docker compose --env-file .env -f docker-compose.production.yml up -d
   bash scripts/health-check.sh
5. 开发/联调路径才使用:
   bash scripts/start-microservices.sh
EOF
pass "离线说明已生成"

printf '打包离线包...\n'
rm -f "${PACKAGE_PATH}"
tar --exclude='.git' \
    --exclude='.gocache' \
    --exclude='_legacy_monolith_archive' \
    --exclude='uploads' \
    -czf "${PACKAGE_PATH}" \
    -C "${PROJECT_ROOT}" \
    cmd \
    config \
    crypto \
    docs \
    frontend \
    internal \
    migrations \
    offline-bundle \
    pkg \
    scripts \
    .env.example \
    .env.microservices.example \
    Dockerfile.microservice \
    docker-compose.microservices.yml \
    docker-compose.production.yml \
    go.mod \
    go.sum \
    init.sql \
    README.md

pass "完整离线交付包已生成"

printf '\n离线准备完成。\n'
printf '目录: %s\n' "${BUNDLE_DIR}"
printf '压缩包: %s\n' "${PACKAGE_PATH}"
printf '可选两种交付方式:\n'
printf '1. 直接复制整个项目目录\n'
printf '2. 复制压缩包 %s 到内网服务器后解压\n' "${PACKAGE_NAME}"
