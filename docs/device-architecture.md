# Device-View Architecture

This document traces what happens from the moment a user presses `enter` on a USB serial device in the USB tab to the moment a live serial-terminal modal is on screen and bytes are flowing. It is organised as a top-down tour, matching the order a developer would naturally trace when reading the code.

## 1. The flow at a glance

```
┌────────────────┐
│  usb tab       │  user selects a serial device, presses enter
│  tab.go:116    │  emits tui.OpenDeviceMsg{Device: info, Source: "usb"}
└───────┬────────┘
        │
        ▼
┌────────────────┐
│  app.go        │  receives OpenDeviceMsg (app.go:162)
│                │  calls openDeviceView (app.go:101)
│                │    ├─ devices.Resolve(info)        ──► picks Descriptor
│                │    ├─ applies Baud/Name defaults
│                │    ├─ blurs the source tab
│                │    ├─ desc.New(theme)              ──► constructs DeviceView
│                │    ├─ view.SetSize(w, h)
│                │    └─ view.Open(info)              ──► returns tea.Cmd
└───────┬────────┘
        │
        ▼
┌────────────────┐
│  DeviceView    │  Open returns serialterm.OpenSerialSession(path, baud)
│  (flipper /    │  goroutine opens port, posts OpenedMsg
│   generic)     │  Update handles OpenedMsg → Install + WaitRx
│                │  each RxMsg feeds Scrollback, re-arms WaitRx
│                │  each key: KeyToBytes → Session.Send
└───────┬────────┘
        │   user presses esc
        ▼
┌────────────────┐
│  app.go        │  receives tui.CloseDeviceMsg (app.go:166)
│                │  calls closeDeviceView (app.go:127)
│                │    ├─ view.Close()                 ──► stops I/O
│                │    └─ restores focus to source tab
└────────────────┘
```

Every arrow in this diagram is a single function call or a single Bubbletea message. The whole mechanism is standard Elm Architecture: no shared mutable state, no callbacks, no locks outside the tiny `devices` registry.

## 2. The overlay model

A device view is not a tab. It is an **overlay modal** owned by the root model (`app.go`). Three fields on `model` (`app.go:15-28`) manage it:

| Field | Type | Purpose |
|-------|------|---------|
| `deviceView` | `tui.DeviceView` | The active view, or `nil` when closed |
| `deviceOpen` | `bool` | Whether the modal is currently rendered |
| `deviceReturnFocus` | `bool` | Whether `focusContent` was true when the modal opened, so it can be restored on close |

The overlay's lifecycle is split across two small methods:

- **`openDeviceView`** (`app.go:101`) resolves the descriptor, constructs the view, sizes it, opens it, and sets `deviceOpen = true`.
- **`closeDeviceView`** (`app.go:127`) calls `view.Close()`, clears `deviceView`, and restores tab focus if needed.

The focus bookkeeping is important. When a device view opens, the source tab is explicitly blurred (`app.go:114-118`) so it won't try to handle keys while the modal is on screen. On close, focus is restored (`app.go:133-138`) only if the source tab had focus before the modal opened.

**Key intercept.** While `deviceOpen` is true, every `tea.KeyPressMsg` goes to the device view first (`app.go:173-177`) — *including* `ctrl+c`, which is forwarded to the device as a `0x03` byte. This means the normal "quit" shortcut does not work inside the modal: press `esc` to close the modal, then `ctrl+c` to quit. This is called out in the comment at `app.go:171-172`.

**Rendering.** In `View()` (`app.go:262-266`), if `deviceOpen` is set, the root model renders *only* the device view's output and returns. The tab bar is hidden — the modal takes over the full terminal. `AltScreen = true` is kept on for clean redraw.

**Window resize.** The root model forwards `WindowSizeMsg` to the device view via `SetSize` (`app.go:150-152`) so the modal can recompute its layout. The view is *not* re-initialised on resize.

## 3. The `DeviceView` interface

Every device package must satisfy `tui.DeviceView` (`tui/deviceview.go:12-33`):

```go
type DeviceView interface {
    Open(info SerialDeviceInfo) tea.Cmd
    Update(msg tea.Msg) (DeviceView, tea.Cmd)
    View(width, height int) string
    SetSize(width, height int)
    Close()
    Name() string
}
```

Six methods, each with a small, tightly specified contract:

