# pkg/ — Server-Side Architecture

## OVERVIEW

12 packages implementing the webhook router server. Layered architecture with strict downward-only dependencies.

## DEPENDENCY LAYERS

```
┌─────────────────────────────────────────────────┐
│ ENTRY: server, api, webhook                     │  ← HTTP handlers, service impls
├─────────────────────────────────────────────────┤
│ ORCHESTRATION: oauth, providerinstance, drivers  │  ← Provider flows, caching
├─────────────────────────────────────────────────┤
│ CORE: core                                       │  ← Event, Publisher, RuleEngine, Config
├─────────────────────────────────────────────────┤
│ PERSISTENCE: storage                             │  ← 6 interfaces, GORM implementations
├─────────────────────────────────────────────────┤
│ FOUNDATION: auth, scm, providers/*               │  ← Provider configs, SCM clients
└─────────────────────────────────────────────────┘
```

**Rule: No upward imports.** `core/` and `storage/` never import `server/`, `api/`, or `webhook/`.

## WHERE TO LOOK

| Package | Purpose | Key Files |
|---------|---------|-----------|
| `core/` | Business logic: rules engine, config, publisher, event model | `rules.go` (engine), `config.go` (AppConfig+defaults), `publisher.go` (Relaybus), `event.go`, `flatten.go` (JSON→dot-notation) |
| `server/` | HTTP server bootstrap + middleware | `server_build.go` (wires everything), `server.go` (lifecycle), `auth_interceptor.go`, `tenant_interceptor.go` |
| `api/` | ConnectRPC service implementations (7 services) | One file per service: `rules_service.go`, `drivers_service.go`, `eventlogs_service.go`, etc. `connect_conversions.go` (proto↔domain) |
| `webhook/` | Provider webhook ingestion + event routing | `registry.go` (Provider interface), `github_handler.go`, `gitlab.go`, `bitbucket.go`, `publish.go` (match→publish) |
| `oauth/` | OAuth2 callback flows per provider | `registry.go`, `handler_github.go`, `handler_gitlab.go`, `handler_bitbucket.go`, `state.go` (CSRF), `refresh.go` |
| `auth/` | OIDC verification + provider auth config | `config.go` (ProviderConfig), `auth_config.go` (OAuth2Config), `oidc/verifier.go` |
| `storage/` | GORM persistence — 6 interfaces, 6 implementations | `store.go` (all interfaces+records), subdirs: `installations/`, `namespaces/`, `rules/`, `drivers/`, `eventlogs/`, `provider_instances/` |
| `drivers/` | Driver config cache + dynamic publisher | `cache.go` (driver config cache), `dynamic_cache.go` (publisher-per-driver), `config.go` (tenant publisher) |
| `providers/` | GitHub/GitLab/Bitbucket SDK wrappers | `github/` (App+OAuth client), subdirs per provider |
| `scm/` | Unified SCM client factory | `client.go` — resolves provider→client from auth context |
| `providerinstance/` | Per-tenant provider config + cache | Wraps `storage.ProviderInstanceStore` with `cache.TenantCache` |
| `gen/` | **DO NOT EDIT** — generated protobuf/ConnectRPC stubs | `cloud/v1/` (Go types), `cloudv1connect/` (service handlers) |

## STORAGE LAYER

6 domain entities, each with own subpackage under `storage/`:

| Entity | Table | Interface | Key Operations |
|--------|-------|-----------|----------------|
| InstallRecord | `githook_installations` | `Store` | Upsert, Get (by provider+account+install), List, Delete |
| NamespaceRecord | `githook_namespaces` | `NamespaceStore` | Upsert, Get (by provider+repo+instance), List (filtered), Delete |
| RuleRecord | `githook_rules` | `RuleStore` | CRUD. ListRules joins driver info |
| DriverRecord | `githook_drivers` | `DriverStore` | GetByID, GetByName, Upsert. Supports global + per-tenant |
| EventLogRecord | `githook_event_logs` | `EventLogStore` | CreateBatch, List (filtered), Analytics, Timeseries, Breakdown |
| ProviderInstanceRecord | `githook_provider_instances` | `ProviderInstanceStore` | CRUD. Per-tenant provider configs |

**Patterns:**
- Each subpackage: `Open()` factory → GORM connection → `AutoMigrate` → `Store` struct
- `store.go` (root) defines all interfaces + domain records + deterministic ID functions (SHA256)
- Multi-tenancy: `storage.WithTenant(ctx, id)` → all queries auto-filter by tenant
- Mock files (`mocks_*.go`) in root for testing

## CONVENTIONS (PKG-SPECIFIC)

- **server_build.go is the wiring hub**: All stores, caches, services, and handlers constructed here in `BuildHandler()`
- **Cleanup pattern**: `addCloser(func())` accumulates shutdown hooks; `cleanup()` runs them LIFO
- **Registry pattern**: `webhook.DefaultRegistry()` and `oauth.DefaultRegistry()` return provider registries iterated during server build
- **ConnectRPC interceptors**: Validation → Auth (OIDC) → Tenant extraction, applied in order via `connect.WithInterceptors()`
- **h2c wrapping**: Handler wrapped with `h2c.NewHandler()` for HTTP/2 cleartext (no TLS required)
- **CORS wide open**: `AllowOriginFunc: func(_ string) bool { return true }` — accepts all origins

## ANTI-PATTERNS

- **Never add circular imports** between packages — dependency flows strictly downward
- **Never query storage directly** from `webhook/` or `api/` — use the defined interfaces
- **Never skip `server_build.go`** when adding a new service — all wiring happens there
- **Never edit `gen/`** — regenerate with `buf generate`
