# Database Migrations

This directory contains database migrations for the Jim.Tennis application.

## Migration Files

Each migration consists of two files:
- `<version>_<name>.up.sql`: SQL statements to apply the migration
- `<version>_<name>.down.sql`: SQL statements to roll back the migration

Migrations are run in numerical order based on the version number prefix.

## Running Migrations

### Using the Migration Tools

We use custom migration tools based on [golang-migrate/migrate](https://github.com/golang-migrate/migrate).

#### Applying Migrations

1. Build the migration tool:
```bash
go build -o bin/migrate cmd/migrate/main.go
```

2. Run the migrations:
```bash
# For SQLite (default)
./bin/migrate -db sqlite3 -db-path ./tennis.db

# For PostgreSQL
./bin/migrate -db postgres -host localhost -port 5432 -user postgres -pass postgres -name tennis
```

The tool will automatically apply any pending migrations.

#### Rolling Back Migrations

1. Build the migrate down tool:
```bash
go build -o bin/migrate-down cmd/migrate-down/main.go
```

2. Roll back to a specific version:
```bash
# Roll back to version 5 (SQLite)
./bin/migrate-down -db-path ./tennis.db -version 5

# Roll back to version 3 (PostgreSQL)
./bin/migrate-down -db postgres -host localhost -port 5432 -user postgres -pass postgres -name tennis -version 3
```

The tool will migrate down to the specified version, running all down migrations in reverse order.

### Command Line Flags

#### Migration Tool (cmd/migrate/main.go)
- `-path`: Path to migration files (default: "./migrations")
- `-db`: Database type (postgres or sqlite3)
- `-host`: Database host (PostgreSQL only)
- `-port`: Database port (PostgreSQL only)
- `-user`: Database user (PostgreSQL only)
- `-pass`: Database password (PostgreSQL only) 
- `-name`: Database name (PostgreSQL only)
- `-db-path`: Database file path (SQLite only)

#### Migrate Down Tool (cmd/migrate-down/main.go)
- `-path`: Path to migration files (default: "./migrations")
- `-db-path`: Database file path (default: "./tennis.db")
- `-version`: Target migration version to migrate down to (**required** — no default)
- `-yes`: Skip the interactive confirmation prompt (for non-interactive use)

> **Safety:** `-version` is required and rolling back prompts for confirmation.
> There is deliberately no default target — a no-arg run must never silently
> destroy schema.

### Environment Variables

The migration tool also supports configuration via environment variables:

- `DB_TYPE`: Database type (postgres or sqlite3)
- `DB_HOST`: Database host (PostgreSQL only)
- `DB_PORT`: Database port (PostgreSQL only)
- `DB_USER`: Database user (PostgreSQL only)
- `DB_PASSWORD`: Database password (PostgreSQL only)
- `DB_NAME`: Database name (PostgreSQL only)
- `DB_PATH`: Database file path (SQLite only)

## Creating New Migrations

To create a new migration:

1. Create two new files in the migrations directory with the following naming convention:
   - `<next_version>_<descriptive_name>.up.sql`
   - `<next_version>_<descriptive_name>.down.sql`

2. Write the SQL statements to apply the migration in the `.up.sql` file.

3. Write the SQL statements to roll back the migration in the `.down.sql` file.

4. Run the migration tool to apply the new migration.

### Version numbering notes

- **Every migration must have both an `.up.sql` and a `.down.sql` file.** A missing
  down file makes any rollback through that version fail, which strands all
  earlier versions as unreachable. (Migration 012 was missing its down file for a
  long time — now fixed.)
- **Never reuse version 026.** It was assigned during Sprint 017 (`lineup_drafts`)
  but never committed, leaving a permanent gap in the sequence (…025, 027, 028…).
  The next available version is **029**. Do not backfill 026.

## Troubleshooting

- **Dirty Database State**: If a migration fails partway through, the database is marked as "dirty" and the schema may be inconsistent. On startup the app now **fails fast** on a dirty database rather than silently forcing the version (which used to mask schema drift). Inspect the schema, roll back to a clean version with the migrate-down tool, then re-run. In development only, you may set `MIGRATE_ALLOW_DIRTY_FORCE=true` to restore the old auto-force behaviour.

- **Migration Version Mismatch**: If you need to force a specific migration version, you can use the migrate down tool to roll back to a specific version, then run the migration tool again to apply migrations up to the desired version.

- **Rolling Back Migrations**: Use the migrate down tool to roll back to a specific version. This is useful for testing migrations or fixing issues with recent schema changes.

- **Database-Specific Syntax**: Be aware that PostgreSQL and SQLite have different SQL syntax for some operations. Make sure your migrations are compatible with the database you're using. 