---
name: patch-add-device-view
description: Walk a developer through adding a new per-device view (the modal that opens when the user presses enter on a USB serial device) to the PATCH TUI. Triggers whenever the user wants to add, implement, or scaffold custom behavior for a USB serial device — phrases like "add support for [X]", "implement an [X] view", "make a device view for [X]", "build a terminal for [X]", "add a Flipper/ESP32/Arduino/Pluto/RP2040 view", "PATCH should recognise [X]", or any mention of extending PATCH with a new peripheral modal. Use this skill even when the request sounds simple — the interview catches cross-cutting implications, the phased plan prevents over-scoping, and device-specific conventions in `devices/` and `serialterm/` are easy to miss. Covers the full flow: scope decision (friendly-name entry vs. custom package), interview (layout, colors, feature set), complexity assessment with POC + phased rollout for hard features, warnings when a change ripples into shared code, and the implementation itself.
---

# Device View Workflow for PATCH

This skill guides a developer through adding a new serial device view to PATCH end-to-end: scope decision, interview, complexity assessment, phased plan when needed, cross-cutting warnings, and implementation. The goal is to get from "I want to support device X" to working code without skipping the questions that matter.

Apply the `patch-conventions` and `patch-components` skills throughout — they auto-apply, but explicitly verify their rules at each step.

## Step 0: Read the docs first

Before asking the developer anything, read the authoritative references so your advice matches the current code:

- `docs/device-architecture.md` — end-to-end flow (USB tab → registry → `DeviceView` → close), the six-method interface, the registry semantics, and the two existing implementations
- `docs/adding-a-device.md` — the step-by-step walkthrough this skill orchestrates; don't paraphrase it, follow it
- `docs/serial-terminal.md` — the `serialterm` package reference (`Session`, `Scrollback`, messages, keys)
- `ARCHITECTURE.md` — project-wide conventions (§9 on testing is the one most relevant here)

Also glance at these to see *current* API shapes in case they drifted since the docs were written:

- `tui/deviceview.go` — the interface
- `tui/messages.go` — `SerialDeviceInfo`, `OpenDeviceMsg`, `CloseDeviceMsg`
- `tui/theme.go` — `TerminalTheme` (the palette device modals use)
- `devices/devices.go` — registry
- `devices/flipper/flipper.go` and `devices/generic/generic.go` — the two reference implementations

## Step 1: Scope decision — friendly-name entry, or custom package?

Ask the developer which of these matches their goal:

1. **"I just want PATCH to show a nicer name and baud default when this device is plugged in."** → Add one row to the `entries` slice in `devices/generic/init.go`. No new package. Usually a single-commit change — but verify the enumerator-name gotcha below before promising the developer it's a one-liner.
2. **"I want custom header fields / on-connect queries / extra keybindings / different layout."** → Proceed to the interview. The output is a new `devices/<name>/` package.

If they're unsure, ask: *"Would the generic serial terminal be enough for day-to-day use, or is there device-specific information or interaction that only this device has?"* Generic + friendly name is the right default for most chips.

### The enumerator-name gotcha

Before recommending the one-line entry, look at `usb/tab.go:resolveSerialPort` (around lines 444–467). The displayed name is built like this:

```go
info.Name = dev.Name                     // USB enumerator string (system_profiler / lsusb)
if desc, ok := devices.Resolve(info); ok {
    info.Baud = desc.Baud
    info.ProfileKey = desc.Key
    if info.Name == "" {                 // descriptor name only wins if enumerator name was empty
        info.Name = best.Product
    }
    if info.Name == "" {
        info.Name = desc.Name
    }
}
```

The descriptor's `Name` only wins when the OS enumerator returned an empty name. For most boards that's fine — they enumerate with a useful product string. But several common families enumerate as something generic and uninformative (CircuitPython boards typically show "USB Serial Device", some CDC bridges show just the chip name like "USB-Serial Controller", etc.). For those, adding the descriptor entry alone will **not** change the displayed header — the enumerator's generic name keeps winning.

So before promising a one-liner:

- Ask the developer what the device currently shows in PATCH's USB tab. If it's already the device's product name, the one-line entry is genuinely sufficient for the rename.
- If it's a generic placeholder ("USB Serial Device", "USB-Serial Controller", etc.), the fix is two parts: the descriptor entry **plus** an extension to `resolveSerialPort` that treats those generic strings as effectively-empty so the descriptor name wins. Frame this as a small shared-code change (Step 4 territory) and confirm the developer wants it before bundling.

If they're not sure what the device currently shows, that's a "go look first" moment — don't guess.

## Step 2: Interview

