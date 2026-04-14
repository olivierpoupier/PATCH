---
name: patch-conventions
description: Go and Bubbletea conventions for the PATCH TUI project. Auto-apply these rules whenever editing, reviewing, or generating Go code in this repository. Trigger on ANY Go file change, code review, or new file creation in the PATCH project — even simple edits. This is a reference skill, not a task skill — silently apply conventions without announcing them.
---

# PATCH Project Conventions

Apply these conventions silently when editing or reviewing Go code in this project. Do not announce that you are following them — just follow them.

## Go Style

- **Receiver names**: 1-2 chars. `m` for Model, `t` for Theme, `s` for Scanner. Never `self` or `this`.
- **Naming**: MixedCaps exported, mixedCaps unexported. No underscores (except test files, build tags).
- **No stuttering**: `usb.Scanner` not `usb.USBScanner`. The package name is context.
- **Errors**: Always check. Wrap with `fmt.Errorf("context: %w", err)`. Never silently ignore.
- **Function length**: If `Update()` exceeds ~60 lines, extract handlers (`handleKeyPress`, `handleDataMsg`) returning `(Model, tea.Cmd)`.
- **Godoc**: `// TypeName does X` or `// FunctionName returns Y`.
- **Tests**: Table-driven with `t.Run` subtests.
- **Composition**: Embed structs, implement interfaces. Small interfaces (1-3 methods) defined at the consumer, not the implementor.

## Bubbletea (v2)

The Elm Architecture is the law: Model holds state, Update mutates, View renders.

- **View()** is pure. No I/O. No state mutation. No side effects. If you want to do something, it belongs in Update.
- **Async work** goes through `tea.Cmd`. Never spawn goroutines directly from Update.
  - Exception: event subscription with a `done` channel for cancellation (the `listenEvents` pattern).
- **tea.Batch** for concurrent commands, **tea.Sequence** for ordered.
- **Messages** are the only inter-component communication. No shared mutable state. No globals. No channels as state.
- **Unexported message types** stay in their own package — the type system handles routing.
- **tea.KeyPressMsg** (v2), not `tea.KeyMsg` (v1).

## PATCH Architecture

- **Theme**: All colors and styles come from `*tui.Theme`. Zero `lipgloss.NewStyle()` in tab code. Zero hardcoded hex colors. Semantic names (`Active`, `Error`, `Warning`), not visual (`Green`, `Red`).
- **Components**: Use `tui/components/` (ScrollableView, Table, Header, Footer). Don't reimplement.
- **View signature**: `View(width, height int)`. Layout math: `bodyHeight = height - lipgloss.Height(header) - lipgloss.Height(footer)`.
- **Scroll**: `m.scroll.ScrollToLine(m.table.Cursor())`.
- **Guards**: Every tab checks `m.focused` before handling keys, `m.active` before scheduling ticks.
- **Messages**: Cross-tab in `tui/messages.go` (exported). Tab-internal in `<tab>/messages.go` (unexported).
- **Imports**: `"github.com/olivierpoupier/patch/tui"`

## Anti-Patterns — Flag or Fix These

- `lipgloss.NewStyle()` outside `theme.go`
- Hardcoded color values (`#FF0000` etc.) outside `theme.go`
- Side effects in `View()`
- Goroutines spawned from `Update` without `tea.Cmd`
- Missing `m.active` check before scheduling a tick
- Missing `m.focused` check before handling key events
- Missing `done` channel cleanup in `SetActive(false)`
- `tea.KeyMsg` instead of `tea.KeyPressMsg`
- Shared mutable state between tabs
- Commands defined in `tab.go` instead of `messages.go`
