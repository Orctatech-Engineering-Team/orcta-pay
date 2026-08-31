# Go Engineering Patterns & Best Practices

**Source**: Analysis of [basecamp/once](https://github.com/basecamp/once) - A CLI/TUI tool for installing and managing web applications from Docker images.

---

## Table of Contents

1. [Architecture Patterns](#architecture-patterns)
2. [Design Patterns](#design-patterns)
3. [Concurrency Patterns](#concurrency-patterns)
4. [Data Patterns](#data-patterns)
5. [Testing Patterns](#testing-patterns)
6. [Code Style Patterns](#code-style-patterns)
7. [Real-World Use Cases](#real-world-use-cases)
8. [Key Takeaways](#key-takeaways)

---

## Architecture Patterns

### 1. Package Organization

The codebase follows the `internal/` package convention, which prevents external imports:

```
internal/
├── background/     # Background task runner
├── command/        # CLI command handlers
├── docker/         # Core Docker operations
├── fsutil/         # File system utilities
├── metrics/        # Prometheus metrics scraping
├── service/        # System service management
├── system/         # System stats collection
├── ui/             # TUI interface (Bubble Tea)
├── userstats/      # User statistics tracking
└── version/        # Version management & self-update
```

**Key Lesson**: Each package has a single responsibility and clear boundaries.

---

### 2. Interface-Based Design (Dependency Injection)

Use interfaces to decouple components and enable testing:

```go
// internal/docker/logs.go
type logsClient interface {
    ContainerLogs(ctx context.Context, container string, options container.LogsOptions) (io.ReadCloser, error)
    ContainerInspect(ctx context.Context, containerID string) (container.InspectResponse, error)
}

// internal/docker/scraper.go
type statsClient interface {
    ContainerList(ctx context.Context, options container.ListOptions) ([]container.Summary, error)
    ContainerStats(ctx context.Context, containerID string, stream bool) (container.StatsResponseReader, error)
}

// internal/service/service.go
type Service interface {
    IsInstalled(name string) bool
    Install(ctx context.Context, name, execPath, namespace string) error
    Remove(ctx context.Context, name string) error
    ServiceName(name string) string
}
```

**Key Lesson**: Define interfaces for external dependencies to enable mocking and testing.

---

### 3. Functional Options Pattern

Use functional options for flexible configuration:

```go
// internal/docker/namespace.go
type NamespaceOption func(*Namespace)

func WithApplications(apps ...ApplicationSettings) NamespaceOption {
    return func(ns *Namespace) {
        for _, s := range apps {
            ns.addApplication(s)
        }
    }
}

func NewNamespace(name string, opts ...NamespaceOption) (*Namespace, error) {
    // ... apply options
    for _, opt := range opts {
        opt(ns)
    }
    return ns, nil
}

// Usage
ns, err := docker.NewNamespace("test", docker.WithApplications(
    docker.ApplicationSettings{Name: "myapp", Host: "myapp.localhost"},
))
```

**Key Lesson**: Use functional options for optional configuration instead of many parameters.

---

## Design Patterns

### 4. Constructor Pattern with Defaults

Every major type has a constructor that applies sensible defaults:

```go
// internal/metrics/scraper.go
type ScraperSettings struct {
    Port       int
    BufferSize int
}

func (s ScraperSettings) withDefaults() ScraperSettings {
    if s.BufferSize == 0 {
        s.BufferSize = defaultBufferSize
    }
    return s
}

func NewMetricsScraper(settings ScraperSettings) *MetricsScraper {
    settings = settings.withDefaults()
    return &MetricsScraper{
        settings: settings,
        client:   &http.Client{Timeout: httpTimeout},
        services: make(map[string]*serviceData),
    }
}
```

**Key Lesson**: Always provide sensible defaults and use a `withDefaults()` method.

---

### 5. Sentinel Errors with Descriptions

Custom error types for better user experience:

```go
// internal/docker/errors.go
type DescribedError interface {
    error
    Description() string
}

type describedError struct {
    msg         string
    description string
}

func (e *describedError) Error() string       { return e.msg }
func (e *describedError) Description() string { return e.description }

// Define sentinel errors
var ErrProxyPortInUse = &describedError{
    msg:         "proxy port conflict",
    description: "Something else is using the web ports on this machine.",
}

var ErrAppNotStarted = &describedError{
    msg:         "application did not start",
    description: "The application did not start within the time limit.",
}

// Helper to extract description
func ErrorMessage(err error) string {
    var de DescribedError
    if errors.As(err, &de) {
        return de.Description()
    }
    return err.Error()
}
```

**Key Lesson**: Use `errors.As` for custom error types and provide user-friendly descriptions.

---

### 6. Context Propagation

Every function that does I/O accepts `context.Context` as the first parameter:

```go
func (a *Application) Deploy(ctx context.Context, progress DeployProgressCallback) error
func (a *Application) Backup(ctx context.Context) error
func (ns *Namespace) Refresh(ctx context.Context) error
func (s *Scraper) Scrape(ctx context.Context)
```

**Key Lesson**: Always pass context as the first parameter for cancellation and timeouts.

---

## Concurrency Patterns

### 7. Goroutine Management with Context

Background tasks use context for graceful shutdown:

```go
// internal/background/runner.go
func (r *Runner) Run(ctx context.Context) error {
    slog.Info("Starting background runner", "namespace", r.namespace, "check_interval", CheckInterval)

    ticker := time.NewTicker(CheckInterval)
    defer ticker.Stop()

    for {
        select {
        case <-ctx.Done():
            slog.Info("Shutting down")
            return nil
        case <-ticker.C:
            if updated, err := r.check(ctx); err != nil {
                slog.Error("Check failed", "error", err)
            } else if updated {
                return nil
            }
        }
    }
}
```

**Key Lesson**: Always use `select` with `ctx.Done()` for graceful shutdown.

---

### 8. Thread-Safe Data Structures

Ring buffer with explicit synchronization:

```go
// internal/docker/ring_buffer.go
// RingBuffer is a fixed-size circular buffer. It is not thread-safe;
// callers must synchronize access externally.
type RingBuffer[T any] struct {
    items []T
    head  int
    count int
}

func NewRingBuffer[T any](size int) *RingBuffer[T] {
    return &RingBuffer[T]{
        items: make([]T, size),
    }
}

func (b *RingBuffer[T]) Add(item T) {
    b.items[b.head] = item
    b.head = (b.head + 1) % len(b.items)
    if b.count < len(b.items) {
        b.count++
    }
}

// FetchNewestFirst returns up to n items in reverse chronological order
func (b *RingBuffer[T]) FetchNewestFirst(n int) []T {
    available := min(n, b.count)
    if available == 0 {
        return nil
    }

    result := make([]T, available)
    for i := range available {
        idx := (b.head - 1 - i + len(b.items)) % len(b.items)
        result[i] = b.items[idx]
    }
    return result
}

// Usage with RWMutex for thread safety
type LogStreamer struct {
    mu      sync.RWMutex
    lines   *RingBuffer[LogLine]
    version uint64
}

func (s *LogStreamer) Fetch(n int) []LogLine {
    s.mu.RLock()
    defer s.mu.RUnlock()
    return s.lines.FetchOldestFirst(n)
}

func (s *LogStreamer) addLine(line LogLine) {
    s.mu.Lock()
    defer s.mu.Unlock()
    s.lines.Add(line)
    s.version++
}
```

**Key Lesson**: Use `sync.RWMutex` for read-heavy workloads, document thread-safety requirements explicitly.

---

### 9. Stream Retry Pattern

Automatic reconnection for streaming APIs:

```go
// internal/docker/events.go
func (w *EventWatcher) watchLoop(ctx context.Context, out chan<- struct{}) {
    for {
        w.streamEvents(ctx, out)

        select {
        case <-ctx.Done():
            return
        case <-time.After(streamRetryDelay): // Pause before retrying
        }
    }
}

func (w *EventWatcher) streamEvents(ctx context.Context, out chan<- struct{}) {
    eventChan, errChan := w.client.Events(ctx, events.ListOptions{
        Filters: filterArgs,
    })

    prefix := w.namespace + "-"

    for {
        select {
        case <-ctx.Done():
            return
        case <-errChan:
            return
        case event := <-eventChan:
            name := event.Actor.Attributes["name"]
            if strings.HasPrefix(name, prefix) {
                select {
                case out <- struct{}{}:
                case <-ctx.Done():
                    return
                }
            }
        }
    }
}
```

**Key Lesson**: Implement retry loops with context cancellation for resilient streaming connections.

---

## Data Patterns

### 10. JSON Serialization in Docker Labels

Store configuration in Docker labels for persistence:

```go
// internal/docker/application_settings.go
type ApplicationSettings struct {
    Name       string             `json:"name"`
    Image      string             `json:"image"`
    Host       string             `json:"host"`
    DisableTLS bool               `json:"disableTLS"`
    EnvVars    map[string]string  `json:"env"`
    SMTP       SMTPSettings       `json:"smtp"`
    Resources  ContainerResources `json:"resources"`
    AutoUpdate bool               `json:"autoUpdate"`
    Backup     BackupSettings     `json:"backup"`
}

func (s ApplicationSettings) Marshal() string {
    b, _ := json.Marshal(s)
    return string(b)
}

func UnmarshalApplicationSettings(s string) (ApplicationSettings, error) {
    var settings ApplicationSettings
    err := json.Unmarshal([]byte(s), &settings)
    return settings, err
}

// Usage in container creation
resp, err := a.namespace.client.ContainerCreate(ctx,
    &container.Config{
        Image: a.Settings.Image,
        Labels: map[string]string{
            "once": a.Settings.Marshal(),  // Store settings as JSON label
        },
        Env: env,
    },
    // ...
)
```

**Key Lesson**: Use JSON serialization for storing structured data in metadata (labels, annotations).

---

### 11. Settings Validation

Validate configuration before use:

```go
func (s ApplicationSettings) Validate() error {
    if s.Image == "" {
        return ErrImageRequired
    }
    if s.Backup.AutoBackup && s.Backup.Path == "" {
        return ErrAutoBackupWithoutPath
    }
    return nil
}

func (s ApplicationSettings) Equal(other ApplicationSettings) bool {
    if s.Name != other.Name || s.Image != other.Image || s.Host != other.Host {
        return false
    }
    if s.Resources != other.Resources {
        return false
    }
    // ... compare all fields
    return true
}
```

**Key Lesson**: Validate settings early and return descriptive errors. Implement `Equal()` for comparison.

---

### 12. Environment Variable Builder

Build environment variables from settings:

```go
func (s ApplicationSettings) BuildEnv(vol ApplicationVolumeSettings) []string {
    env := []string{
        "SECRET_KEY_BASE=" + vol.SecretKeyBase,
        "VAPID_PUBLIC_KEY=" + vol.VAPIDPublicKey,
        "VAPID_PRIVATE_KEY=" + vol.VAPIDPrivateKey,
    }

    if !s.TLSEnabled() {
        env = append(env, "DISABLE_SSL=true")
    }

    if s.Resources.CPUs > 0 {
        env = append(env, "NUM_CPUS="+strconv.Itoa(s.Resources.CPUs))
    }

    env = append(env, s.SMTP.BuildEnv()...)

    for k, v := range s.EnvVars {
        env = append(env, k+"="+v)
    }

    return env
}
```

**Key Lesson**: Use builder pattern for constructing complex configurations.

---

## Testing Patterns

### 13. Table-Driven Tests with Subtests

Use subtests for related test cases:

```go
// internal/docker/application_test.go
func TestURL(t *testing.T) {
    // Local helper function for test setup
    newAppWithProxy := func(host string, disableTLS bool, proxySettings *ProxySettings) *Application {
        ns := &Namespace{}
        ns.proxy = &Proxy{Settings: proxySettings}
        return &Application{
            namespace: ns,
            Settings:  ApplicationSettings{Host: host, DisableTLS: disableTLS},
        }
    }

    t.Run("empty host", func(t *testing.T) {
        app := &Application{Settings: ApplicationSettings{}}
        assert.Equal(t, "", app.URL())
    })

    t.Run("nil namespace", func(t *testing.T) {
        app := &Application{Settings: ApplicationSettings{Host: "app.example.com"}}
        assert.Equal(t, "https://app.example.com", app.URL())
    })

    t.Run("default HTTP port", func(t *testing.T) {
        app := newAppWithProxy("app.localhost", true, &ProxySettings{HTTPPort: 80})
        assert.Equal(t, "http://app.localhost", app.URL())
    })

    t.Run("custom HTTPS port", func(t *testing.T) {
        app := newAppWithProxy("app.example.com", false, &ProxySettings{HTTPSPort: 8443})
        assert.Equal(t, "https://app.example.com:8443", app.URL())
    })

    t.Run("localhost disables TLS", func(t *testing.T) {
        app := newAppWithProxy("chat.localhost", false, &ProxySettings{HTTPPort: 9090})
        assert.Equal(t, "http://chat.localhost:9090", app.URL())
    })
}
```

**Key Lesson**: Use subtests for related test cases with local helper functions.

---

### 14. Testify Assertions

Use `require` for preconditions, `assert` for actual checks:

```go
import (
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
)

func TestSomething(t *testing.T) {
    // require stops test on failure (for preconditions)
    require.NoError(t, err)
    require.NotNil(t, app)

    // assert continues test on failure (for actual checks)
    assert.Equal(t, expected, actual)
    assert.Contains(t, err.Error(), "expected message")
    assert.ErrorIs(t, err, ErrVerificationFailed)
    assert.True(t, condition)
    assert.Len(t, items, 5)
}
```

**Key Lesson**: Use `require` for preconditions that would cause panics, `assert` for actual test checks.

---

### 15. Testing with httptest

Use `httptest.NewServer` for testing HTTP clients:

```go
func TestVerifyHTTP_Success(t *testing.T) {
    var requestPath string
    server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        requestPath = r.URL.Path
        w.WriteHeader(http.StatusOK)
    }))
    defer server.Close()

    app := &Application{
        Settings: ApplicationSettings{
            Host:       server.Listener.Addr().String(),
            DisableTLS: true,
        },
    }

    err := app.verifyHTTP(context.Background())
    assert.NoError(t, err)
    assert.Equal(t, HealthCheckPath, requestPath)
}

func TestVerifyHTTP_ServerError(t *testing.T) {
    server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        w.WriteHeader(http.StatusInternalServerError)
    }))
    defer server.Close()

    app := &Application{
        Settings: ApplicationSettings{
            Host:       server.Listener.Addr().String(),
            DisableTLS: true,
        },
    }

    err := app.verifyHTTP(context.Background())
    require.Error(t, err)
    assert.ErrorIs(t, err, ErrVerificationFailed)
    assert.Contains(t, err.Error(), "unexpected status 500")
}
```

**Key Lesson**: Use `httptest.NewServer` for testing HTTP clients without real network calls.

---

### 16. Testing Error Cases

```go
func TestWithApplicationNotFound(t *testing.T) {
    ns, err := docker.NewNamespace("test")
    require.NoError(t, err)

    err = withApplication(ns, "missing.localhost", "testing", func(app *docker.Application) error {
        t.Fatal("should not be called")
        return nil
    })

    require.Error(t, err)
    assert.Contains(t, err.Error(), `no application found at host "missing.localhost"`)
}

func TestErrorMessage(t *testing.T) {
    t.Run("returns description for described error", func(t *testing.T) {
        assert.Equal(t, ErrProxyPortInUse.Description(), ErrorMessage(ErrProxyPortInUse))
    })

    t.Run("returns description for wrapped described error", func(t *testing.T) {
        wrapped := fmt.Errorf("setup failed: %w", ErrProxyPortInUse)
        assert.Equal(t, ErrProxyPortInUse.Description(), ErrorMessage(wrapped))
    })

    t.Run("returns Error for plain error", func(t *testing.T) {
        err := errors.New("something broke")
        assert.Equal(t, "something broke", ErrorMessage(err))
    })
}
```

**Key Lesson**: Test both success and error paths. Test error wrapping/unwrapping behavior.

---

## Code Style Patterns

### 17. Method Organization

Organize methods in a consistent order:

```go
type Application struct {
    namespace    *Namespace
    Settings     ApplicationSettings
    Running      bool
    RunningSince time.Time
}

// Constructor
func NewApplication(ns *Namespace, settings ApplicationSettings) *Application {
    return &Application{
        namespace: ns,
        Settings:  settings,
    }
}

// Public methods first
func (a *Application) ContainerName(ctx context.Context) (string, error) { ... }
func (a *Application) Volume(ctx context.Context) (*ApplicationVolume, error) { ... }
func (a *Application) URL() string { ... }
func (a *Application) Deploy(ctx context.Context, progress DeployProgressCallback) error { ... }
func (a *Application) Start(ctx context.Context) error { ... }
func (a *Application) Stop(ctx context.Context) error { ... }

// Private
func (a *Application) verifyHTTP(ctx context.Context) error { ... }
func (a *Application) removeContainersExcept(ctx context.Context, keep string) error { ... }
func (a *Application) volumeMounts(vol *ApplicationVolume) []mount.Mount { ... }
func (a *Application) containerConfig(env []string) *container.Config { ... }

// Helpers (package-level, not methods)
func parseBackupTime(appName, filename string) (time.Time, bool) { ... }
func writeTarEntry(tw *tar.Writer, name string, data []byte) error { ... }
```

**Key Lesson**: Organize code: constructor -> public methods -> private methods -> helpers.

---

### 18. Error Wrapping

Always wrap errors with context using `%w`:

```go
// Good - wraps with context
return fmt.Errorf("restoring namespace: %w", err)
return fmt.Errorf("creating container: %w", err)
return fmt.Errorf("downloading binary: %w", err)

// Good - wraps sentinel errors
return fmt.Errorf("%w: %w", ErrVerificationFailed, err)
return fmt.Errorf("%w: %w", ErrDeployFailed, err)

// Good - creates new errors
return fmt.Errorf("no container found for app %s", a.Settings.Name)
return fmt.Errorf("invalid environment variable %q: must be in KEY=VALUE format", e)
```

**Key Lesson**: Use `%w` for error wrapping to preserve error chains. Always add context.

---

### 19. Constants for Magic Values

Extract magic values into named constants:

```go
const (
    DefaultHTTPPort    = 80
    DefaultHTTPSPort   = 443
    DefaultMetricsPort = 1318
    deployTimeout      = "120s"
)

const (
    AutomaticTaskInterval = 24 * time.Hour
    HealthCheckPath       = "/up"
    httpVerifyTimeout     = 30 * time.Second
)

const (
    BackupDataDir         = "data"
    BackupRetention       = 30 * 24 * time.Hour
    backupTimeFormat      = "20060102-150405"
)

const (
    DefaultLogBufferSize = 10000
    defaultTailLines     = "10000"
    scannerBufSize       = 64 * 1024
    scannerMaxSize       = 1024 * 1024
)
```

**Key Lesson**: Extract magic values into named constants. Use unexported constants for internal use.

---

### 20. Import Organization

Organize imports in sections:

```go
import (
    // Standard library
    "context"
    "errors"
    "fmt"
    "io"
    "log/slog"
    "strings"
    "time"

    // Third-party
    "github.com/docker/docker/api/types/container"
    "github.com/docker/docker/api/types/image"
    "github.com/docker/docker/api/types/mount"
    "github.com/docker/docker/api/types/network"

    // Internal
    "github.com/basecamp/once/internal/docker"
    "github.com/basecamp/once/internal/ui"
)
```

**Key Lesson**: Group imports: stdlib, third-party, internal. Alphabetize within groups.

---

## Real-World Use Cases

### 21. CLI Application with Cobra

```go
type deployCommand struct {
    cmd   *cobra.Command
    flags settingsFlags
}

func newDeployCommand() *deployCommand {
    d := &deployCommand{}
    d.cmd = &cobra.Command{
        Use:   "deploy <image>",
        Short: "Deploy an application",
        Args:  cobra.ExactArgs(1),
        RunE:  WithNamespace(d.run),
    }
    d.flags.register(d.cmd)
    return d
}

func (d *deployCommand) run(ctx context.Context, ns *docker.Namespace, cmd *cobra.Command, args []string) error {
    imageRef := args[0]

    if err := ns.Setup(ctx); err != nil {
        return fmt.Errorf("%w: %w", docker.ErrSetupFailed, err)
    }

    host := d.flags.host
    if host == "" {
        host = docker.NameFromImageRef(imageRef) + ".localhost"
    }

    if ns.HostInUse(host) {
        return docker.ErrHostnameInUse
    }

    settings, err := d.flags.buildSettings(imageRef, host)
    if err != nil {
        return err
    }

    baseName := docker.NameFromImageRef(imageRef)
    name, err := ns.UniqueName(baseName)
    if err != nil {
        return fmt.Errorf("generating app name: %w", err)
    }
    settings.Name = name

    app := docker.NewApplication(ns, settings)

    return runWithProgress("Deploying "+host, func(progress docker.DeployProgressCallback) error {
        if err := app.Deploy(ctx, progress); err != nil {
            if cleanupErr := app.Destroy(context.Background(), true); cleanupErr != nil {
                slog.Error("Failed to clean up after deploy failure", "app", name, "error", cleanupErr)
            }
            return fmt.Errorf("%w: %w", docker.ErrDeployFailed, err)
        }

        if err := app.VerifyHTTPOrRemove(ctx); err != nil {
            return err
        }

        return nil
    })
}
```

**Key Lesson**: Use Cobra for CLI applications. Wrap errors with context. Clean up on failure.

---

### 22. TUI with Bubble Tea (Elm Architecture)

The entire UI is built with the Elm architecture (Model-Update-View):

```go
// Component interface for sub-components
type Component interface {
    Init() tea.Cmd
    Update(tea.Msg) (Component, tea.Cmd)
    View() string
}

// Main App implements tea.Model
type App struct {
    namespace     *docker.Namespace
    currentScreen Component
    // ...
}

func (m *App) Init() tea.Cmd {
    return tea.Batch(
        m.currentScreen.Init(),
        m.scheduleNextScrapeTick(),
        m.watchForChanges(),
    )
}

func (m *App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    switch msg := msg.(type) {
    case tea.WindowSizeMsg:
        m.lastSize = msg

    case NavigateToDashboardMsg:
        if err := m.namespace.Refresh(m.watchCtx); err != nil {
            slog.Error("refreshing namespace", "err", err)
        }
        apps := m.namespace.Applications()
        return m, m.navigateTo(NewDashboard(m.namespace, apps, ...))

    // ...
    }

    var cmd tea.Cmd
    m.currentScreen, cmd = m.currentScreen.Update(msg)
    return m, cmd
}

func (m *App) View() tea.View {
    content := m.currentScreen.View()
    v := tea.NewView(content)
    v.AltScreen = true
    return v
}
```

**Key Lesson**: Use Elm architecture for complex UIs. Separate concerns with interfaces.

---

### 23. Self-Update Mechanism

```go
func (u *Updater) UpdateBinary() error {
    rel, err := u.fetchRelease()
    if err != nil {
        return err
    }

    if rel.TagName == u.currentVersion {
        fmt.Printf("You already have the latest version (%s)\n", u.currentVersion)
        return nil
    }

    assetName := fmt.Sprintf("once-%s-%s", runtime.GOOS, runtime.GOARCH)
    var downloadURL string
    for _, a := range rel.Assets {
        if a.Name == assetName {
            downloadURL = a.URL
            break
        }
    }
    if downloadURL == "" {
        return fmt.Errorf("no release asset found for %s", assetName)
    }

    execPath, err := os.Executable()
    if err != nil {
        return fmt.Errorf("finding executable path: %w", err)
    }

    tmpFile := filepath.Join(filepath.Dir(execPath), updateTempFile)
    if err := u.downloadBinary(downloadURL, tmpFile); err != nil {
        return err
    }

    if err := u.replaceBinary(execPath, tmpFile); err != nil {
        return err
    }

    fmt.Println("Update complete.")
    return nil
}

func (u *Updater) replaceBinary(execPath, newPath string) error {
    oldPath := execPath + ".old"

    if err := os.Rename(execPath, oldPath); err != nil {
        return fmt.Errorf("backing up current binary: %w", err)
    }

    if err := os.Rename(newPath, execPath); err != nil {
        // Attempt to restore from backup
        os.Rename(oldPath, execPath)
        return fmt.Errorf("replacing binary: %w", err)
    }

    os.Remove(oldPath)
    return nil
}
```

**Key Lesson**: Implement atomic file operations with rollback on failure.

---

### 24. Background Task Runner

```go
type Runner struct {
    namespace string
}

func NewRunner(namespace string) *Runner {
    return &Runner{
        namespace: namespace,
    }
}

func (r *Runner) Run(ctx context.Context) error {
    slog.Info("Starting background runner", "namespace", r.namespace, "check_interval", CheckInterval)

    scraper := userstats.NewScraper(r.namespace)
    go scraper.Run(ctx)

    ticker := time.NewTicker(CheckInterval)
    defer ticker.Stop()

    for {
        select {
        case <-ctx.Done():
            slog.Info("Shutting down")
            return nil
        case <-ticker.C:
            if updated, err := r.check(ctx); err != nil {
                slog.Error("Check failed", "error", err)
            } else if updated {
                return nil
            }
        }
    }
}

func (r *Runner) check(ctx context.Context) (bool, error) {
    ns, err := docker.RestoreNamespace(ctx, r.namespace)
    if err != nil {
        return false, fmt.Errorf("restoring namespace: %w", err)
    }

    state, err := ns.LoadState(ctx)
    if err != nil {
        return false, fmt.Errorf("loading state: %w", err)
    }

    if r.checkSelfUpdate(ctx, ns, state) {
        return true, nil
    }

    for _, app := range ns.Applications() {
        if !app.Running {
            continue
        }

        r.checkUpdate(ctx, app, state)
        r.checkBackup(ctx, app, state)
    }

    return false, nil
}
```

**Key Lesson**: Use goroutines for background tasks. Log errors but continue running.

---

### 25. Progress Tracking with Callbacks

```go
type DeployProgress struct {
    Stage      DeployStage
    Percentage int
}

type DeployProgressCallback func(DeployProgress)

func (a *Application) Deploy(ctx context.Context, progress DeployProgressCallback) error {
    if progress != nil {
        progress(DeployProgress{Stage: DeployStageDownloading, Percentage: 0})
    }

    // ... do work

    if progress != nil {
        progress(DeployProgress{Stage: DeployStageStarting})
    }

    // ... do more work

    if progress != nil {
        progress(DeployProgress{Stage: DeployStageFinished})
    }

    return nil
}

// Usage
return runWithProgress("Deploying "+host, func(progress docker.DeployProgressCallback) error {
    return app.Deploy(ctx, progress)
})
```

**Key Lesson**: Use callbacks for progress reporting. Allow nil callbacks for optional functionality.

---

## Key Takeaways

### Architecture
1. **Use `internal/` packages** to enforce encapsulation
2. **Single responsibility** - each package does one thing well
3. **Clear boundaries** - interfaces define contracts between packages

### Design
4. **Always use interfaces** for external dependencies
5. **Propagate context everywhere** for cancellation support
6. **Use functional options** for optional configuration
7. **Validate early, fail fast** with descriptive errors

### Concurrency
8. **Always use `select` with `ctx.Done()`** for graceful shutdown
9. **Use `sync.RWMutex`** for read-heavy concurrent access
10. **Implement retry loops** with context for resilient connections

### Data
11. **Use JSON serialization** for metadata storage
12. **Implement `Equal()`** for struct comparison
13. **Use builder pattern** for complex configurations

### Testing
14. **Use subtests** for related test cases
15. **Use `require`** for preconditions, `assert` for checks
16. **Use `httptest`** for HTTP client testing
17. **Test both success and error paths**

### Code Style
18. **Organize methods**: constructor -> public -> private -> helpers
19. **Wrap errors with context** using `%w`
20. **Extract constants** for magic values
21. **Group imports**: stdlib, third-party, internal

### Patterns
22. **Use Cobra** for CLI applications
23. **Use Elm architecture** for complex UIs
24. **Implement atomic operations** with rollback
25. **Use callbacks** for progress reporting

---

## Further Reading

- [Effective Go](https://go.dev/doc/effective_go)
- [Go Code Review Comments](https://go.dev/wiki/CodeReviewComments)
- [Uber Go Style Guide](https://github.com/uber-go/guide)
- [Dave Cheney's Blog](https://dave.cheney.net/)
- [Bubble Tea Documentation](https://github.com/charmbracelet/bubbletea)

---

*Document generated from analysis of [basecamp/once](https://github.com/basecamp/once) codebase.*
