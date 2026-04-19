package serialterm

import tea "charm.land/bubbletea/v2"

// KeyToBytes maps a bubbletea key press to the byte sequence a serial device
// expects. Returns nil for keys that should be swallowed. Device views call
// this to translate user keystrokes before handing the bytes to Session.Send.
func KeyToBytes(k tea.KeyPressMsg) []byte {
	switch k.String() {
	case "enter":
		return []byte{'\r'}
	case "tab":
		return []byte{'\t'}
	case "backspace":
		return []byte{0x7f}
	case "up":
		return []byte("\x1b[A")
	case "down":
		return []byte("\x1b[B")
	case "right":
		return []byte("\x1b[C")
	case "left":
		return []byte("\x1b[D")
	case "home":
		return []byte("\x1b[H")
	case "end":
		return []byte("\x1b[F")
	case "pgup":
		return []byte("\x1b[5~")
	case "pgdown":
		return []byte("\x1b[6~")
	case "delete":
		return []byte("\x1b[3~")
	case "ctrl+@", "ctrl+space":
		return []byte{0x00}
	case "ctrl+a":
		return []byte{0x01}
	case "ctrl+b":
		return []byte{0x02}
	case "ctrl+c":
		return []byte{0x03}
	case "ctrl+d":
		return []byte{0x04}
	case "ctrl+e":
		return []byte{0x05}
	case "ctrl+f":
		return []byte{0x06}
	case "ctrl+g":
		return []byte{0x07}
	case "ctrl+h":
		return []byte{0x08}
	case "ctrl+i":
		return []byte{0x09}
	case "ctrl+j":
		return []byte{0x0a}
	case "ctrl+k":
		return []byte{0x0b}
	case "ctrl+n":
		return []byte{0x0e}
	case "ctrl+o":
		return []byte{0x0f}
	case "ctrl+p":
		return []byte{0x10}
	case "ctrl+u":
		return []byte{0x15}
	case "ctrl+w":
		return []byte{0x17}
	case "ctrl+x":
		return []byte{0x18}
	case "ctrl+y":
		return []byte{0x19}
	case "ctrl+z":
		return []byte{0x1a}
	case "space":
		return []byte{' '}
	}
	if len(k.Text) > 0 {
		return []byte(k.Text)
	}
	return nil
}
