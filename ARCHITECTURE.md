# PATCH Architecture & Design Patterns Guide

## Context

PATCH (Peripheral Access Terminal for Connected Hardware) is a multi-tab TUI for managing machine peripherals. It currently has USB and Bluetooth tabs and is growing to 8-10+ tabs (WiFi, Storage, SSH, Audio, Display, etc.) with rich per-tab features.

This guide establishes the architecture patterns, conventions, and shared infrastructure that all tabs must follow. It is based on an audit of the codebase and research into Bubbletea/Bubbles/Lipgloss best practices.

> **Looking for the device-view layer?** The per-device modal that opens when a user presses `enter` on a USB serial device (Flipper Zero, generic serial terminal, future devices) is documented under [`docs/`](docs/README.md). Start with [`docs/device-architecture.md`](docs/device-architecture.md) for the end-to-end flow and [`docs/adding-a-device.md`](docs/adding-a-device.md) to build a new one.

---

## 1. Project Structure

### Project structure
```
patch/
  main.go                           # Entry point, tab registration
  app.go                            # Root model, tab bar, message routing
  
  tui/
    tab.go                          # Tab interface
    theme.go                        # Centralized Theme struct (replaces style.go)
    messages.go                     # Cross-tab shared message types
    components/
      scrollview.go                 # Viewport + scrollbar + auto-scroll
      table.go                      # Device table with cursor + proportional columns
      header.go                     # StatusHeader (key-value label pairs)
      footer.go                     # HelpFooter (keybinding help text)
      text.go                       # truncateText, padRight, scrollText (marquee)
  
  platform/
    capabilities.go                 # Platform interface + types
    capabilities_darwin.go          # macOS capabilities
    capabilities_linux.go           # Linux capabilities  
    capabilities_stub.go            # Fallback

  usb/
    tab.go                          # Model + Init/Update/View
    messages.go                     # Unexported message types + command constructors
    scanner.go                      # Domain logic (tree building, classification, PortGroup)
    scanner_darwin.go               # macOS system_profiler
    scanner_linux.go                # Linux sysfs + lsblk
    scanner_stub.go                 # Unsupported platforms
    tab_test.go                     # Model unit tests
    scanner_test.go                 # Scanner mock tests

  bluetooth/
    tab.go
    messages.go
    adapter.go
    composite.go
    ble_scanner.go
    scanner_darwin.go
    scanner_stub.go
    tab_test.go

  wifi/                             # Same 3-file minimum pattern
    tab.go
    messages.go
    scanner_darwin.go
    scanner_linux.go
    scanner_stub.go

  storage/
    tab.go
    messages.go
    ...

  ssh/
    tab.go
    messages.go
    ...

  logging/
    logging.go
```

### Convention: every tab package has at minimum
1. `tab.go` -- Model struct, New() constructor, Init/Update/View, SetFocused/SetActive
2. `messages.go` -- All unexported message types + all tea.Cmd constructor functions
3. One or more domain/scanner files (with platform variants via build tags)

---

## 2. Centralized Theme System

### Pattern: Theme struct in `tui/theme.go`

All colors and pre-built styles are centralized in the Theme struct. No tab should create inline `lipgloss.NewStyle()` calls or hardcode hex color values.

```go
package tui

import "charm.land/lipgloss/v2"

// Theme holds all application-wide colors and pre-built styles.
// Pass to tabs via their constructor: usb.New(theme).
type Theme struct {
    // -- Semantic colors --
    Active   lipgloss.Color // #04B575 - primary accent (borders, selected, headers)
    Inactive lipgloss.Color // #888888 - secondary (help text, disabled, dim)
    Error    lipgloss.Color // #FF4444 - error messages
    Warning  lipgloss.Color // #FFAA00 - warnings, "paired" status
    Text     lipgloss.Color // #FFFFFF - primary text
    TextDim  lipgloss.Color // #888888 - dimmed text (built-in devices, etc.)
    
    // -- Pre-built styles (created once, reused everywhere) --
    Label       lipgloss.Style // Bold + Active foreground (for "Adapter:", "Power:", etc.)
    Value       lipgloss.Style // Text foreground
    Dim         lipgloss.Style // TextDim foreground
    ErrorText   lipgloss.Style // Error foreground
    Help        lipgloss.Style // Inactive foreground
    
    // Table styles
    TableHeader lipgloss.Style // Bold + Active + padding
    TableCell   lipgloss.Style // Text + padding
    Selected    lipgloss.Style // Active background + black foreground + padding
    
    // Tab bar
    TabActive   lipgloss.Style // Bold + Active + rounded border
    TabInactive lipgloss.Style // Inactive + rounded border
}

func NewTheme() *Theme {
    active   := lipgloss.Color("#04B575")
    inactive := lipgloss.Color("#888888")
    // ... build all styles once ...
    return &Theme{ ... }
}
```

