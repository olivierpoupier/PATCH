package flipper

import (
	"bufio"
	"strings"
)

// parseFlipperKV parses Flipper CLI "key : value" lines, tolerant of the
// prompt characters and padding the firmware uses ("hardware_ver       : 13").
func parseFlipperKV(raw []byte) map[string]string {
	out := make(map[string]string)
	sc := bufio.NewScanner(strings.NewReader(string(raw)))
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, ">:") {
			continue
		}
		idx := strings.Index(line, ":")
		if idx <= 0 {
			continue
		}
		key := strings.TrimSpace(line[:idx])
		val := strings.TrimSpace(line[idx+1:])
		if key == "" || val == "" {
			continue
		}
		out[key] = val
	}
	return out
}
