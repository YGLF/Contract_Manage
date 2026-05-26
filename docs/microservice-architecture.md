# Contract Management Microservice Baseline

This repository now contains a first-stage microservice baseline alongside the legacy monolith.

## Added services

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

## Shared platform packages

- `pkg/microplatform/config`
- `pkg/microplatform/app`
- `pkg/microplatform/httpx`
- `pkg/microplatform/middleware`
- `pkg/microplatform/security`
- `pkg/microplatform/events`

## First-stage capabilities

- service bootstrap and health probes
- JWT issuing and verification
- trace id propagation
- contract registration service skeleton
- document two-step upload skeleton
- performance plan versioning skeleton
- approval workflow skeleton
- risk event lifecycle skeleton
- amendment workflow skeleton
- closure request lifecycle skeleton
- archive/borrow/destroy skeleton
- contract lifecycle persistence and status transition endpoint
- audit client baseline for cross-service audit writes
- outbox foundation for database-backed domain event recording
- notification and report consumer skeletons
- party master data and credit snapshot skeleton
- search-ai orchestration skeleton with business API retrieval and 910B model endpoint placeholder
- outbox dispatcher baseline
- audit sink skeleton
- gateway reverse proxy entrypoints
- optional database mode for first-stage core services through `DB_ENABLED=true`

## Notes

- The legacy monolith remains intact for business continuity.
- The new services provide the implementation baseline for progressive extraction.
- Database isolation is still a shared-database transition strategy; repository-level data isolation is the next step.