### How tabs use it
```go
// In main.go:
theme := tui.NewTheme()
m := newModel(theme, []tui.Tab{
    usb.New(theme),
    bluetooth.New(theme),
})

// In usb/tab.go:
type Model struct {
    theme *tui.Theme
    // ...
}

func (m Model) View(width, height int) string {
    label := m.theme.Label.Render("Devices:")
    // No more inline lipgloss.NewStyle() calls
}
```

### Rules
- **Never** create `lipgloss.NewStyle()` inline in View/Update methods
- **Never** hardcode color hex values outside of theme.go
- All colors must have semantic names (Active, Error, Warning) not visual names (Green, Red)
- Tab-specific styles that don't generalize (e.g., USB tree branch characters) can live in the tab's scope but must reference theme colors

---

## 3. Shared Component Library (`tui/components/`)

### Design principle: composable primitives
Components are small, focused, and composable. Each follows the Bubbles contract:
- `Model` struct holding component state
- `New()` constructor
- `Update(tea.Msg) (Model, tea.Cmd)` 
- `View() string`
- Getter/setter methods for configuration

Tabs compose these freely -- USB combines ScrollableView + its own tree renderer, Bluetooth combines ScrollableView + DeviceTable + marquee.

### Component A: ScrollableView (`tui/components/scrollview.go`)

Wraps `viewport.Model` with:
- Scrollbar overlay (the `overlayScrollbar` logic currently duplicated in both tabs)
- Auto-scroll-to-cursor (keep cursor in middle ~60% of viewport, 20% margin)
- Dimension management (SetWidth, SetHeight)

```go
type ScrollableView struct {
    viewport viewport.Model
    theme    *tui.Theme
}

func NewScrollableView(theme *tui.Theme) ScrollableView { ... }
func (s ScrollableView) SetContent(content string) ScrollableView { ... }
func (s ScrollableView) ScrollToLine(line int) ScrollableView { ... }
func (s ScrollableView) SetSize(w, h int) ScrollableView { ... }
func (s ScrollableView) Update(msg tea.Msg) (ScrollableView, tea.Cmd) { ... }
func (s ScrollableView) View() string { ... } // renders content + scrollbar overlay
```

### Component B: DeviceTable (`tui/components/table.go`)

Wraps `lipgloss/v2/table` with:
- Cursor-based row selection with highlighting
- Proportional column width allocation
- Integration with theme styles

```go
type Column struct {
    Title   string
    Percent float64 // fraction of available width
}

type DeviceTable struct {
    columns []Column
    rows    [][]string
    cursor  int
    theme   *tui.Theme
    width   int
}

func NewDeviceTable(theme *tui.Theme, columns []Column) DeviceTable { ... }
func (t DeviceTable) SetRows(rows [][]string) DeviceTable { ... }
func (t DeviceTable) SetWidth(w int) DeviceTable { ... }
func (t DeviceTable) CursorUp() DeviceTable { ... }
func (t DeviceTable) CursorDown() DeviceTable { ... }
func (t DeviceTable) Cursor() int { ... }
func (t DeviceTable) SelectedRow() []string { ... }
func (t DeviceTable) View() string { ... }
```

### Component C: StatusHeader (`tui/components/header.go`)

Renders key-value pairs in the label-value format both tabs use.

```go
type HeaderField struct {
    Label string
    Value string
    Color lipgloss.Color // optional override (e.g., red for "Power: Off")
}

func RenderHeader(theme *tui.Theme, fields [][]HeaderField, width int) string
// fields is rows of field pairs, rendered as "Label: Value  Label: Value"
```

### Component D: HelpFooter (`tui/components/footer.go`)

Renders keybinding help text consistently.

```go
type KeyBinding struct {
    Keys        string // "enter", "up/down/jk"
    Description string // "connect", "navigate"
}

func RenderFooter(theme *tui.Theme, bindings []KeyBinding, width int) string
func RenderError(theme *tui.Theme, err error, width int) string
```

### Component E: Text utilities (`tui/components/text.go`)

Currently duplicated between usb/tree.go and bluetooth/tab.go.