| Method | Called when | Must do | Must not do |
|--------|-------------|---------|-------------|
| `Open(info)` | once per modal opening, right after construction | reset per-device state, return the command that opens I/O (almost always `serialterm.OpenSerialSession`) | assume I/O is already live |
| `Update(msg)` | for every message while `deviceOpen` | handle `serialterm.OpenedMsg`/`OpenErrMsg`/`RxMsg`/`RxClosedMsg`/`TxDoneMsg` and `tea.KeyPressMsg`; return `self` cast to `DeviceView` | spawn goroutines outside of a `tea.Cmd`; keep holding on to closed channels |
| `View(w, h)` | once per frame while `deviceOpen` | render a full-screen string from state only | mutate state; do I/O |
| `SetSize(w, h)` | on window resize | cache the dimensions; the next `View` call will use them | render or re-layout eagerly |
| `Close()` | once, on modal tear-down | call `session.Close()`, clear errors, clear disconnect flag | close twice safely — but see below |
| `Name()` | for logs and error messages | return a short identifier | do anything else |

**Safety note on `Close()`.** The contract (`tui/deviceview.go:27-29`) says `Close()` must be safe on a freshly constructed view. This matters because if `Open()` never fires (the app aborts before modal activation), `closeDeviceView` may still call it. Both existing implementations satisfy this because `Session.Close()` is a no-op on a nil / empty session (`serialterm/session.go:85-98`).

## 4. `SerialDeviceInfo` — the input

The data bag `Open` receives is `tui.SerialDeviceInfo` (`tui/messages.go:45-55`):

```go
type SerialDeviceInfo struct {
    VID          string // "0483" — normalised, no 0x prefix
    PID          string // "5740"
    Name         string // user-friendly
    VendorName   string
    Product      string // from serial enumerator
    SerialNumber string
    PortPath     string // "/dev/cu.usbmodem14203" or "/dev/ttyACM0"
    Baud         int
    ProfileKey   string
}
```

The USB tab populates this in `resolveSerialPort` (`usb/tab.go:414`), which cross-references `USBDevice` metadata with the OS serial enumerator (`go.bug.st/serial/enumerator`) to find the port path. Before the struct reaches `Open`, the root model applies `Descriptor`-level defaults for empty `Baud` and `Name` (`app.go:107-112`).

Implementations should still guard against zeros — both Flipper (`devices/flipper/flipper.go:66-71`) and Generic (`devices/generic/generic.go:49-54`) re-check and apply their own defaults. This keeps the view usable if someone constructs it outside the normal flow (e.g., in tests).

## 5. The `devices` registry

The registry lives in `devices/devices.go`. It is a compile-time plugin system modelled after `image.RegisterFormat` (`devices/devices.go:1-6`).

### `Descriptor`

```go
type Descriptor struct {
    Key   string                                  // "flipper_zero" — for logs
    Name  string                                  // "Flipper Zero" — default if SerialDeviceInfo.Name is empty
    Baud  int                                     // 115200 — default if SerialDeviceInfo.Baud is 0
    Match func(info tui.SerialDeviceInfo) bool    // nil for fallback
    New   func(theme *tui.Theme) tui.DeviceView   // factory
}
```

One `Descriptor` per device type. The factory `New` is called every time the modal opens — device views are **not reused**, they are constructed fresh and torn down on close.

### `Register` semantics

`Register` (`devices/devices.go:51-60`) splits on `Match`:

- **Non-nil `Match`** → appended to the `descriptors` slice in registration order.
- **Nil `Match`** → replaces the single `fallback` slot (last registration wins).

Registration happens at process start via `init()` functions in each device package. The blank imports in `main.go:9-10` are what pull those `init` functions in:

```go
_ "github.com/olivierpoupier/patch/devices/flipper"
_ "github.com/olivierpoupier/patch/devices/generic"
```

Without the blank import, the linker drops the package and the device never registers.

### `Resolve` algorithm

`Resolve` (`devices/devices.go:65-77`) is intentionally trivial:

```
for d in descriptors:           # in registration order
    if d.Match(info):
        return d
if fallback != nil:
    return fallback
return not-found
```

**First match wins.** This means registration order matters: register the most specific matcher first. The test `TestResolvePrefersEarlierRegistration` (`devices/devices_test.go:61-83`) spells this out — a descriptor with an exact `(VID, PID)` match registered before a broader VID-only wildcard correctly claims the specific device while the wildcard still catches everything else on that VID.

### `HasCustom`

`HasCustom` (`devices/devices.go:82-91`) is a sibling of `Resolve` that skips the fallback: it returns true *only* if a matcher-bearing descriptor matches. The USB tab uses it at `usb/tab.go:495` to decide whether to render the `⏎ serial` hint in the Action column — the hint appears only for devices that have a dedicated matcher, so the table surfaces "this device is recognised" as a UI affordance. The fallback itself is invisible.

