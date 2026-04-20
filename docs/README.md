# PATCH Documentation

This folder documents the **device-view layer**: the per-device modal that opens when a user presses `enter` on a USB serial device in the USB tab. The material spans the `devices/` registry, the `serialterm/` I/O package, and the `DeviceView` overlay in `app.go`.

For the overall tab architecture (theme, shared components, tab interface, cross-tab messaging), see [`../ARCHITECTURE.md`](../ARCHITECTURE.md) at the repo root. That document explains how to add a *new tab*; this one explains how to add a *new device view* inside the existing USB tab.

## Reading order

1. **[device-architecture.md](device-architecture.md)** — Start here. The end-to-end flow from a keypress in the USB tab to a live serial-terminal modal. Covers the overlay model, the `DeviceView` interface, the `devices` registry, and the two existing implementations (Flipper, Generic).
2. **[adding-a-device.md](adding-a-device.md)** — Step-by-step walkthrough for adding a new device view. Worked example: a Raspberry Pi Pico (RP2040) view.
3. **[serial-terminal.md](serial-terminal.md)** — Reference for the `serialterm/` package: `Session`, `Scrollback`, message types, key translation. Consult while implementing.

## Source files per doc

| Doc | Covers |
|-----|--------|
| `device-architecture.md` | `app.go`, `tui/deviceview.go`, `tui/messages.go`, `tui/theme.go`, `devices/devices.go`, `devices/flipper/*`, `devices/generic/*`, `usb/tab.go` (entry point), `main.go` (blank imports) |
| `adding-a-device.md` | All of the above, walked through as a template |
| `serial-terminal.md` | `serialterm/session.go`, `serialterm/transport.go`, `serialterm/messages.go`, `serialterm/scrollback.go`, `serialterm/keys.go`, `serialterm/errors.go` |

## Companion skills

Two project-local Claude Code skills provide quick-reference material that these docs don't duplicate:

- **`patch-conventions`** — Go style, receiver naming, Bubbletea v2 patterns, anti-patterns. Applied automatically when editing Go code in this repo.
- **`patch-components`** — API reference for `tui/components/` (ScrollableView, DeviceTable, StatusHeader, HelpFooter, text utilities) and the `Theme` struct.

Both live under `.claude/skills/` and are auto-applied by Claude Code. Human contributors can read them directly.
