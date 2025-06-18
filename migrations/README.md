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
- `-version`: Target migration version to migrate down to (default: 5)

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

- **Migration Version Mismatch**: If you need to force a specific migration version, you can use the migrate down tool to roll back to a specific version, then run the migration tool again to apply migrations up to the desired version.

- **Rolling Back Migrations**: Use the migrate down tool to roll back to a specific version. This is useful for testing migrations or fixing issues with recent schema changes.

- **Database-Specific Syntax**: Be aware that PostgreSQL and SQLite have different SQL syntax for some operations. Make sure your migrations are compatible with the database you're using. 