# Adding a New Device View

This guide walks through adding a dedicated device view to PATCH, start to finish. The worked example is a Raspberry Pi Pico (RP2040) view — a realistic next device, identifiable by Vendor ID `2e8a` and Product ID `0005`. Adjust the names as you follow along for whatever device you are actually adding.

Before starting, make sure you have skimmed:

- [device-architecture.md](device-architecture.md) — especially §3 (the `DeviceView` interface) and §5 (the `devices` registry).
- [serial-terminal.md](serial-terminal.md) — the `Session` lifecycle and the "single most important rule" about re-arming `WaitRx()`.

## 0. Decide: custom view or just a generic entry?

Not every device needs its own package. The matrix:

| You need… | Do this |
|-----------|---------|
| a friendly display name and a correct baud default | Add an entry to `devices/generic/init.go` — no new package needed |
| the same, plus on-connect queries, custom header fields, or extra keybindings | Create a new `devices/<name>/` package (this guide) |
| a fundamentally different UI (split panes, tabs, command palette) | New package, and consider whether the `DeviceView` interface needs extending — raise it first |

For the RP2040 we want to show the board's `BOOTSEL` state in the header, so it's a custom view.

## 1. Create the package skeleton

```
devices/rp2040/
├── init.go       # Registers the descriptor at process start
├── rp2040.go     # Model + Open/Update/View/Close/SetSize/Name
├── banner.go     # ASCII banner constant
└── parse.go      # (optional) key-value parser for on-connect query responses
```

The easiest starting point is to **copy `devices/generic/generic.go`** into `rp2040.go` and rename the package and factory. Do not copy from `flipper.go` unless you also need the capture-window pattern — it is extra machinery you might not need.

```bash
mkdir devices/rp2040
cp devices/generic/generic.go devices/rp2040/rp2040.go
cp devices/generic/banner.go   devices/rp2040/banner.go
```

Then in each file change `package generic` → `package rp2040`, rename the banner constant, and delete the helpers you don't need.

## 2. Register the descriptor

Create `devices/rp2040/init.go`:

```go
package rp2040

import (
    "github.com/olivierpoupier/patch/devices"
    "github.com/olivierpoupier/patch/tui"
)

func init() {
    devices.Register(devices.Descriptor{
        Key:   "rp2040",
        Name:  "Raspberry Pi Pico",
        Baud:  115200,
        Match: devices.MatchVIDPID("2e8a", "0005"),
        New: func(theme *tui.Theme) tui.DeviceView {
            return New(theme)
        },
    })
}
```

**Matcher precision.** The Pico uses VID `2e8a`; PID `0005` is the CDC device-mode interface (different from `0003` which is the picoprobe / bootloader). Match the exact `(VID, PID)` pair so you don't accidentally claim a picoprobe session that belongs to a different tool. If you want to claim every RP2040 variant, pass an empty PID — but *register this descriptor first* so the wildcard doesn't shadow a later specific match.

**Registration order note.** Within the `devices/` tree there is no explicit ordering guarantee beyond the blank-import order in `main.go`. If you need a new descriptor to win over an existing one, add its blank import *before* the other entry in `main.go:9-10`. For RP2040 this doesn't matter — no existing descriptor claims `2e8a`.

## 3. Implement `DeviceView`

Your copied `rp2040.go` already satisfies the interface. Now customise it.

### 3a. Add your state fields

Anything specific to this device goes on the `Model`. For RP2040 we'll add a `bootselMode` flag that a (hypothetical) `status\r\n` query will set:

```go
type Model struct {
    theme  *tui.Theme
    device tui.SerialDeviceInfo

    session    *serialterm.Session
    scrollback *serialterm.Scrollback
    scroll     components.ScrollableView

    // RP2040-specific
    bootselMode  bool
    capturing    bool
    captureBuf   []byte

    err          error
    disconnected bool
    width, height int
}
```

### 3b. Reset state in `Open`

`Open` is called once per modal opening. Zero out any per-device state that could leak across openings:

```go
func (m *Model) Open(info tui.SerialDeviceInfo) tea.Cmd {
    m.device = info
    if m.device.Baud == 0 { m.device.Baud = 115200 }
    if m.device.Name == "" { m.device.Name = "Raspberry Pi Pico" }
    m.scrollback.Clear()
    m.bootselMode = false
    m.captureBuf = m.captureBuf[:0]
    m.capturing = false
    m.err = nil
    m.disconnected = false
    return serialterm.OpenSerialSession(m.device.PortPath, m.device.Baud)
}
```

### 3c. Add on-connect behaviour (optional)

If your device has a meaningful startup query, follow Flipper's capture-window pattern (`devices/flipper/flipper.go:190-202`): start a capture window on `OpenedMsg`, redirect RX into a buffer while `capturing` is true, parse on a timed `captureEndMsg`, then resume normal scrollback. See `device-architecture.md` §6 for the mechanics.

If your device has no startup query, your `handleOpened` is one line:

```go
case serialterm.OpenedMsg:
    m.session.Install(msg)
    m.err = nil
    return m, m.session.WaitRx()
```

### 3d. Wire extra keybindings

Add them to the first `switch` in your `Update`'s `tea.KeyPressMsg` case, *before* the fallthrough to `KeyToBytes`:

```go
case tea.KeyPressMsg:
    switch msg.String() {
    case "esc":
        return m, func() tea.Msg { return tui.CloseDeviceMsg{} }
    case "ctrl+l":
        m.scrollback.Clear()
        return m, nil
    case "ctrl+b":
        // Your custom action (e.g., re-run status query).
        return m, m.refreshStatus()
    }
    if !m.session.Active() || m.disconnected {
        return m, nil
    }
    data := serialterm.KeyToBytes(msg)
    if len(data) == 0 {
        return m, nil
    }
    return m, m.session.Send(data)
```

