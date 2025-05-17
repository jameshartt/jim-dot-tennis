# Database Migrations

This directory contains database migrations for the Jim.Tennis application.

## Migration Files

Each migration consists of two files:
- `<version>_<name>.up.sql`: SQL statements to apply the migration
- `<version>_<name>.down.sql`: SQL statements to roll back the migration

Migrations are run in numerical order based on the version number prefix.

## Running Migrations

### Using the Migration Tool

We use a custom migration tool based on [golang-migrate/migrate](https://github.com/golang-migrate/migrate).

1. Build the tool:
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

### Command Line Flags

- `-path`: Path to migration files (default: "./migrations")
- `-db`: Database type (postgres or sqlite3)
- `-host`: Database host (PostgreSQL only)
- `-port`: Database port (PostgreSQL only)
- `-user`: Database user (PostgreSQL only)
- `-pass`: Database password (PostgreSQL only) 
- `-name`: Database name (PostgreSQL only)
- `-db-path`: Database file path (SQLite only)

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

## Troubleshooting

- **Dirty Database State**: If a migration fails partway through, the database may be marked as "dirty". The migration tool will automatically attempt to handle this by forcing the version before retrying.

- **Migration Version Mismatch**: If you need to force a specific migration version, you can modify the code in `ExecuteMigrations` to call `m.Migrate(targetVersion)` instead of `m.Up()`.

- **Database-Specific Syntax**: Be aware that PostgreSQL and SQLite have different SQL syntax for some operations. Make sure your migrations are compatible with the database you're using. 