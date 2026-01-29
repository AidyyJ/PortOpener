# Migrations

SQLite migrations live in this directory. Apply them in order using your preferred tool.

For local development, the simplest path is:

```bash
sqlite3 ./data/portopener.db < ./migrations/0001_initial.sql
```

Database wiring is introduced in Phase 1+; Phase 0 provides schema scaffolding only.

