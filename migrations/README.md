# Database Migrations Baseline

## Scope

This directory is the first managed database migration baseline for the current schema set in `models/`.

- `0001_baseline.sql`: current schema baseline for existing tables
- Future files: append-only, ordered, never rewrite a released migration

## Environment policy

- `APP_ENV` unset, `development`, `local`, `test`: application startup may use guarded `AutoMigrate`
- `APP_ENV=production`, `staging`, `preprod`: application startup must not mutate schema by default
- `DB_MIGRATION_MODE=force`: explicit emergency override for application-side `AutoMigrate`; do not use as a normal release path
- `DB_MIGRATION_MODE=manual` or `baseline-only`: verify schema only, no startup DDL

## Apply order

1. Provision database/account with `init.sql`
2. Apply `migrations/0001_baseline.sql`
3. Start the service with `APP_ENV=production` and `DB_MIGRATION_MODE=manual`
4. Apply future migration files in lexical order before deploying code that depends on them

## Change governance

- One migration file must correspond to one auditable change set
- Production migrations must be reviewed alongside code, with rollback notes
- Avoid mixing schema changes, seed data changes, and business hotfixes in one file unless they are operationally inseparable
- If a migration changes state semantics, permissions, approval flow, or reporting definitions, document impact and rollback constraints in the file header

## Reserved conventions for next rounds

### Outbox

Reserve a dedicated append-only outbox table family, not ad hoc business tables. Required fields should include:

- stable event ID
- aggregate type and aggregate ID
- event type and schema version
- payload, headers, retry metadata
- available/published timestamps
- producer identity and trace/audit correlation identifiers

### Monetary fields

Current baseline keeps legacy `DOUBLE` columns for compatibility. Follow-up rectification should move monetary amounts to `DECIMAL`, with:

- explicit scale and currency rules
- backfill and reconciliation SQL
- dual-read or compatibility window if code rollout is staggered
- verification against reports and downstream exports

### Status enumerations

Current baseline keeps legacy string status columns for compatibility. Follow-up rectification should:

- define canonical allowed values centrally
- include data cleansing for historical invalid values
- avoid silent semantic drift between application constants and database contents
- assess whether `CHECK` constraints, reference tables, or managed enum strategy is the safest fit for MySQL 8 deployment practice

## Audit gap still present

This repo now blocks default production `AutoMigrate`, but it still lacks a full migration runner and signed execution trail. Until that is added, production release records must capture:

- operator identity
- execution timestamp
- exact SQL file version
- target database instance
- verification result and rollback outcome
