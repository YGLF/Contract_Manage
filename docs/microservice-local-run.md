# Microservice Local Run

## Services

- gateway-service: `8100`
- identity-service: `8101`
- audit-service: `8102`
- contract-service: `8103`
- document-service: `8104`
- performance-service: `8105`
- approval-workflow-service: `8106`
- risk-service: `8107`
- amendment-service: `8108`
- closure-service: `8109`
- archive-service: `8110`
- notification-service: `8111`
- report-service: `8112`
- party-service: `8113`
- search-ai-service: `8114`

## Start with Docker Compose

```bash
docker compose --env-file .env -f docker-compose.microservices.yml up
```

This path is for local development and integration only. Production deployment should use `docker-compose.production.yml`, immutable images, and the validation flow documented in `系统部署手册.md`.

## Manual start

```bash
go run ./cmd/identity-service
go run ./cmd/audit-service
set DOCUMENT_SERVICE_URL=http://localhost:8104
set PARTY_SERVICE_URL=http://localhost:8113
go run ./cmd/contract-service
go run ./cmd/document-service
go run ./cmd/performance-service
set CONTRACT_SERVICE_URL=http://localhost:8103
set PERFORMANCE_SERVICE_URL=http://localhost:8105
set ARCHIVE_SERVICE_URL=http://localhost:8110
go run ./cmd/approval-workflow-service
set NOTIFICATION_SERVICE_URL=http://localhost:8111
set RISK_NOTIFICATION_RECIPIENT=u-admin
go run ./cmd/risk-service
go run ./cmd/amendment-service
go run ./cmd/closure-service
go run ./cmd/archive-service
set AUTO_SEND_RISK_ALERTS=true
go run ./cmd/notification-service
set APPROVAL_SERVICE_URL=http://localhost:8106
set RISK_SERVICE_URL=http://localhost:8107
set ARCHIVE_SERVICE_URL=http://localhost:8110
set CONTRACT_SERVICE_URL=http://localhost:8103
go run ./cmd/report-service
go run ./cmd/party-service
set AUDIT_SERVICE_URL=http://localhost:8102
set REPORT_SERVICE_URL=http://localhost:8112
set AI_MODEL_ENDPOINT=http://127.0.0.1:9000/infer
go run ./cmd/search-ai-service
go run ./cmd/gateway-service
set DB_ENABLED=true
set NOTIFICATION_SERVICE_URL=http://localhost:8111
set REPORT_SERVICE_URL=http://localhost:8112
go run ./cmd/outbox-dispatcher
```

## Optional database mode

The following services now support a database-backed mode:

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

Enable it with:

```bash
set DB_ENABLED=true
set DB_DRIVER=mysql
set DB_HOST=127.0.0.1
set DB_PORT=3306
set DB_USER=root
set DB_PASSWORD=your_password
set DB_NAME=contract_manage
```

For the target国产数据库 environment, keep using the same abstraction layer and switch to the compatible connection settings that match the Kingbase MySQL-compatible endpoint.

## Example flow

Recommended contract intake flow:

1. Upload temp document
2. Query temp document state
3. Commit temp document
4. Create contract through `/api/v1/contracts/intake`
5. Verify document state changes to `bound`
6. Let contract-service sync cooperation history to `party-service`
7. Continue later lifecycle operations

1. Login through identity service:

```bash
curl -X POST http://localhost:8101/api/v1/auth/login ^
  -H "Content-Type: application/json" ^
  -d "{\"username\":\"admin\",\"password\":\"<use-configured-admin-password>\"}"
```

2. Create a contract directly against contract service:

```bash
curl -X POST http://localhost:8103/api/v1/contracts ^
  -H "Content-Type: application/json" ^
  -d "{\"title\":\"示例合同\",\"counterparty_id\":\"party-001\",\"document_ids\":[\"doc-temp-0001\"]}"
```

3. Upload a temp document:

```bash
curl -X POST http://localhost:8104/api/v1/documents/temp -F "file=@example.pdf"
```

