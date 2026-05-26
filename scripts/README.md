# Operational Scripts

These one-off database and verification tools are excluded from normal builds and tests.

Run them only with the explicit build tag:

```bash
go run -tags operational_scripts ./scripts/<name>.go ./scripts/operational_runtime.go
```

Each script requires `CONTRACT_MANAGE_DSN` and will exit if it is not set.