Work through these with the developer. Don't just read the list — respond to what they say, probe, and follow up. If an answer triggers a cross-cutting concern (Step 4), raise it immediately instead of waiting.

### Identity

1. **Display name** — what should PATCH show in the header? (e.g., "Flipper Zero", "Raspberry Pi Pico")
2. **VID and PID** — as lowercase hex, no `0x` prefix. Help them find it:
   - macOS: `system_profiler SPUSBDataType | grep -iE 'product id|vendor id'`
   - Linux: `lsusb` (the format is `VID:PID`)
3. **Matcher precision** — exact `(VID, PID)` pair, or VID-only wildcard (empty PID matches any product on that vendor)? Default to exact; only go wildcard if they genuinely want to claim every product from this vendor.
4. **Default baud** — almost always 115200 for USB-CDC. If the device uses a non-standard rate, confirm and mention that it goes in the `Descriptor.Baud` field.
5. **Key** — short snake_case identifier for logs (e.g., `flipper_zero`, `rp2040`). Must be unique across the registry.

### Layout & styling

The palette is fixed: `theme.Terminal` (amber-on-dark) is used by every device modal. Do not invent a new one unless the developer has a specific reason — that's cross-cutting (see Step 4). The styling choice inside a view is *which* pre-built style to apply where.

1. **Banner** — reuse `devices/generic/banner.go` verbatim, or supply custom ASCII art? If custom, ask whether they have the art ready or want a simple text-based placeholder for now. Keep the banner narrow enough to fit in 50 columns.
2. **Header field order** — the defaults are `Device / Port / Baud` on row 1, `VID:PID / Serial` on row 2. What else goes in the header, and in what order?
3. **Dynamic header fields** — any fields populated from device output (e.g., firmware version, battery %, mode)? If yes, that's the "capture window" pattern from Flipper — note it for Step 3 complexity assessment.
4. **Style emphasis** — what should be rendered with `HeaderVal` (bold warm) vs. `HeaderKey` (dim amber) vs. `DisconnectBanner` (red)? Defaults are fine unless they call out otherwise.
5. **Footer keybindings** — any additions to the standard `esc close / ctrl+l clear / keys → device` line?

### Core behavior

1. **On-connect behavior** — silent (like `generic`), or send a query to populate the header (like `flipper`)? If queries: which commands, what line terminators (`\r`, `\r\n`, `\n`), and roughly how long does the device take to respond?
2. **Parsing** — if there are dynamic header fields, how are they shaped in the device output? `key: value` per line (Flipper style), CSV, JSON, binary frames, something else?
3. **Custom keybindings** — any keys beyond `esc` (close) and `ctrl+l` (clear)? For each:
   - Which key? (Avoid `ctrl+c` — forwarding to device is load-bearing.)
   - What does it do?
   - Is it only meaningful while session is active, or also while disconnected?

   **Check `serialterm/keys.go` first.** `KeyToBytes` already maps every common control character: `ctrl+a`–`ctrl+k`, `ctrl+n`–`ctrl+p`, `ctrl+u`/`ctrl+w`/`ctrl+x`/`ctrl+y`/`ctrl+z`, plus arrows, home/end, page up/down, delete, backspace, tab, enter. So a binding like *"ctrl+b sends 0x02 (MicroPython soft reset)"* is **already happening today** — it just hits the device with no UI affordance.

   That means a view-local intercept is usually not duplication: it's about *intent*. Sending the byte is identical, but the local handler lets you (a) show the action in the footer hint so the user knows it exists, (b) attach logging or a brief on-screen confirmation, (c) gate on connection state (the raw `KeyToBytes` path doesn't), and (d) document the device-specific meaning of an otherwise-anonymous control char. Mention this to the developer when the chosen key is already in `keys.go` — they should know they're choosing presentation/clarity, not adding new wire behavior.
4. **Byte handling** — plain UTF-8 text (default, via `Scrollback`), or a binary framing protocol? If binary, flag as cross-cutting immediately — `Scrollback` is text-mode and would need extension or replacement.
5. **Disconnect UX** — is the `DEVICE DISCONNECTED` banner enough, or does the device need something special when it drops (e.g., "press ctrl+r to reconnect")?

### Advanced features

Ask about each of these explicitly — developers often don't volunteer them:

1. **Stateful mode switching** — does the device have distinct modes (e.g., bootloader vs. app, CLI vs. binary) where the view should behave differently?
2. **Interactive input** — do they want command history, autocompletion, or cooked-mode line editing? Today, every key goes straight to the device.
3. **Sub-views** — terminal + status panel? Split-screen? Multiple tabs within the modal? This likely requires extending the `DeviceView` interface or embedding a child model.
4. **Persistence across openings** — today every `desc.New(theme)` creates a fresh view and nothing survives `Close`. If they need scrollback history or login state to persist, flag it.
5. **File transfer or firmware flash** — needs framing, progress UI, and likely a protocol layer on top of `Session.Send`.
6. **Real-time graphics** — charts, meters, visualizations beyond text. Not currently supported; would need custom rendering inside the body area.
7. **Non-serial transport** — HID, TCP, USB bulk, Bluetooth serial. Extends `serialterm.Transport` or introduces a parallel transport package.

## Step 3: Assess complexity and decide on phasing

Classify the total scope based on the answers:

| Complexity | Characteristics | Approach |
|------------|-----------------|----------|
| **Simple** | Plain text terminal + light header customization + 0–1 custom keybindings | Implement in one pass; no plan file needed |
| **Moderate** | On-connect queries + key-value parsing + dynamic header + 1–2 custom keybindings | Implement in one pass; write a short plan with checkpoints |
| **Complex** | Stateful mode switching, 3+ custom keybindings, custom disconnect handling, or multiple parsed sources | POC + phases (see below) |
| **Very complex** | Sub-views, file transfer, firmware flash, real-time graphics, persistent state | Write a design doc first; POC may still be worth it but confirm feasibility before scoping phases |
| **Infeasible without foundational work** | New transport layer, binary protocol needing custom framing, GUI-style widgets | Say so. Propose the upstream work (new `Transport`, new parser) as a separate task before the device view |

For **Complex** and above, break into phases and write them to a plan file at `/Users/<user>/.claude/plans/`:

- **Phase 1 — POC.** Minimal package that registers the descriptor, matches the VID/PID, and opens a plain generic-style terminal. This proves the device works with PATCH's I/O before any custom work. Verification: `⏎ serial` appears in the USB tab row; pressing enter opens a modal; keys round-trip; unplug shows `DEVICE DISCONNECTED`.
- **Phase 2 — Static customization.** Banner, header field labels, footer keybindings text. No dynamic behavior. Verification: visual review.
- **Phase 3 — On-connect queries.** Implement the `capturing` / `captureBuf` / `captureEndMsg` pattern (see `devices/flipper/flipper.go:190-208`). Verification: header shows parsed fields within ~1s of opening.
- **Phase 4 — Custom keybindings.** Add each key one at a time; test that existing keys (`esc`, `ctrl+l`, arrow keys passed to the device) still work. Verification: each binding's documented behavior.
- **Phase 5 — Parsing refinements.** Handle edge cases, ANSI in responses, partial lines, error responses from the device.
- **Phase 6+ — Advanced features**, one increment each.

Confirm the plan with the developer before writing any code. Each phase should be independently mergeable.

## Step 4: Cross-cutting concerns — warn before implementing

These changes affect more than the new device package. Raise them as soon as the interview surfaces them and confirm the developer understands the blast radius. Do **not** silently extend shared code on behalf of a single device.

| Change | Propagates to | Impact |
|--------|---------------|--------|
| New case in `serialterm/keys.go` | Every device view uses `KeyToBytes` | Usually additive and safe, but check the key isn't one a device needs passed through as raw text |
| Modifying `serialterm/Scrollback` | Every device's terminal behavior | Must add / update cases in `ansi_test.go` |
| New `Transport` implementation | None directly — `Session` is transport-agnostic | Stays contained *if* it satisfies `io.ReadWriteCloser` and `Session.Close` works correctly |
| Changing the `DeviceView` interface | All existing device views must be updated | Only consider if a feature genuinely needs it (e.g., sub-views). Change the interface, update Flipper and Generic, then add the new view |
| Overriding `ctrl+c` locally in the view | Users lose the "esc then ctrl+c to quit" path | Don't. Pick a different key |
| Adding a second `TerminalTheme` | `tui/theme.go` gains a selector; every device view must pick one | Consider whether the existing palette genuinely can't render what's needed first |
| Persistent state across openings | Needs a new registry keyed by `ProfileKey` or similar; `app.go` lifecycle must change | Rare. Confirm it's truly required before designing |
| New top-level package | Project layout changes | Don't. Device views are subpackages of `devices/` |
| Modifying `tui.SerialDeviceInfo` | USB tab enumeration, all device views, `app.go` routing | Only for genuinely missing fields; avoid adding view-specific data here |
| New cross-tab message in `tui/messages.go` | Broadcast to every tab | Consider whether tab-internal routing via unexported types is sufficient first |