3a. Query temp document state:

```bash
curl http://localhost:8104/api/v1/documents/temp/doc-temp-0001
```

3b. Commit the temp document:

```bash
curl -X POST http://localhost:8104/api/v1/documents/commit ^
  -H "Content-Type: application/json" ^
  -d "{\"temp_document_id\":\"doc-temp-0001\"}"
```

3c. Create contract through intake endpoint:

```bash
curl -X POST http://localhost:8103/api/v1/contracts/intake ^
  -H "Content-Type: application/json" ^
  -d "{\"title\":\"绀轰緥鍚堝悓\",\"counterparty_id\":\"party-001\",\"document_ids\":[\"doc-temp-0001\"]}"
```

The intake endpoint now calls `document-service` for strict validation. Every referenced temp document must already be `committed`; once the contract is created successfully, those same documents are switched to `bound`.

3d. Verify the temp document state after intake:

```bash
curl http://localhost:8104/api/v1/documents/temp/doc-temp-0001
```

4. Create an approval request:

```bash
curl -X POST http://localhost:8106/api/v1/approval-requests ^
  -H "Content-Type: application/json" ^
  -d "{\"contract_id\":\"ctr-0001\",\"request_type\":\"status_change\",\"requested_by\":\"u-admin\"}"
```

Supported approval request types are now constrained to:

- `status_change`
- `plan_adjustment`
- `archive_borrow`
- `archive_destroy`

4a. Example status change approval payload:

```bash
curl -X POST http://localhost:8106/api/v1/approval-requests ^
  -H "Content-Type: application/json" ^
  -d "{\"contract_id\":\"ctr-0001\",\"request_type\":\"status_change\",\"requested_by\":\"u-admin\",\"payload\":{\"status\":\"closed\"}}"
```

4b. Example plan adjustment approval payload:

```bash
curl -X POST http://localhost:8106/api/v1/approval-requests ^
  -H "Content-Type: application/json" ^
  -d "{\"contract_id\":\"ctr-0001\",\"request_type\":\"plan_adjustment\",\"requested_by\":\"u-admin\",\"payload\":{\"nodes\":[{\"node_name\":\"楠屾敹\",\"node_type\":\"acceptance\",\"due_date\":\"2026-06-30T00:00:00Z\"}]}}"
```

4c. Approve the request and trigger downstream business callback:

```bash
curl -X POST http://localhost:8106/api/v1/approval-requests/apr-0001/approve ^
  -H "Content-Type: application/json" ^
  -d "{\"approved_by\":\"u-admin\"}"
```

5. Create a risk event:

```bash
curl -X POST http://localhost:8107/api/v1/risk/events ^
  -H "Content-Type: application/json" ^
  -d "{\"contract_id\":\"ctr-0001\",\"rule_code\":\"expiry_warning\",\"severity\":\"high\",\"description\":\"合同即将到期\"}"
```

When `NOTIFICATION_SERVICE_URL` and `RISK_NOTIFICATION_RECIPIENT` are configured, creating or closing a risk event will also create an in-app notification. If `AUTO_SEND_RISK_ALERTS=true` is set on notification-service, the generated risk notifications will be marked as sent immediately.

5a. Inspect generated notifications:

```bash
curl http://localhost:8111/api/v1/notifications/messages
```

5b. Close a risk event and trigger a closure notification:

```bash
curl -X POST http://localhost:8107/api/v1/risk/events/risk-0001/close
```

6. Create an amendment:

```bash
curl -X POST http://localhost:8108/api/v1/amendments ^
  -H "Content-Type: application/json" ^
  -d "{\"contract_id\":\"ctr-0001\",\"title\":\"补充协议一\",\"reason\":\"履约条款调整\",\"supplement_doc_id\":\"doc-temp-0001\"}"
```

6a. Approve the amendment and let amendment-service push the latest amendment view back to contract-service:

```bash
curl -X POST http://localhost:8108/api/v1/amendments/amd-0001/approve ^
  -H "Content-Type: application/json" ^
  -d "{\"approved_by\":\"u-admin\"}"
```