```go
func TruncateText(s string, maxWidth int) string    // "Text..." if too long
func PadRight(s string, width int) string            // pad with spaces
func ScrollText(s string, visibleWidth, offset int) string // marquee effect
```

---

## 4. Tab Interface & Lifecycle

### Current interface (keep as-is)
```go
type Tab interface {
    Name() string
    Init() tea.Cmd
    Update(msg tea.Msg) (Tab, tea.Cmd)
    View(width, height int) string
    SetFocused(focused bool) (Tab, tea.Cmd)
    SetActive(active bool) (Tab, tea.Cmd)
}
```

This interface is clean and sufficient. Do NOT add methods like SetTheme or SetSize -- pass theme via constructor, pass dimensions via View() parameters.

### Lifecycle contract every tab must follow

1. **Init()** -- Return a command that initializes the scanner/adapter asynchronously. Never block.
2. **SetActive(true)** -- Start polling/event subscriptions. Create a `done` channel for cancellation.
3. **SetActive(false)** -- Close the `done` channel. Stop discovery/scanning. Clean up goroutines.
4. **SetFocused(true)** -- Enable keyboard input handling in Update.
5. **SetFocused(false)** -- Disable keyboard input handling (return nil cmd for key events).
6. **Update** -- Only process own unexported message types + tea.KeyPressMsg (when focused). Ignore everything else.
7. **View** -- Pure render from state. Never trigger side effects. Reference theme for all styles.

### Refresh/polling convention
- Every tab that polls must check `m.active` before rescheduling ticks
- Default refresh interval: 5 seconds (consistent across tabs)
- System profiler / heavy operations: 10 seconds
- UI animations (marquee, etc.): 400ms or less
- Tick message types follow naming: `refreshTickMsg`, `scanTickMsg`, etc.

---

## 5. Message Routing & Cross-Tab Communication

### Message routing in app.go (keep current pattern)
The current broadcast pattern is correct and idiomatic:
- Key messages go to the active tab only (when focused) or to the tab bar (when not focused)
- All other messages are broadcast to all tabs
- Each tab ignores messages it doesn't own (unexported types in separate packages = zero-cost filtering)

### WindowSizeMsg propagation
`app.go` handles WindowSizeMsg by storing dimensions and then falling through to the broadcast loop, so all tabs receive the resize event and can cache dimensions for use during Update.

### Cross-tab messages (`tui/messages.go`)
For tabs that need to communicate (USB storage device appearing in Storage tab, SSH over WiFi, etc.), define shared exported message types:

```go
package tui

// DeviceEventMsg is emitted by any tab when a device connects/disconnects.
// Other tabs can react (e.g., Storage tab reacts to USB storage connection).
type DeviceEventMsg struct {
    DeviceID   string
    DeviceName string
    Type       string // "usb", "bluetooth", "wifi"
    Event      string // "connected", "disconnected"
}

// NetworkStateMsg is emitted by WiFi tab when network state changes.
// SSH tab can react to network availability.
type NetworkStateMsg struct {
    Connected bool
    SSID      string
}
```

Rules:
- Cross-tab messages live in `tui/messages.go` (exported, importable by all tab packages)
- Tab-internal messages stay in `<tab>/messages.go` (unexported)
- A tab emits a cross-tab message by returning it from a command
- The broadcast in app.go delivers it to all tabs naturally
- No shared mutable state -- all state flows through messages

---

## 6. Platform Abstraction (`platform/`)

### Pattern: capability detection + build-tag implementations

```go
// platform/capabilities.go
package platform

type Capabilities struct {
    HasBluetooth bool
    HasWiFi      bool
    HasUSB       bool // always true
    HasStorage   bool // always true
    HasSSH       bool // always true
    OS           string
}

func Detect() Capabilities // implemented per-platform via build tags
```

```go
// platform/capabilities_darwin.go
//go:build darwin

func Detect() Capabilities {
    return Capabilities{
        HasBluetooth: true,
        HasWiFi:      true,
        HasUSB:       true,
        HasStorage:   true,
        HasSSH:       true,
        OS:           "darwin",
    }
}
```

### Usage in main.go
```go
caps := platform.Detect()
var tabs []tui.Tab
// Always register
tabs = append(tabs, usb.New(theme), storage.New(theme), ssh.New(theme))
// Conditionally register
if caps.HasBluetooth { tabs = append(tabs, bluetooth.New(theme)) }
if caps.HasWiFi { tabs = append(tabs, wifi.New(theme)) }
```

