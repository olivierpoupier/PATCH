package serialterm

// bannerFlipper is a stylised "FLIPPER" marker. Deliberately small so it fits
// on narrow terminals.
const bannerFlipper = `   ___ _ _
  |  _| (_)_ __ _ __   ___ _ __
  | |_| | | '_ \| '_ \ / _ \ '__|
  |  _| | | |_) | |_) |  __/ |
  |_| |_|_| .__/| .__/ \___|_|
          |_|   |_|`

const bannerESP32 = `  ___ ___ ___ _______
 | __/ __| _ \__ /_  )
 | _|\__ \  _/|_ \/ /
 |___|___/_|  |___/___|`

const bannerArduino = `    _             _       _
   / \   _ __ __| |_   _(_)_ __   ___
  / _ \ | '__/ _` + "`" + ` | | | | | '_ \ / _ \
 / ___ \| | | (_| | |_| | | | | | (_) |
/_/   \_\_|  \__,_|\__,_|_|_| |_|\___/`

const bannerPluto = ` ___ _ _ _ _____ ___
| _ \ | | | |_   _/ _ \
|  _/ |_| | | | || (_) |
|_|  \___/  |_| \___/`

const bannerGeneric = ` ___ ___ ___ ___   _   _
/ __| __| _ \_ _| /_\ | |
\__ \ _||   /| | / _ \| |__
|___/___|_|_\___/_/ \_\____|`

// bannerFor returns the ASCII banner for a profile key, falling back to the
// generic banner when no match is found.
func bannerFor(key string) string {
	switch key {
	case profileFlipperZero:
		return bannerFlipper
	case profileESP32:
		return bannerESP32
	case profileArduino:
		return bannerArduino
	case profilePlutoSDR:
		return bannerPluto
	default:
		return bannerGeneric
	}
}