### `MatchVIDPID` and `NormalizeHex`

The common matcher factory is `MatchVIDPID(vid, pid)` (`devices/devices.go:96-108`):

- Both VID and PID are normalised via `NormalizeHex` (strip `0x` / `0X` prefix, lowercase, trim whitespace — see `devices/devices.go:110-115`).
- An **empty PID** acts as a **VID-only wildcard**. The ESP32 entry uses this pattern (`devices/generic/init.go:20`): VID `303a` matches any Espressif chip, regardless of specific PID.

The hex normalisation is covered by `TestMatchVIDPIDNormalizesHexPrefix` and `TestNormalizeHex` (`devices/devices_test.go:117-140`) — `"0x0483"`, `"0X0483"`, and `"0483"` all match the same devices.

## 6. Worked example: Flipper Zero

`devices/flipper/` is a **custom** device view. It does two things the generic view does not:

### Registration

```go
// devices/flipper/init.go:8-18
devices.Register(devices.Descriptor{
    Key:   "flipper_zero",
    Name:  "Flipper Zero",
    Baud:  115200,
    Match: devices.MatchVIDPID("0483", "5740"),
    New:   func(theme *tui.Theme) tui.DeviceView { return New(theme) },
})
```

Exact VID/PID match for the STM32-based Flipper. Registered first (alphabetically `flipper` comes before `generic`, and Go's blank-import order matches file order within `main.go`) so it always wins over the generic fallback.

### On-connect diagnostic queries

When the port opens, Flipper's `handleOpened` (`devices/flipper/flipper.go:119-127`) does two things in parallel via `tea.Batch`:

1. Starts listening for RX (`session.WaitRx()`).
2. Calls `startHeaderCapture` (`devices/flipper/flipper.go:190-202`), which sends `device_info\r\n` and `info power\r\n` over the wire, flips `capturing = true`, and schedules `captureEndMsg` after 900 ms (`captureDuration`, `devices/flipper/flipper.go:21`).

While `capturing` is true, `handleRx` (`devices/flipper/flipper.go:129-139`) diverts incoming bytes into `captureBuf` instead of the scrollback. This keeps the diagnostic response out of the user-visible terminal. After 900 ms, `handleCaptureEnd` (`devices/flipper/flipper.go:150-159`) parses the buffer via `parseFlipperKV` and populates `headerFields`. Subsequent RX goes to the scrollback normally.

### Header rendering

The parsed `headerFields` drive an extra row in the terminal header. `renderHeader` (`devices/flipper/flipper.go:277-292`) walks `headerFieldOrder` (`firmware_version`, `hardware_ver`, `charge.level` — see `devices/flipper/flipper.go:25`) and emits a `fieldLine` with pretty labels from `prettyLabel` (`devices/flipper/flipper.go:400-419`). A `%` suffix is appended to `charge.level` for display (`devices/flipper/flipper.go:284-286`).

### Extra keybinding

Flipper adds `ctrl+r` to re-run the diagnostic capture without reopening the port (`devices/flipper/flipper.go:168-169`). This is how the header is refreshed when, e.g., the battery level changes.

## 7. Worked example: Generic

`devices/generic/` is both a **plain fallback terminal** and the host for six **known-chip entries** that use the same view with different display names.

### Registration

`devices/generic/init.go:12-43` registers seven descriptors that all point to the same factory:

| Key | VID | PID | Display name |
|-----|-----|-----|--------------|
| `esp32` | `303a` | (wildcard) | ESP32 |
| `arduino` | `2341` | (wildcard) | Arduino |
| `pluto_sdr` | `0456` | `b673` | Pluto SDR |
| `ftdi` | `0403` | (wildcard) | FTDI USB-Serial |
| `ch340` | `1a86` | `7523` | CH340 USB-Serial |
| `cp2102` | `10c4` | `ea60` | CP2102 USB-Serial |
| `generic` | — | — | USB Serial Device (**fallback**) |

**These entries exist only to set friendly names and baud defaults.** They do not change the view's behaviour — every one of them calls the same `factory` and produces the same `generic.Model`. If a developer wants to *customise* behaviour for one of these chips (e.g., add ESP32-specific bootloader detection), they should move that entry into a new dedicated package (see [adding-a-device.md](adding-a-device.md)).

### Behaviour

`generic.Model` (`devices/generic/generic.go:19-31`) is a minimal version of the Flipper model: no `headerFields`, no `capturing`, no `captureBuf`. `Open` (`devices/generic/generic.go:47-59`) just opens the port. `Update` (`devices/generic/generic.go:72-114`) handles exactly the five `serialterm` messages plus `tea.KeyPressMsg`. RX goes straight to the scrollback.

Keybindings are the minimum: `esc` closes, `ctrl+l` clears, anything else goes through `serialterm.KeyToBytes` to the device (`devices/generic/generic.go:96-111`).

## 8. The terminal theme

Device modals render in a dedicated amber-on-dark palette called `TerminalTheme` (`tui/theme.go:46-66`). It lives on the main `Theme` as `Theme.Terminal` and is reached via `m.theme.Terminal.*`.

| Style | Used for |
|-------|----------|
| `Banner` | bold amber — ASCII art at the top of the header |
| `HeaderKey` | dim amber — labels (`Device:`, `Port:`, …) |
| `HeaderVal` | bold warm text — values |
| `Body` | warm text — scrollback default |
| `Dim` | faded text — placeholders (`Opening serial port…`, `(waiting for output — type to send bytes)`) |
| `Separator` | dim amber — the `═════` horizontal rules framing the header |
| `Border` | amber — heavy borders |
| `DisconnectBanner` | red background, bold — `DEVICE DISCONNECTED` and `FAILED TO OPEN PORT` |
| `Help` | faded text — footer hints |

**Don't use the main `Theme` styles inside a device modal.** The main theme (green accent on black) is designed for the tab bar; mixing it in the modal looks broken. Stick to `m.theme.Terminal` everywhere inside a device view, and the `patch-conventions` skill's "no raw lipgloss" rule still applies — no inline `lipgloss.NewStyle()` calls, no hardcoded hex colors.

## 9. Shared keybindings

Both existing implementations handle the same core set:

| Key | Action | Where |
|-----|--------|-------|
| `esc` | emit `tui.CloseDeviceMsg{}` — the root model tears down the overlay | `generic/generic.go:98-99`, `flipper/flipper.go:163-164` |
| `ctrl+l` | `scrollback.Clear()` — wipes the visible terminal, not the device | `generic/generic.go:100-102`, `flipper/flipper.go:165-167` |
| any other key | `serialterm.KeyToBytes(k)` → `session.Send(data)` | `generic/generic.go:104-111`, `flipper/flipper.go:171-178` |

`KeyToBytes` is documented in [serial-terminal.md §4](serial-terminal.md#4-keytobytes-keygo) — it handles text, arrows as ANSI sequences, and `ctrl+X` as ASCII control codes.

**Guard before `Send`.** Both implementations check `!m.session.Active() || m.disconnected` before forwarding keys (`generic/generic.go:104`, `flipper/flipper.go:171`). This matters after `RxClosedMsg`: the read loop has exited, but the user may still type — we silently drop those keystrokes rather than crash on a nil transport.

## 10. Error paths

Three error messages matter:

### `OpenErrMsg` — port open failed

Posted by `OpenSerialSession` (`serialterm/session.go:30-32`) when `OpenSerial` returns an error. The view stores it in `m.err` and renders the "FAILED TO OPEN PORT" banner in the body (`generic/generic.go:215-235`, `flipper/flipper.go:329-349`).

Both views call `serialterm.IsPermissionDenied(m.err.Error())` (`serialterm/errors.go:9-14`) and, on Linux only, append a dialout-group hint (`generic/generic.go:224-233`). The hint text lives in the view — if a third platform ever needs a different hint, that's where to add the branch.

### `RxClosedMsg` — disconnect or read error

Posted by `readLoop` (`serialterm/session.go:119-147`) when the transport errors (device unplugged, kernel reset) or when the read goroutine exits cleanly on `doneCh`.

Both views handle this identically: set `m.disconnected = true`, stash any error, and call `session.MarkDisconnected()` (`generic/generic.go:84-90`, `flipper/flipper.go:141-148`). `MarkDisconnected` (`serialterm/session.go:103-113`) closes the transport but **does not** close `doneCh` — the read goroutine has already returned, so closing it again would panic.

After disconnect, the view stays open (the user can read the final scrollback) until they press `esc`. The `DEVICE DISCONNECTED` banner is added to the header (`generic/generic.go:180-182`, `flipper/flipper.go:294-296`), and key sends are silently dropped.

### `TxDoneMsg` — write result

Posted after every `Session.Send` (`serialterm/session.go:50-59`). Both views store the error if non-nil but don't do anything dramatic with it — transient write errors are usually followed by `RxClosedMsg` anyway, which is the authoritative "we're done" signal.

---

Next: [adding-a-device.md](adding-a-device.md) for the step-by-step walkthrough, or [serial-terminal.md](serial-terminal.md) for the `serialterm` package reference.