### Per-tab platform code convention
Each tab keeps its own `scanner_{darwin,linux,stub}.go` files for implementation details. The platform package only handles capability detection (should this tab exist at all?), not implementation differences.

---

## 7. Command & Async Patterns

### Convention: all commands in messages.go
Every `tea.Cmd`-returning function lives in `<tab>/messages.go`. This makes it easy to find all async operations for a tab and to mock them in tests.

### Pattern: lifecycle-aware polling
```go
// In messages.go:
func scheduleRefreshTick() tea.Cmd {
    return tea.Tick(refreshInterval, func(time.Time) tea.Msg {
        return refreshTickMsg{}
    })
}

// In tab.go Update():
case refreshTickMsg:
    if !m.active {
        return m, nil // Stop polling when inactive
    }
    return m, tea.Batch(fetchData(m.scanner), scheduleRefreshTick())
```

### Pattern: cancellable event subscriptions
```go
// Used by tabs with real-time event streams (Bluetooth, WiFi)
func listenEvents(done <-chan struct{}) tea.Cmd {
    return func() tea.Msg {
        select {
        case event := <-eventChannel:
            return eventMsg{event}
        case <-done:
            return nil
        }
    }
}

// In SetActive(false):
close(m.done) // Cancels all goroutines using this channel
```

### Pattern: ordered async operations
Use `tea.Sequence` when operations must happen in order:
```go
// Connect, then refresh device list
return m, tea.Sequence(
    connectDevice(m.adapter, dev),
    fetchDevices(m.adapter),
)
```

### Error handling convention
- Each tab defines its own `errorMsg{err error}` type
- Errors are displayed in the footer via the shared `RenderError` function
- Transient errors (failed refresh) are overwritten on next successful refresh
- Fatal errors (scanner init failed) show a persistent message with retry hint

---

## 8. Keyboard Shortcuts Convention

### Global shortcuts (handled by app.go, always active)
| Key | Action |
|-----|--------|
| `q`, `ctrl+c` | Quit |
| `tab`, `right`, `l` | Next tab |
| `shift+tab`, `left`, `h` | Previous tab |
| `1`-`9` | Jump to tab N |
| `enter` | Focus into active tab |
| `esc` | Exit tab focus |

### Per-tab shortcuts (handled by tab, only when focused)
Every tab must support:
| Key | Action |
|-----|--------|
| `up`, `k` | Cursor up |
| `down`, `j` | Cursor down |
| `enter` | Primary action (connect, expand, open) |

Tab-specific shortcuts should be documented in the tab's help footer and follow conventions:
- `t` for toggling view modes
- `p` for power/toggle operations
- `r` for manual refresh
- `d` for delete/disconnect
- `/` for search/filter (future)

---

## 9. Testing Patterns

### Model unit tests (highest priority)
Bubbletea models are pure functions: Update takes model + message, returns model + command. Test them directly:

```go
func TestUSBTab_RefreshTickWhenInactive(t *testing.T) {
    m := usb.NewTestModel() // constructor that doesn't need real scanner
    m, _ = m.SetActive(false)
    updated, cmd := m.Update(refreshTickMsg{})
    if cmd != nil {
        t.Error("expected nil cmd when inactive, got non-nil")
    }
}
```

### Command tests
Commands are just functions returning tea.Msg. Call them and check the result:

```go
func TestFetchData_ReturnsDataMsg(t *testing.T) {
    scanner := newMockScanner(testBuses, testPorts)
    cmd := fetchData(scanner)
    msg := cmd()
    data, ok := msg.(dataMsg)
    if !ok {
        t.Fatalf("expected dataMsg, got %T", msg)
    }
    if len(data.groups) != 2 {
        t.Errorf("expected 2 groups, got %d", len(data.groups))
    }
}
```

### Scanner interface for mocking
For testability, scanners should implement an interface:
```go
type Scanner interface {
    Refresh() error
    Data() ([]USBBus, []ThunderboltPort)
}
```
Production code uses `*USBScanner` (concrete), tests use a mock. The `fetchData` command function can accept the interface.

### View snapshot tests (golden files)
```go
func TestUSBTab_TreeView(t *testing.T) {
    m := newModelWithTestData(...)
    got := m.View(80, 24)
    golden := filepath.Join("testdata", "tree_view.golden")
    if *update {
        os.WriteFile(golden, []byte(got), 0644)
    }
    want, _ := os.ReadFile(golden)
    if got != string(want) {
        t.Errorf("view mismatch, run with -update to regenerate")
    }
}
```

