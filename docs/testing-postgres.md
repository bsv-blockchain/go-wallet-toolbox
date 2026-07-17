## Testing with Postgres

This document explains how to run the wallet storage tests against Postgres locally and describes the test isolation model.

### Local development setup

The repository includes a `docker-compose.yaml` with a Postgres service (`db`). To run tests locally:

1. **Start the Postgres service:**
   ```bash
   docker compose up -d db
   ```

2. **Run storage-related tests in Postgres mode:**
   ```bash
   TEST_DB_MODE=postgres go test -p 1 ./pkg/storage/...
   ```

   The `-p 1` flag runs tests serially (Postgres test isolation does not support parallel execution).

3. **Run all storage, internal storage, and testability tests:**
   ```bash
   TEST_DB_MODE=postgres go test -p 1 ./pkg/internal/storage/... ./pkg/storage/... ./pkg/internal/testabilities/...
   ```

### Configuration

#### Database name override

By default, tests use the Postgres database specified in your connection string. To override the database name:

```bash
TEST_DB_NAME=my_test_db TEST_DB_MODE=postgres go test -p 1 ./pkg/storage/...
```

This is useful when you want to isolate tests to a specific database instance.

#### Connection parameters

Tests expect Postgres to be running on `localhost:5432` with:
- **User:** `postgres`
- **Password:** `postgres`
- **Database:** `postgres` (default, can be overridden with `TEST_DB_NAME`)

These match the defaults in `docker-compose.yaml` and the CI workflow.

### Test isolation

#### Schema-per-test isolation

Each test creates its own isolated Postgres schema for the duration of the test. This approach:

- **Prevents cross-test pollution:** Tests do not see each other's data.
- **Enables parallel-safe testing:** Each schema is unique and independent (though tests currently run serially with `-p 1` to avoid connection pool contention).
- **Automatic cleanup:** When a test completes, its schema is dropped and all connections to that schema are closed.

This isolation strategy is implemented in the fixture code (`testmode.PostgresMode`) and requires no explicit teardown in individual tests — the cleanup happens automatically during test shutdown.

### Postgres-only tests

Tests that require Postgres features or cannot run against SQLite should skip gracefully when `TEST_DB_MODE` is unset or set to a non-Postgres value:

```go
func TestSomePostgresFeature(t *testing.T) {
    if os.Getenv("TEST_DB_MODE") != "postgres" {
        t.Skip("postgres-only test; set TEST_DB_MODE=postgres to run")
    }
    // Test implementation here
}
```

This ensures:
- Tests pass on CI/dev environments without Postgres.
- Developers can opt into Postgres tests when needed.
- The test suite remains portable across databases.

### CI workflow

Postgres tests run automatically in the repo-local CI workflow: `.github/workflows/postgres-tests.yml`

This workflow is intentionally **not** part of the synced fortress template suite. Fortress workflows (`fortress-*.yml`) are overwritten by [Sync] pull requests from the template repository, so Postgres CI lives in its own separate workflow file to survive template syncs.

The workflow:
- Runs on all pushes to `main`/`master` and on all pull requests.
- Starts a Postgres 17-Alpine service with health checks.
- Runs the storage test suites with `TEST_DB_MODE=postgres`.
- Uses a 30-minute timeout and serial execution (`-p 1`) to ensure reliable isolation.

For details, see `.github/workflows/postgres-tests.yml`.
