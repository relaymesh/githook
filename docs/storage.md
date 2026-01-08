# Installation Storage

Githooks can persist SCM installation data in a SQL database via GORM.
Two tables are used:
- `githooks_installations` for install/token metadata
- `git_namespaces` for repositories (owner/name metadata per provider)
This is intended for multi‑org setups where you need to track tokens and install
metadata per account.

## Multi-tenant scoping

Storage is tenant-aware. If the request context contains a tenant ID, all
installation, namespace, and rule operations are scoped to that tenant. The
server populates this automatically from the `tenant_id` claim in the auth token.

If no tenant ID is present, storage behaves as single-tenant.

## Configuration

```yaml
storage:
  driver: postgres
  dsn: postgres://githooks:githooks@localhost:5432/githooks?sslmode=disable
  dialect: postgres
  table: githooks_installations
  auto_migrate: true
```

Supported dialects:
- `postgres`
- `mysql`
- `sqlite` (pure-Go via `github.com/glebarez/sqlite`)

## Data Model

Each installation record includes:
- Provider
- Account ID / name
- Installation ID
- Access / refresh tokens (if applicable)
- Expiration time
- Metadata JSON

GitHub App tokens are short‑lived and should not be stored. GitLab/Bitbucket
tokens can be stored when you control their lifecycle.

## Example Usage

```go
store, err := installations.Open(installations.Config{
  Driver:      "postgres",
  DSN:         os.Getenv("DB_DSN"),
  Dialect:     "postgres",
  AutoMigrate: true,
})
if err != nil {
  log.Fatal(err)
}
defer store.Close()

err = store.UpsertInstallation(ctx, storage.InstallRecord{
  Provider:       "github",
  AccountID:      "12345",
  AccountName:    "dummy-org",
  InstallationID: "999",
})

nsStore, err := namespaces.Open(namespaces.Config{
  Driver:      "postgres",
  DSN:         os.Getenv("DB_DSN"),
  Dialect:     "postgres",
  AutoMigrate: true,
})
if err != nil {
  log.Fatal(err)
}
defer nsStore.Close()

err = nsStore.UpsertNamespace(ctx, storage.NamespaceRecord{
  Provider:      "github",
  AccountID:     "12345",
  InstallationID: "999",
  RepoID:        "42",
  Owner:         "dummy-org",
  RepoName:      "demo",
  FullName:      "dummy-org/demo",
  Visibility:    "private",
  DefaultBranch: "main",
  HTTPURL:       "https://github.com/dummy-org/demo",
})
```
