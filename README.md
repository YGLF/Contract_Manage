# 合同管理系统微服务交付目录

本目录为政企合同管理系统的微服务部署交付版，部署目标为：

- 操作系统：麒麟 V10 SP3
- 数据库：南大金仓 GBADE（MySQL 兼容模式）
- AI 推理：910B 算力服务器 HTTP 推理接口

CentOS 7 仅作为应用系统部署测试环境，不作为实际生产基线。

系统边界保持不变：

- 仅管理已盖章生效合同的全生命周期
- 不包含合同起草、签署、电子签、用印流程
- 审批服务仅承接允许存在的子流程审批

## 当前目录用途

本目录用于微服务部署、联调、测试和交付，不再作为旧单体版本的开发目录。

根目录保留的重点内容：

- `cmd/`：各微服务启动入口
- `internal/`：微服务业务实现
- `pkg/`：公共组件、中间件、审计、配置、数据库、Outbox
- `frontend/`：前端工程
- `docs/`：部署与说明文档
- `docker-compose.microservices.yml`：微服务编排文件
- `docker-compose.production.yml`：生产镜像编排文件
- `Dockerfile.microservice`：统一微服务镜像构建入口
- `.env.example`：环境变量模板

## 微服务清单

`cmd/` 下当前包含：

- `gateway-service`
- `identity-service`
- `audit-service`
- `contract-service`
- `document-service`
- `performance-service`
- `approval-workflow-service`
- `risk-service`
- `amendment-service`
- `closure-service`
- `archive-service`
- `notification-service`
- `report-service`
- `party-service`
- `search-ai-service`
- `outbox-dispatcher`

## 部署入口

优先参考以下文件：

- [系统部署手册](E:/HTGL/AnXin_Contract_Manage_microservices_v1.1/docs/系统部署手册.md)
- [微服务编排文件](E:/HTGL/AnXin_Contract_Manage_microservices_v1.1/docker-compose.microservices.yml)
- [生产镜像编排文件](E:/HTGL/AnXin_Contract_Manage_microservices_v1.1/docker-compose.production.yml)

启动示例：

```bash
docker compose --env-file .env -f docker-compose.microservices.yml up -d
```

生产部署示例：

```bash
COMPOSE_FILE=./docker-compose.production.yml bash scripts/check-env.sh
docker compose --env-file .env -f docker-compose.production.yml build
docker compose --env-file .env -f docker-compose.production.yml up -d
COMPOSE_FILE=./docker-compose.production.yml bash scripts/health-check.sh
```

## 目录整理说明

为避免旧单体代码和微服务交付目录混用，历史单体源码、旧运行产物、旧日志和旧可执行文件已归档到：

- `_legacy_monolith_archive/`

该归档目录仅用于历史追溯，不作为当前微服务交付入口。

## 建议后续动作

部署前建议继续完成以下检查：

1. 按麒麟 V10 SP3 / GBADE / 910B 环境修正 `.env`
2. 生产环境优先校验 `docker-compose.production.yml`，联调环境再校验 `docker-compose.microservices.yml`
3. 清理或替换默认密钥与测试账号配置
4. 依据部署手册执行健康检查和验收测试
