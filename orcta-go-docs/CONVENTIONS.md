# Coding Conventions

Rules for this codebase. Every domain package follows them. `internal/orders`, `internal/payments`, `internal/dispatch` are the reference implementations.

Source: `GO_ENGINEERING_PATTERNS.md` and `STYLE_GUIDE.md`. Where they overlap, this document is the single rule.

---

## 1. Packages & Project Layout

1. Put all application code in `internal/`. Packages: `accounts`, `auth`, `admin`, `riders`, `vendors`, `menus`, `orders`, `dispatch`, `geospatial`, `payments`, `notifications`, `pricing`, `rating`.
2. Package names: short, lowercase, single word. No `util` or `common`. Cross-domain value types get a dedicated package (`internal/money`) only when stable and shared by ≥3 domains. Otherwise duplicate the type.
3. `cmd/<name>/` is wiring only: parse config, build graph, start, shut down. `cmd/api/main.go` is the HTTP server. `cmd/worker/main.go` runs background jobs. No logic in `cmd/`.

## 2. Imports

Order: standard library, blank line, third-party, blank line, module-internal. Enforce with `goimports` or `task fmt`.

```go
import (
	"context"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/orctatech/orcta-go-backend/internal/orders"
)
```

## 3. Design & APIs

1. Declare interfaces where they are used, not where they are implemented. `dispatch` declares `RiderLocator`; `orders` declares `PaymentWaiter`, `FeeCalculator`; `admin` declares `RoleGranter`, `RiderManager`, `PayoutInitiator`. Implementations in `storage/*`, `payments`, `accounts` satisfy them implicitly. No interface lives beside its implementation.
2. Verify fake and adapter compliance at compile time: `var _ dispatch.RiderLocator = (*fakeLocator)(nil)`.
3. Every major type has a constructor that applies defaults: `orders.NewService(...)`, `payments.NewService(...)`. Optional configuration uses functional options (`dispatch.WithMatchTimeout(...)`), not config structs with many nullable fields.
4. Validate inputs at the top of a function and return immediately. Do not propagate invalid state.
5. `context.Context` is the first parameter of any function that does I/O, crosses a boundary, or can block. Propagate it. Do not store it on structs. Use unexported key types for request-scoped values.
6. File order: constructor, exported methods, unexported methods, helpers. Exported identifiers have doc comments that start with the identifier name.
7. `main` exits exactly once. `main` calls `run() error` and handles the error. Libraries never call `os.Exit` or `log.Fatal`. `cmd/api/main.go` and `cmd/worker/main.go` are wiring-only; `platform/` builds the graph.

### Sealed interfaces (tagged unions)

Go has no sum type. Choose by whether variants carry different data.

**Closed enum** — same shape, fixed set. Typed constant + `Valid()`; unknown values fail closed. Example: `PaymentMethod`.

**Sealed interface** — variants carry different data. Unexported marker method restricts implementers to the declaring package; `exhaustive` enforces all cases in `switch`.

```go
type ChargeResult interface{ isChargeResult() }

type ChargeSucceeded struct { ExternalRef string; SettledAt time.Time }
type ChargePending   struct { ExternalRef string }
type ChargeFailed    struct { Reason string }

func (ChargeSucceeded) isChargeResult() {}
func (ChargePending) isChargeResult()   {}
func (ChargeFailed) isChargeResult()    {}
```

Use a sealed interface instead of a nullable-field struct. Applies to `RiderAssignment`, `VendorAcceptance`, `ChargeResult`.

### Observability

`internal/observability` exposes `LoggerFromContext(ctx)` and `StartSpan(ctx, name)`. Domain packages import nothing from OpenTelemetry, Prometheus, or Sentry. Backend swaps must not ripple through ten packages.

### Wide events

Accumulate one structured event per unit of work and emit it once at the end. Every layer contributes to the same event. `dispatch` sets `rider_id`, `broadcast_attempt_count`; `orders` sets `order_id`, `vendor_id`, `time_to_confirm_ms`; middleware sets `trace_id`, `status_code`, `total_latency_ms`. One line, dozens of high-cardinality fields. Additive to level-based logging — `WARN` and `ERROR` still fire independently.

## 4. Errors

1. Wrap across package boundaries with `%w` and a package prefix: `fmt.Errorf("orders: transition to cancelled: %w", err)`.
2. Sentinel errors for expected failures, compared with `errors.Is`: `orders.ErrInvalidTransition`. Prefix `Err`.
3. Handle an error once: either handle it or propagate with context, not both. Best-effort side effects state explicitly why continuing is safe.
4. Use comma-ok for type assertions.
5. Do not panic in library code. Panics only for programmer errors at startup (`template.Must`).

## 5. Concurrency

