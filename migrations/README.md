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
# SQLite (the only supported database)
./bin/migrate -db sqlite3 -db-path ./tennis.db
```

The tool will automatically apply any pending migrations.

#### Rolling Back Migrations

1. Build the migrate down tool:
```bash
go build -o bin/migrate-down cmd/migrate-down/main.go
```

2. Roll back to a specific version:
```bash
# Roll back to version 5
./bin/migrate-down -db-path ./tennis.db -version 5
```

The tool will migrate down to the specified version, running all down migrations in reverse order.

### Command Line Flags

#### Migration Tool (cmd/migrate/main.go)
- `-path`: Path to migration files (default: "./migrations")
- `-db`: Database type (only sqlite3 is supported; anything else is rejected)
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

- `DB_TYPE`: Optional; only sqlite3 is supported (any other value fails)
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
  The next available version is **032**. Do not backfill 026.

### Changing a column of a table that other tables reference

Foreign keys are enforced on every connection (`_foreign_keys=on`) and each
migration file runs inside a transaction, where `PRAGMA foreign_keys = OFF` has
no effect. The usual SQLite recipe "create a new table, copy the rows, drop the
old table, rename" is therefore **destructive** for a table that others point at
with `ON DELETE CASCADE`: `DROP TABLE users` deletes every session, passkey and
captain note along with it.

Change the column in place instead, as migration 034 does for
`users.password_hash` (NOT NULL to nullable):

```sql
ALTER TABLE users ADD COLUMN password_hash_optional TEXT;
UPDATE users SET password_hash_optional = password_hash;
ALTER TABLE users DROP COLUMN password_hash;
ALTER TABLE users RENAME COLUMN password_hash_optional TO password_hash;
```

No row of the table is deleted, so nothing cascades, and rowids, indexes and
the other columns are untouched (the changed column moves to the end of the
table; always name columns in `INSERT`). `DROP COLUMN` refuses a column that
is indexed, part of a key or used in a trigger, view or CHECK. Write a test
that migrates a populated database through the real driver and compares the
referencing tables before and after, like
`internal/database/migration_034_test.go`.

### Edited migrations

golang-migrate records only the version number of each applied migration, not
a checksum of its contents, so editing an already-released migration changes
what new databases get without touching databases that have already run it.
This has been done deliberately once:

- **004 (`create_auth_tables`) no longer inserts any user accounts** (Sprint 020,
  WI-117). It used to create eight named administrator accounts sharing one
  password hash in every new database. A new install now starts with no
  accounts and the first administrator is created through first-run setup
  (`/setup`, protected by a one-time code printed to the server log).
  Databases migrated before the change keep whatever accounts they have; remove
  or re-password any you do not recognise from the admin users page. The down
  migration (dropping the three auth tables) is unchanged and still correct.
  At every start the app logs a warning naming any account that still has a
  password hash seeded by an old version of this migration (it compares
  SHA-256 fingerprints of the old hashes, `internal/secrets/selfcheck.go`; it
  never changes the account). `internal/secrets/isolation_test.go` checks that
  a freshly migrated database has no rows in any table, and that upgrading an
  old database leaves its accounts and sessions unchanged.

## Troubleshooting

- **Dirty Database State**: If a migration fails partway through, the database is marked as "dirty" and the schema may be inconsistent. On startup the app now **fails fast** on a dirty database rather than silently forcing the version (which used to mask schema drift). Inspect the schema, roll back to a clean version with the migrate-down tool, then re-run. In development only, you may set `MIGRATE_ALLOW_DIRTY_FORCE=true` to restore the old auto-force behaviour.

- **Migration Version Mismatch**: If you need to force a specific migration version, you can use the migrate down tool to roll back to a specific version, then run the migration tool again to apply migrations up to the desired version.

- **Rolling Back Migrations**: Use the migrate down tool to roll back to a specific version. This is useful for testing migrations or fixing issues with recent schema changes.

- **SQLite Syntax**: SQLite is the only supported database, so write migrations in SQLite-compatible SQL. 