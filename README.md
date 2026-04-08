# PATCH
Peripheral Access Terminal for Connected Hardware

## Quick Start

```sh
# Install Go dependencies
make deps

# First time on macOS only — installs the haraltd Bluetooth daemon
make setup

# Build and run (handles haraltd automatically on macOS)
make run
```

### Platform Requirements

| Platform | Requirement |
|----------|-------------|
| Linux | BlueZ (`sudo apt install bluez` / `sudo pacman -S bluez`) |
| macOS | haraltd daemon (installed via `make setup`, requires `gh` CLI) |

Run `make help` to see all available commands.

## Keybindings

### Tab Bar (default)

| Key | Action |
|-----|--------|
| `Tab` / `Right` / `l` | Next tab |
| `Shift+Tab` / `Left` / `h` | Previous tab |
| `1`, `2` | Jump to tab by number |
| `Enter` | Focus into the selected tab |
| `q` / `Ctrl+C` | Quit |

### Bluetooth Tab (when focused)

| Key | Action |
|-----|--------|
| `Up` / `k` | Move cursor up |
| `Down` / `j` | Move cursor down |
| `Enter` | Connect/disconnect selected device |
| `p` | Toggle Bluetooth power |
| `Esc` | Return to tab bar |