1. No fire-and-forget goroutines. Every goroutine has a `stop` and `done` channel, a `Stop()` that closes `stop` and blocks on `done`, idempotent via `sync.Once`. `main` defers `Stop()`. Workers use ticker + `select` on `ctx.Done()`.
2. Wait for goroutines on shutdown. Server errors go through a size-one buffered channel and select against the signal channel.
3. `sync.RWMutex` for read-heavy, `sync.Mutex` otherwise. Zero-value mutexes are valid. The lock is a field of the struct it protects. Never copy after first use.
4. Buffered channels are size one or unbuffered. Size one requires a documented justification (`serveErr := make(chan error, 1)`).
5. `defer` the unlock. Keep critical sections minimal.
6. Loop with `select` on `ctx.Done()` or `stop`. Never `for range ticker.C` without an exit path.

## 6. Style & Naming

1. Match the file's existing conventions when editing. Modernize where listed here (`interface{}` → `any`).
2. Enums start at one, or zero is invalid and fails closed. Unknown `Role` has `Level() == 0` (least privilege). Zero must never grant access.
3. Initialize structs with field names. Omit zero-value fields. Use `var` for zero-value structs.
4. Reduce nesting. Use guard clauses and early returns. No `else` after a returning `if`.
5. No mutable globals. State is owned by structs and injected.
6. Constants for magic values, grouped at the top.
7. Do not use built-in names as identifiers (`cap`, `len`, `new`). MixedCaps, no snake_case. Prefix unexported package vars only on collision.
8. Use `any`, not `interface{}`.
9. Lines ≤ ~100 characters. Wrap long signatures and literals.
10. Group similar declarations in const/var blocks with a block doc comment.
11. One term per concept. `rider` not `driver`. `vendor` not `merchant`. `order` not `delivery`.

## 7. Data & Performance

1. `strconv` over `fmt` for string↔number (`strconv.Itoa`, not `fmt.Sprintf("%d", ...)`).
2. Specify capacity when known: `make([]time.Time, 0, len(hits)+1)`.
3. Avoid repeated string↔byte conversions in hot paths.
4. `time.Time` for instants, `time.Duration` for periods. No raw `int` seconds except at API boundaries, convert immediately.
5. Field tags on marshaled structs (`json:"..."`, `db:"..."`), `json:"-"` where a field must never serialize.
6. `nil` is a valid slice. Return `nil`, not empty, when there is nothing.
7. Money is `money.Money` (integer pesewas), never `float` or bare `int`. No package outside `payments` does money arithmetic.

## 8. Testing

1. Table-driven tests with `t.Run` for related cases. Cover success and error paths. Keep tables simple; extract setup to helpers.
2. Preconditions use `t.Fatal`/`t.Fatalf`; assertions use `t.Error`/`t.Errorf`.
3. HTTP: `httptest.NewRequest` + `httptest.NewRecorder` against the real router (`internal/api`), `httptest.NewServer` for clients.
4. Fakes over mocks. Small in-memory implementations with compile-time checks. Integration tests use a real database.
5. Helpers call `t.Helper()` and live in `*_test.go`.
6. Inject time. Do not sleep in tests.
7. CI runs `go test -race ./...`.

## 9. Security

Applies to all new code handling credentials.

1. Constant-time compare for secrets: `hmac.Equal` or `subtle.ConstantTimeCompare`, never `==`.
2. Hash secrets at rest (SHA-256 for high-entropy tokens), `json:"-"` on hash fields, never log or serialize, return plaintext once.
3. `crypto/rand`, 256 bits, hex-encoded, prefix `ort_`.
4. Fail closed: empty `WEBHOOK_SECRET` disables `/webhook`; unknown roles have zero privilege; 401 ≠ 403.
5. Do not trust `X-Forwarded-For` unless a trusted proxy rewrites it. Key limits on `RemoteAddr`.
6. Credential endpoints: 10/min vs API 100/min; respond with `Retry-After`.
7. Cookies: `HttpOnly` + `SameSite=Lax`, `Secure` when TLS, rotate on login, destroy on logout.
8. Bound request bodies with `io.LimitReader` and a named constant.
9. Generic errors outward ("Invalid token"), detailed audit events inward (`auth.bearer_failed` with IP/path, never the credential).

## 10. Tooling

- `task fmt` / `goimports` — required.
- `task lint` (`golangci-lint run`) — required. Enabled: `errcheck`, `govet`, `staticcheck`, `gosec`, `revive`, `goimports`, `misspell`, `unconvert`, `unparam`, `exhaustive`.
- `task test` — `go test -v ./...`; CI adds `-race`.
- No new third-party dependencies without discussion. Prefer stdlib.

## 11. Conformance Checklist

Before submitting, verify: gofmt-clean, imports grouped, errors wrapped with `%w`, sentinels via `errors.Is`, no new globals, goroutines have `Stop`, structs use field names, table-driven tests for success and error, secrets hashed and constant-time compared, sealed interfaces exhaustive, money is `money.Money`.