If the developer says "yes I need this," write the cross-cutting change as its own phase *before* the phases that depend on it. Never bundle a shared-code change with a device-specific feature in the same commit.

## Step 5: Implement

Follow `docs/adding-a-device.md` for the concrete steps. The skill's job here is to keep the developer honest about each guardrail. For every file you generate or edit:

- **Receiver names**: `m` for Model, `t` for TerminalTheme. Never `self` or `this`.
- **Styles**: all from `m.theme.Terminal.*`. Zero `lipgloss.NewStyle()` in view code. Zero hardcoded hex colors.
- **Commands**: every `tea.Cmd` constructor lives in `devices/<name>/` — either alongside the model or in a sibling file. Don't put commands in `tab.go`-style files; the device package is small enough.
- **`WaitRx()` re-arm**: after every `case serialterm.RxMsg:` in `Update`, return `m.session.WaitRx()` as the command. Forgetting this is the single most common bug.
- **Disconnect path**: `case serialterm.RxClosedMsg:` calls `session.MarkDisconnected()`, *not* `session.Close()`. Read `docs/serial-terminal.md §1` if unsure.
- **Key guard**: `if !m.session.Active() || m.disconnected { return m, nil }` before `session.Send`.
- **Blank import in `main.go`**: adding the `init()` descriptor is useless without `_ "github.com/olivierpoupier/patch/devices/<name>"` in `main.go`. This is the easiest step to forget.
- **`Close()` safety**: must be safe on a freshly-constructed view (before `Open` fires). Delegating to `session.Close()` handles this automatically.

When the developer asks you to copy from an existing view, copy from **Generic** unless they have on-connect queries. Flipper's capture-window machinery is ~80 lines of extra code that do nothing for a device without on-connect queries.

## Step 6: Verify

After each phase:

1. `go build ./...`
2. `go vet ./...`
3. `go test ./...` — at minimum the registry tests in `devices/devices_test.go` still pass; ideally add tests for the new device's parsing or state transitions per `ARCHITECTURE.md §9`
4. Manual test against the physical device:
   - `⏎ serial` affordance visible in the USB tab row
   - Modal opens on `enter`
   - Header renders as designed
   - Custom keybindings work; `esc`, `ctrl+l`, arrow keys still work
   - Unplug shows `DEVICE DISCONNECTED`
   - Reconnect: close modal, open again, everything works
   - Permission-denied path on Linux (if the developer is on Linux) shows the dialout-group hint

The developer runs the manual test — you can't. Be explicit about what they should check.

## Step 7: Final checklist

Before marking a phase done, verify:

- [ ] Descriptor registered with non-nil `Match` (unless this phase *is* the fallback, which is rare)
- [ ] `Key` is unique across the registry
- [ ] `New` factory returns `tui.DeviceView`, not `*Model`
- [ ] `Open` resets all per-device state (defensive — views are fresh per opening, but this protects against misuse)
- [ ] `Close` delegates to `session.Close()` and is safe on a zero-valued view
- [ ] Every `case serialterm.RxMsg:` ends with `return m, m.session.WaitRx()`
- [ ] `case serialterm.RxClosedMsg:` calls `session.MarkDisconnected()`
- [ ] Key sends are guarded by `!m.session.Active() || m.disconnected`
- [ ] `esc` emits `tui.CloseDeviceMsg{}`
- [ ] All styles come from `m.theme.Terminal.*`
- [ ] Blank import in `main.go`
- [ ] Build / vet / test all pass
- [ ] Manual verification against the device
- [ ] Any cross-cutting change has its own commit and its own test coverage

## Pitfalls to flag proactively

- **"I'll just mock the serial library in tests."** Don't mock `go.bug.st/serial`. Write a minimal `Transport` stub — it's only `io.ReadWriteCloser`.
- **"I'll spawn a goroutine for my background parsing."** Don't. `tea.Cmd` / `tea.Batch` / `tea.Tick` cover every async case. The one already-spawned goroutine (the read loop) is owned by `Session`; don't add more.
- **"I need to call `Scrollback.Write` from my goroutine."** `Scrollback` isn't thread-safe. All mutations must happen in `Update`, which is serialised by the Bubbletea runtime.
- **"I'll track state via package-level variables."** No shared mutable state. The registry is the only package-level state in `devices/`, and it's lock-protected and written-at-init-only.
- **"My device needs different colors."** Try mapping to the existing `TerminalTheme` styles first. A second palette is a big change. If you truly need it, open it as its own phase.
- **"Let me add a keybinding to toggle `ctrl+c` forwarding."** Don't. Users need a single, predictable quit path across every device view.