### Cancellation tests
```go
func TestListenEvents_CancelsOnDone(t *testing.T) {
    done := make(chan struct{})
    cmd := listenDeviceEvents(done)
    close(done)
    msg := cmd()
    if msg != nil {
        t.Error("expected nil msg after cancel")
    }
}
```

---

## 10. New Tab Template

When creating a new tab, copy this skeleton and fill in the domain-specific parts.

### `newtab/tab.go`
```go
package newtab

import (
    "github.com/olivierpoupier/patch/tui"
    "github.com/olivierpoupier/patch/tui/components"
    tea "charm.land/bubbletea/v2"
)

const tabName = "NewTab"

type Model struct {
    // Core
    theme       *tui.Theme
    scanner     *Scanner // your domain-specific scanner
    initialized bool
    active      bool
    focused     bool
    err         error
    done        chan struct{}

    // UI state
    cursor   int
    items    []YourItem // your domain data
    scroll   components.ScrollableView
    table    components.DeviceTable
    width    int
    height   int
}

func New(theme *tui.Theme) *Model {
    return &Model{
        theme: theme,
        done:  make(chan struct{}),
        scroll: components.NewScrollableView(theme),
        table:  components.NewDeviceTable(theme, []components.Column{
            {Title: "Name", Percent: 0.4},
            {Title: "Type", Percent: 0.3},
            {Title: "Status", Percent: 0.3},
        }),
    }
}

func (m *Model) Name() string { return tabName }

func (m *Model) Init() tea.Cmd {
    return initScanner()
}

func (m *Model) Update(msg tea.Msg) (tui.Tab, tea.Cmd) {
    switch msg := msg.(type) {

    case initMsg:
        m.scanner = msg.scanner
        m.initialized = true
        return m, tea.Batch(fetchData(m.scanner), scheduleRefreshTick())

    case dataMsg:
        m.items = msg.items
        m.err = nil
        // Rebuild table rows from items
        rows := make([][]string, len(m.items))
        for i, item := range m.items {
            rows[i] = []string{item.Name, item.Type, item.Status}
        }
        m.table = m.table.SetRows(rows)
        return m, nil

    case refreshTickMsg:
        if !m.active {
            return m, nil
        }
        return m, tea.Batch(fetchData(m.scanner), scheduleRefreshTick())

    case errorMsg:
        m.err = msg.err
        return m, nil

    // Cross-tab messages (from tui/messages.go)
    case tui.DeviceEventMsg:
        // React to events from other tabs if relevant
        return m, nil

    case tea.KeyPressMsg:
        if !m.focused {
            return m, nil
        }
        switch msg.String() {
        case "up", "k":
            m.table = m.table.CursorUp()
        case "down", "j":
            m.table = m.table.CursorDown()
        case "enter":
            // Primary action on selected item
        }
        return m, nil
    }

    return m, nil
}

func (m *Model) View(width, height int) string {
    m.width = width
    m.height = height

    if !m.initialized {
        return m.theme.Dim.Render("Initializing...")
    }

    // Header
    header := components.RenderHeader(m.theme, [][]components.HeaderField{
        {
            {Label: "Items", Value: fmt.Sprintf("%d", len(m.items))},
        },
    }, width)

    // Footer
    bindings := []components.KeyBinding{
        {Keys: "up/down/jk", Description: "navigate"},
        {Keys: "enter", Description: "select"},
    }
    footer := components.RenderFooter(m.theme, bindings, width)
    if m.err != nil {
        footer = components.RenderError(m.theme, m.err, width) + "\n" + footer
    }

    // Body
    bodyHeight := height - lipgloss.Height(header) - lipgloss.Height(footer)
    m.table = m.table.SetWidth(width)
    m.scroll = m.scroll.SetContent(m.table.View())
    m.scroll = m.scroll.SetSize(width, bodyHeight)
    m.scroll = m.scroll.ScrollToLine(m.table.Cursor())

    return header + "\n" + m.scroll.View() + "\n" + footer
}

func (m *Model) SetFocused(focused bool) (tui.Tab, tea.Cmd) {
    m.focused = focused
    return m, nil
}

func (m *Model) SetActive(active bool) (tui.Tab, tea.Cmd) {
    m.active = active
    if active {
        m.done = make(chan struct{})
        return m, tea.Batch(fetchData(m.scanner), scheduleRefreshTick())
    }
    close(m.done)
    return m, nil
}
```

