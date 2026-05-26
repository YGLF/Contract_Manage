#!/usr/bin/env bash

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"
ENV_FILE="${PROJECT_ROOT}/.env"
COMPOSE_FILE="${PROJECT_ROOT}/docker-compose.microservices.yml"
CHECK_SCRIPT="${SCRIPT_DIR}/check-env.sh"
HEALTH_SCRIPT="${SCRIPT_DIR}/health-check.sh"

cd "${PROJECT_ROOT}"

if [[ ! -f "${ENV_FILE}" ]]; then
  printf '[FAIL] 未找到 %s\n' "${ENV_FILE}"
  printf '请先执行: cp .env.microservices.example .env\n'
  exit 1
fi

if [[ -x "${CHECK_SCRIPT}" ]]; then
  "${CHECK_SCRIPT}"
else
  bash "${CHECK_SCRIPT}"
fi

printf '\n启动微服务...\n'
docker compose --env-file "${ENV_FILE}" -f "${COMPOSE_FILE}" up -d

printf '\n当前容器状态:\n'
docker compose -f "${COMPOSE_FILE}" ps

printf '\n等待 gateway-service 健康检查通过...\n'
GATEWAY_PORT="$(grep -E '^GATEWAY_SERVICE_PORT=' "${ENV_FILE}" | tail -n 1 | cut -d '=' -f 2)"
GATEWAY_PORT="${GATEWAY_PORT:-8100}"
GATEWAY_HEALTH_URL="http://127.0.0.1:${GATEWAY_PORT}/health"

for _ in $(seq 1 20); do
  if curl -fsS --connect-timeout 3 --max-time 5 "${GATEWAY_HEALTH_URL}" >/dev/null 2>&1; then
    printf '[PASS] gateway-service 已就绪: %s\n' "${GATEWAY_HEALTH_URL}"
    break
  fi
  sleep 2
done

if ! curl -fsS --connect-timeout 3 --max-time 5 "${GATEWAY_HEALTH_URL}" >/dev/null 2>&1; then
  printf '[WARN] gateway-service 健康检查未通过，请查看日志:\n'
  printf 'docker compose -f docker-compose.microservices.yml logs --tail=200 gateway-service\n'
  exit 1
fi

printf '\n执行基础健康检查...\n'
if [[ -x "${HEALTH_SCRIPT}" ]]; then
  "${HEALTH_SCRIPT}"
else
  bash "${HEALTH_SCRIPT}"
fi
