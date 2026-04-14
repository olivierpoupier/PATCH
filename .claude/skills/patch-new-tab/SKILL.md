---
name: patch-new-tab
description: Scaffold a new peripheral tab for the PATCH TUI project. Use when explicitly invoked via /patch-new-tab to generate the full tab package (tab.go, messages.go, scanner files), tests, and integration checklist.
disable-model-invocation: true
---

# New Tab Scaffolding for PATCH

This skill generates a complete, architecture-compliant tab package for PATCH — the multi-tab Bubbletea TUI for managing machine peripherals.

## Step 1: Gather Requirements

Before generating any code, ask the user for these details (present as a numbered list and wait for answers):

1. **Tab name** — lowercase package name (e.g., `wifi`, `storage`, `audio`, `display`)
2. **Peripheral description** — what hardware does this tab manage? (one sentence)
3. **Platform scanners needed** — which of: `darwin`, `linux`, `stub`? (default: all three)
4. **UI type** — Does this tab need a `DeviceTable` (list of items with columns), or just a `ScrollableView` (free-form rendered content), or both (like the USB tab's tree+table toggle)?
5. **If DeviceTable**: what columns? (provide name and approximate percent width, must sum to 100)
6. **Extra keybindings** — beyond the standard `up/k`, `down/j`, `enter`? (e.g., `t` toggle view, `p` power, `d` disconnect, `r` refresh)
7. **Scanner data types** — what domain types does the scanner return? (e.g., `[]Network`, `[]StorageDevice`, `[]AudioDevice`)

## Step 2: Read Current Architecture

Before generating code, read these files to get the latest patterns — they may have evolved since this skill was written:

- `ARCHITECTURE.md` (project root) — sections 2, 3, 4, 7, 10, 11
- `tui/tab.go` — the Tab interface
- `tui/theme.go` — the Theme struct (actual field names and types)
- `tui/components/scrollview.go` — ScrollableView API
- `tui/components/table.go` — DeviceTable API (note: `Column.Percent` is `int`, not `float64`; `View()` takes an optional `StyleFunc`)
- `tui/components/header.go` — `RenderHeader` signature and `HeaderField` struct
- `tui/components/footer.go` — `RenderFooter`, `RenderError` signatures and `KeyBinding` struct
- `tui/components/text.go` — `TruncateText`, `PadRight`, `ScrollText`
- An existing tab for reference (e.g., `usb/tab.go` + `usb/messages.go`)
- `platform/capabilities.go` + one platform variant (e.g., `platform/capabilities_darwin.go`)

This ensures the generated code matches current API signatures, import paths, and conventions.

## Step 3: Generate the Tab Package

Create the directory `<tabname>/` and generate these files:

### File 1: `<tabname>/tab.go`

Key requirements:
- Package name matches directory name
- Import path: `github.com/olivierpoupier/patch/<tabname>`
- `Model` struct with pointer receiver methods
- Constructor: `func New(theme *tui.Theme) *Model`
- Must implement `tui.Tab` interface: `Name()`, `Init()`, `Update()`, `View()`, `SetFocused()`, `SetActive()`
- Store `*tui.Theme` on the model, use `m.theme.Label`, `m.theme.Value`, etc. — never `lipgloss.NewStyle()` inline
- `components.ScrollableView` for scrollable content (always needed)
- `components.DeviceTable` if the user requested a table view
- `active`, `focused`, `initialized` bool fields
- `done chan struct{}` for cancellation (only if the tab uses event subscriptions)
- `err error` field for transient errors

**Update() rules:**
- `case refreshTickMsg:` must check `if !m.active { return m, nil }` before rescheduling
- `case tea.KeyPressMsg:` must check `if !m.focused { return m, nil }` before handling keys
- Standard keybindings: `"up"/"k"` cursor up, `"down"/"j"` cursor down, `"enter"` primary action
- All message types come from `messages.go` (unexported, same package)

**View() rules:**
- Accept `(width, height int)`, cache them on the model
- Layout: header + body + footer
- Use `components.RenderHeader(m.theme, ...)` for the header
- Use `components.RenderFooter(m.theme, ...)` for the footer
- Use `components.RenderError(m.theme, ...)` for errors (above footer)
- Compute `bodyHeight = height - headerHeight - footerHeight - 1`
- Set scroll size and content, scroll to cursor line
- If using DeviceTable: call `m.table.View(nil)` (or pass a custom `StyleFunc` if needed)

**SetActive() rules:**
- `SetActive(true)`: start polling — return `tea.Batch(fetchData(m.scanner), scheduleRefreshTick())`
- `SetActive(false)`: if using a `done` channel, `close(m.done)` then create a new one
- Guard: if `!m.initialized || m.scanner == nil`, don't start polling

**SetFocused() rules:**
- Just set `m.focused = focused`
- If the tab has a DeviceTable, also call `m.table = m.table.SetFocused(focused)`

### File 2: `<tabname>/messages.go`

Key requirements:
- Same package as tab.go
- `const refreshInterval = 5 * time.Second` (or 10s for heavy scanner operations)
- Unexported message types using grouped `type ( ... )` syntax:
  - `initMsg` — carries the initialized scanner
  - `dataMsg` — carries the scanned data
  - `errorMsg` — carries `err error`
  - `refreshTickMsg` — empty struct for polling ticks
  - Any additional messages the tab needs
- Command constructor functions:
  - `func initScanner() tea.Cmd` — creates scanner, does initial refresh, returns `initMsg` or `errorMsg`
  - `func fetchData(scanner *Scanner) tea.Cmd` — refreshes scanner, returns `dataMsg` or `errorMsg`
  - `func scheduleRefreshTick() tea.Cmd` — returns `tea.Tick(refreshInterval, ...)`
- Use `log/slog` for error logging inside commands
- **No commands in tab.go** — all `tea.Cmd` constructors live here

### File 3+: Scanner platform files

Generate one file per requested platform:

**`<tabname>/scanner_darwin.go`**
```
//go:build darwin
```
- `Scanner` struct with platform-specific fields
- `func NewScanner() (*Scanner, error)`
- `func (s *Scanner) Refresh() error` — calls system commands / APIs
- Domain-specific getter methods returning the types from the user's answer

**`<tabname>/scanner_linux.go`**
```
//go:build linux
```
- Same interface as darwin, different implementation

**`<tabname>/scanner_stub.go`**
```
//go:build !darwin && !linux
```
- (Or `!darwin` if no linux variant)
- Empty/no-op implementation returning nil/empty data

The build tag for the stub must be the negation of all real platforms. If only darwin: `!darwin`. If darwin+linux: `!darwin && !linux`.

### File 4: `<tabname>/scanner.go` (optional, shared types)

If the scanner has domain types, constants, or helper functions shared across platforms, put them in a file without build tags. This is where the Scanner interface for testing goes:

```go
// Scanner describes the platform-specific scanning capability.
type ScannerInterface interface {
    Refresh() error
    // ... domain-specific getters
}
```

## Step 4: Generate Test Stubs

Create `<tabname>/tab_test.go` with skeleton tests:

1. **Model unit test** — create model, send messages, assert state changes
2. **Command test** — call command constructors, assert returned message types
3. **Inactive tick test** — verify `refreshTickMsg` returns nil cmd when `m.active == false`
4. **Cancellation test** (if using done channel) — verify goroutines exit when done is closed

Use `t.Skip("TODO: implement")` in each test body so the file compiles but tests are clearly unfinished.

## Step 5: Run Architecture Checklist

After generating all files, walk through every item from ARCHITECTURE.md section 11 and report status:

- [ ] `tab.go` with Model implementing `tui.Tab` — DONE
- [ ] `messages.go` with unexported messages and commands — DONE
- [ ] `scanner_{platform}.go` files — DONE
- [ ] `*tui.Theme` in constructor, zero inline styles — DONE
- [ ] `components.ScrollableView` for scrollable content — DONE
- [ ] `components.DeviceTable` if showing items — DONE/N/A
- [ ] `components.RenderHeader` and `components.RenderFooter` — DONE
- [ ] `SetActive(true)` starts polling, `SetActive(false)` cleans up — DONE
- [ ] `m.active` check before rescheduling ticks — DONE
- [ ] `m.focused` check before handling keys — DONE
- [ ] Standard keybindings: up/k, down/j, enter — DONE
- [ ] Tab-specific keybindings in help footer — DONE
- [ ] `tui.DeviceEventMsg` handling if relevant — CHECK WITH USER
- [ ] Platform capability in `platform/capabilities_*.go` — **USER TODO**
- [ ] Tab registration in `main.go` — **USER TODO**
- [ ] Model unit tests — STUBS CREATED
- [ ] Command tests — STUBS CREATED
- [ ] Cancellation tests — STUBS CREATED

## Step 6: Integration Reminders

After the checklist, remind the user:

1. **Add capability flag**: Add `Has<TabName> bool` to the `Capabilities` struct in `platform/capabilities.go`, then set it appropriately in each `platform/capabilities_*.go` file.

2. **Register the tab** in `main.go`:
   ```go
   import "github.com/olivierpoupier/patch/<tabname>"
   
   // In the tab registration block:
   if caps.Has<TabName> {
       tabs = append(tabs, <tabname>.New(theme))
   }
   ```

3. **Implement the scanner**: The generated scanner files have TODO stubs — fill in the actual platform commands (e.g., `system_profiler` on macOS, sysfs on Linux).

4. **Build and test**:
   ```
   go build ./...
   go vet ./...
   go test ./<tabname>/...
   ```

## Important Notes

- The generated code should compile immediately (`go build ./...` passes) even before scanner implementation — the stub file provides no-op implementations.
- Follow the existing naming convention in the project: scanner platform files use `scanner_darwin.go` / `scanner_linux.go` / `scanner_stub.go` (some older tabs use `sysprof_` prefix — new tabs should use `scanner_`).
- The `DeviceTable.View()` method takes an optional `StyleFunc` parameter — pass `nil` for default styling.
- `Column.Percent` is `int` (not `float64`) — percentages should sum to 100.
- `HeaderField.Color` is `color.Color` (from `image/color`), not `lipgloss.Color`.
- Always import tea as: `tea "charm.land/bubbletea/v2"`
- Always import lipgloss as: `"charm.land/lipgloss/v2"`