### `newtab/messages.go`
```go
package newtab

import (
    "time"
    tea "charm.land/bubbletea/v2"
)

const refreshInterval = 5 * time.Second

// -- Message types (unexported = only this package handles them) --

type initMsg struct{ scanner *Scanner }
type dataMsg struct{ items []YourItem }
type errorMsg struct{ err error }
type refreshTickMsg struct{}

// -- Command constructors --

func initScanner() tea.Cmd {
    return func() tea.Msg {
        scanner, err := NewScanner()
        if err != nil {
            return errorMsg{err}
        }
        return initMsg{scanner}
    }
}

func fetchData(scanner *Scanner) tea.Cmd {
    return func() tea.Msg {
        if err := scanner.Refresh(); err != nil {
            return errorMsg{err}
        }
        return dataMsg{items: scanner.Items()}
    }
}

func scheduleRefreshTick() tea.Cmd {
    return tea.Tick(refreshInterval, func(time.Time) tea.Msg {
        return refreshTickMsg{}
    })
}
```

### `newtab/scanner_darwin.go`
```go
//go:build darwin

package newtab

type Scanner struct { /* ... */ }

func NewScanner() (*Scanner, error) { /* ... */ }
func (s *Scanner) Refresh() error { /* ... */ }
func (s *Scanner) Items() []YourItem { /* ... */ }
```

### `newtab/scanner_stub.go`
```go
//go:build !darwin && !linux

package newtab

type Scanner struct{}

func NewScanner() (*Scanner, error) { return &Scanner{}, nil }
func (s *Scanner) Refresh() error   { return nil }
func (s *Scanner) Items() []YourItem { return nil }
```

### Registration in `main.go`
```go
caps := platform.Detect()
theme := tui.NewTheme()
var tabs []tui.Tab
tabs = append(tabs, usb.New(theme))
if caps.HasBluetooth { tabs = append(tabs, bluetooth.New(theme)) }
if caps.HasNewFeature { tabs = append(tabs, newtab.New(theme)) }
// ...
m := newModel(theme, tabs)
```

---

## 11. Checklist for Adding a New Tab

- [ ] Create `<tab>/tab.go` with Model implementing `tui.Tab` interface
- [ ] Create `<tab>/messages.go` with unexported message types and command constructors
- [ ] Create `<tab>/scanner_{darwin,linux,stub}.go` platform implementations
- [ ] Accept `*tui.Theme` in constructor, use theme styles everywhere (zero inline styles)
- [ ] Use `components.ScrollableView` for scrollable content
- [ ] Use `components.DeviceTable` if showing a list of items
- [ ] Use `components.RenderHeader` and `components.RenderFooter` for consistent layout
- [ ] Implement `SetActive(true)` to start polling, `SetActive(false)` to close `done` channel
- [ ] Check `m.active` before rescheduling any tick
- [ ] Check `m.focused` before handling key events
- [ ] Add standard keybindings: up/k, down/j, enter for primary action
- [ ] Document tab-specific keybindings in help footer
- [ ] Handle `tui.DeviceEventMsg` if cross-tab events are relevant
- [ ] Add platform capability check in `platform/capabilities_*.go`
- [ ] Register tab conditionally in `main.go` based on platform capabilities
- [ ] Write model unit tests for Update logic
- [ ] Write command tests for async operations
- [ ] Write cancellation tests for event subscriptions

---

## 12. Key Infrastructure Files

### Shared infrastructure (do not modify without understanding impact)
- `tui/theme.go` -- Centralized Theme struct with all colors and styles
- `tui/tab.go` -- Tab interface (the contract all tabs implement)
- `tui/messages.go` -- Cross-tab shared message types
- `tui/components/scrollview.go` -- ScrollableView (viewport + scrollbar + auto-scroll)
- `tui/components/table.go` -- DeviceTable (cursor-based table with proportional columns)
- `tui/components/header.go` -- StatusHeader renderer
- `tui/components/footer.go` -- HelpFooter + error renderer
- `tui/components/text.go` -- TruncateText, PadRight, ScrollText utilities
- `platform/capabilities.go` + platform variants -- Platform capability detection
- `app.go` -- Root model, tab bar, message routing
- `main.go` -- Tab registration, platform detection, theme creation

### Verification checklist
1. Build: `go build ./...`
2. Vet: `go vet ./...`
3. Test: `go test ./...`
4. Run on macOS: verify all tabs render correctly
5. Run on Linux: verify platform-specific tabs work
6. Resize terminal: verify all tabs respond to window resize
7. Switch tabs rapidly: verify no goroutine leaks
