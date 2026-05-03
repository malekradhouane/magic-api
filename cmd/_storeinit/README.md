# storeinit

`storeinit` is a tool to initialize magic storages.

## Configuration

Configuration is loaded from `config.yaml` in current directory.
You can change the path to configuration file by setting the
environment variable `STOREINIT_CONFIG`.

Example:

```
storages:
  - name: pg-magic
    enabled: true
    driver: pg
    dbURI: "postgresql://admin:magic@localhost:5435/magicdb?sslmode=disable"
    schemaFolder: "file://testdata/migrations/pg/magic"
    forceSchemaVersion: 1
```

Corresponding schemas can be found in `testdata/migrations` folder.

## Supported drivers

- `mongodb` for Mongo DB
- `pg` for PostgreSQL

## Schemas migration

storeinit reads migrations from sources and applies them in the correct order to a database.
Since storeinit uses go-migrate as a backend, you can find more information at https://github.com/golang-migrate/migrate?tab=readme-ov-file#readme

## Usage

```
> storeinit --help

Usage of storeinit:
  -down
        Remove all schemas
  -force
        Force migration to configured version and clear dirty flag
  -lock-timeout float
        lock timeout in seconds (default: 15.00s) (default 15)
  -status
        Report migration status
  -up
        Apply all schemas
  -v    show version and exit
```

### With empty storage:

Create all schemas:

```
> storeinit --up

+---+--------------+------------+---------+-------+
| # | REPO         | DRIVER     | UPDATED | ERROR |
+---+--------------+------------+---------+-------+
| 0 | mongo-users  | mongodb    | Yes     |       |
| 1 | mongo-magic | mongodb    | Yes     |       |
| 2 | pg-magic    | PostgreSQL | Yes     |       |
+---+--------------+------------+---------+-------+
```

Check migration status:

```
> storeinit --status

+---+--------------+-------+---------+-------+
| # | REPO         | DIRTY | VERSION | ERROR |
+---+--------------+-------+---------+-------+
| 0 | mongo-users  |       |       1 |       |
| 1 | mongo-magic |       |      24 |       |
| 2 | pg-magic    |       |       1 |       |
+---+--------------+-------+---------+-------+
```

### With dirty storage

```
> storeinit --status

+---+--------------+------------+-------+---------+-------+
| # | REPO         | DRIVER     | DIRTY | VERSION | ERROR |
+---+--------------+------------+-------+---------+-------+
| 0 | mongo-users  | mongodb    |       |       1 |       |
| 1 | mongo-magic | mongodb    |   X   |       2 |       |
| 2 | pg-magic    | PostgreSQL |       |       1 |       |
+---+--------------+------------+-------+---------+-------+
```

### With non empty storage

**use with caution**

Force schema to configured version:

```
> storeinit --force
```

Remove all schemas:

```
> storeinit --down
```
