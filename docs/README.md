# 文档目录说明

本目录仅保留当前微服务交付版所需文档。

建议阅读顺序：

1. [系统部署手册](E:/HTGL/AnXin_Contract_Manage_microservices_v1.1/docs/系统部署手册.md)
2. [microservice-architecture](E:/HTGL/AnXin_Contract_Manage_microservices_v1.1/docs/microservice-architecture.md)
3. [microservice-local-run](E:/HTGL/AnXin_Contract_Manage_microservices_v1.1/docs/microservice-local-run.md)
4. [运维操作手册](E:/HTGL/AnXin_Contract_Manage_microservices_v1.1/docs/运维操作手册.md)
5. [监控告警配置](E:/HTGL/AnXin_Contract_Manage_microservices_v1.1/docs/监控告警配置.md)
6. [离线部署手册](E:/HTGL/AnXin_Contract_Manage_microservices_v1.1/docs/离线部署手册.md)
7. [支持矩阵与失败信号说明](E:/HTGL/AnXin_Contract_Manage_microservices_v1.1/docs/支持矩阵与失败信号说明.md)

如果你是第一次部署，建议直接按这个顺序：

1. 先看 `系统部署手册.md`
2. 严格按手册准备 `.env`
3. 按手册启动服务
4. 如果启动失败，再看 `运维操作手册.md` 的报错处理部分

新手部署建议直接按这 3 步执行：

1. `bash scripts/check-env.sh`
2. `bash scripts/start-microservices.sh`
3. `bash scripts/health-check.sh`

如果走生产镜像路径，改为：

1. `COMPOSE_FILE=./docker-compose.production.yml bash scripts/check-env.sh`
2. `docker compose --env-file .env -f docker-compose.production.yml build`
3. `docker compose --env-file .env -f docker-compose.production.yml up -d`
4. `COMPOSE_FILE=./docker-compose.production.yml bash scripts/health-check.sh`

当前保留文档说明：

- `系统部署手册.md`：正式部署入口
- `microservice-architecture.md`：微服务架构说明
- `microservice-local-run.md`：本地运行说明
- `运维操作手册.md`：运行维护操作
- `监控告警配置.md`：监控与告警配置
- `离线部署手册.md`：离线镜像与内网交付说明
- `swagger.json` / `swagger.html`：接口查看

说明：

- 已删除旧版、重复、泛化和历史分析文档。
- 如文档内容与微服务实现冲突，以当前微服务代码和部署手册为准。