6b. Query the contract and verify `latest_amendment_id` and `latest_amendment_title` are updated:

```bash
curl http://localhost:8103/api/v1/contracts/ctr-0001
```

7. Create a closure request:

```bash
curl -X POST http://localhost:8109/api/v1/closure/requests ^
  -H "Content-Type: application/json" ^
  -d "{\"contract_id\":\"ctr-0001\",\"request_type\":\"close\",\"reason\":\"履约完成申请结案\",\"requested_by\":\"u-admin\",\"risk_checked\":true,\"performance_ok\":true,\"evidence_ready\":true}"
```

8. Create an archive case:

```bash
curl -X POST http://localhost:8110/api/v1/archive/cases ^
  -H "Content-Type: application/json" ^
  -d "{\"contract_id\":\"ctr-0001\",\"archive_type\":\"electronic\"}"
```

9. Update contract status and inspect lifecycle:

```bash
curl -X POST http://localhost:8103/api/v1/contracts/ctr-0001/status ^
  -H "Content-Type: application/json" ^
  -d "{\"status\":\"closed\",\"operator_id\":\"u-admin\",\"description\":\"结案完成\"}"
```

```bash
curl http://localhost:8103/api/v1/contracts/ctr-0001/lifecycle
```

10. Dispatch pending outbox events:

```bash
set DB_ENABLED=true
set NOTIFICATION_SERVICE_URL=http://localhost:8111
set REPORT_SERVICE_URL=http://localhost:8112
go run ./cmd/outbox-dispatcher
```

11. Query dashboard and workbench aggregates:

```bash
curl http://localhost:8112/api/v1/reports/dashboard
```

```bash
curl http://localhost:8112/api/v1/reports/workbench
```

The frontend dashboard page now prefers the microservice aggregation endpoints above instead of the legacy monolith statistics endpoints, so keeping `report-service`, `risk-service`, `approval-workflow-service`, `archive-service`, and `contract-service` available is important during demos.

12. Manage counterparties and credit snapshots:

```bash
curl -X POST http://localhost:8113/api/v1/parties ^
  -H "Content-Type: application/json" ^
  -d "{\"name\":\"绀轰緥渚涘簲鍟嗭紝\",\"unified_social_code\":\"91310000TEST0001X\",\"contact_name\":\"寮犱笁\",\"contact_phone\":\"13800000000\",\"credit_rating\":\"A\",\"credit_source\":\"manual\",\"status\":\"active\"}"
```

```bash
curl -X POST http://localhost:8113/api/v1/parties/party-0001/credit-snapshots ^
  -H "Content-Type: application/json" ^
  -d "{\"rating\":\"AA\",\"source\":\"government_credit\",\"risk_flag\":\"low\",\"description\":\"淇＄敤鐘舵€佽壇濂?\"}"
```

```bash
curl http://localhost:8113/api/v1/parties/party-0001/cooperation-summary
```

If `PARTY_SERVICE_URL` is configured on contract-service, creating a contract now validates that the referenced counterparty exists and is `active`, then writes a cooperation history record back to `party-service`.

The frontend customer-management tab now reads and writes counterparty master data through `party-service`. The contract-type tab still uses the legacy endpoints for now, so both paths may coexist during the transition phase.

13. Ask the controlled AI assistant:

```bash
curl -X POST http://localhost:8114/api/v1/search-ai/ask ^
  -H "Content-Type: application/json" ^
  -d "{\"question\":\"请汇总当前风险情况\",\"user_id\":\"u-admin\"}"
```

The search-ai service only reads data through business APIs, writes an audit trail, and optionally calls the configured 910B inference endpoint through `AI_MODEL_ENDPOINT`.

## Current scope

- This is a first-stage executable scaffold for progressive service extraction.
- The first-stage core services can now run in memory or database mode.
- The legacy monolith remains the current feature-complete implementation reference.
- Next implementation steps should replace in-memory state with service-owned repositories and introduce shared database boundary rules.