Anything you handle in the first `switch` never reaches `KeyToBytes`, so it won't be sent to the device. Anything you don't handle goes to the device as a literal keystroke.

If your custom binding clashes with a control code users might reasonably want to send to the device (`ctrl+c` is a common example — don't rebind it), pick a different key. `ctrl+b`, `ctrl+r`, `ctrl+y` are typically safe because most serial-attached firmware doesn't use them.

### 3e. Render the header

Device-specific header content goes in `renderHeader`. Add a new row for your parsed fields:

```go
if m.bootselMode {
    rows = append(rows, t.HeaderVal.Render("  BOOTSEL mode"))
}
```

All styles come from `m.theme.Terminal.*` (see [device-architecture.md §8](device-architecture.md#8-the-terminal-theme)). No raw lipgloss, no hardcoded colors — the `patch-conventions` skill enforces this.

### 3f. Update the footer's keybinding hints

```go
bindings := []components.KeyBinding{
    {Keys: "esc",    Description: "close"},
    {Keys: "ctrl+l", Description: "clear"},
    {Keys: "ctrl+b", Description: "refresh status"},
    {Keys: "keys",   Description: "→ device"},
}
```

## 4. Wire the package into `main.go`

Add the blank import to `main.go` alongside `flipper` and `generic`:

```go
import (
    _ "github.com/olivierpoupier/patch/devices/flipper"
    _ "github.com/olivierpoupier/patch/devices/generic"
    _ "github.com/olivierpoupier/patch/devices/rp2040"
)
```

Without this line the `init()` function never runs and the descriptor is never registered. This is the single easiest step to forget.

## 5. Verify

```bash
go build ./...
go vet ./...
go test ./...
```

If the build fails with `imported and not used: "github.com/olivierpoupier/patch/tui"` in `init.go`, you forgot to reference `tui.DeviceView` in the factory. If `go vet` flags a shadowed variable in `handleRx`, you probably named a local the same as a struct field — rename.

Plug in the device and run `make run` (or whatever build target you use). Navigate to the USB tab, find the Pico, and confirm:

1. The Action column shows `⏎ serial` (this means `devices.HasCustom` saw your matcher — `usb/tab.go:495`). If it shows nothing, your matcher doesn't match — check the VID/PID your device actually enumerates as (`system_profiler SPUSBDataType` on macOS, `lsusb` on Linux).
2. Pressing `enter` opens the modal with your banner at the top and the correct port path in the header.
3. The scrollback shows device output. Pressing a normal letter key echoes it to the device (verify on the Pico side).
4. Pressing `esc` closes the modal and returns focus to the USB tab.
5. Unplugging the device shows the `DEVICE DISCONNECTED` banner instead of crashing.
6. Your custom keybinding (e.g., `ctrl+b`) performs its action.

## 6. Testing

Two kinds of tests are worth writing.

### Registry tests

Pattern from `devices/devices_test.go`. If your device has non-obvious matching rules (e.g., you want `2e8a:0005` to win over a hypothetical future `2e8a:*` wildcard), write a test that pins that ordering:

```go
func TestRP2040WinsOverGenericESPIfRegisteredFirst(t *testing.T) {
    withClean(t, func() {
        Register(Descriptor{Key: "rp2040", Match: MatchVIDPID("2e8a", "0005")})
        Register(Descriptor{Key: "generic", Match: nil}) // fallback
        got, _ := Resolve(tui.SerialDeviceInfo{VID: "2e8a", PID: "0005"})
        if got.Key != "rp2040" {
            t.Fatalf("expected rp2040, got %s", got.Key)
        }
    })
}
```

The `withClean` helper (`devices/devices_test.go:13-28`) snapshots and restores the global registry, so your test doesn't leak descriptors into other tests.

### Model unit tests

Bubbletea models are pure functions. Test the interesting paths — e.g., that `captureEndMsg` correctly populates `bootselMode`:

```go
func TestRP2040_CaptureEndParsesBootsel(t *testing.T) {
    m := New(tui.NewTheme())
    m.capturing = true
    m.captureBuf = []byte("status: bootsel=1\r\n")
    updated, _ := m.Update(captureEndMsg{})
    got := updated.(*Model)
    if !got.bootselMode {
        t.Error("expected bootselMode=true after parsing 'bootsel=1'")
    }
}
```

See `ARCHITECTURE.md §9` for the general Bubbletea testing patterns that apply across the project.

## 7. Checklist

Before merging:

- [ ] Descriptor has a non-nil `Match` and a unique `Key`
- [ ] `New` factory returns `tui.DeviceView`, not `*Model`
- [ ] `Open` resets all per-device state (re-openings reuse the factory output on a fresh `desc.New`, so this is defensive)
- [ ] `Close` is safe on a zero-valued view (delegates to `session.Close()`)
- [ ] Every `case serialterm.RxMsg` re-arms `session.WaitRx()`
- [ ] `case serialterm.RxClosedMsg` calls `session.MarkDisconnected()`, not `session.Close()`
- [ ] Key sends are guarded by `!m.session.Active() || m.disconnected`
- [ ] `esc` emits `tui.CloseDeviceMsg{}`
- [ ] All styles come from `m.theme.Terminal.*` — no raw `lipgloss.NewStyle()`, no hardcoded hex colors
- [ ] Blank-imported in `main.go`
- [ ] `go build ./...`, `go vet ./...`, and `go test ./...` all pass
- [ ] Manual verification against the physical device
