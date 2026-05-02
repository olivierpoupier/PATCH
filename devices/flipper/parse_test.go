package flipper

import "testing"

func TestParseFlipperKV(t *testing.T) {
	input := []byte(
		"\x1b[31;1m>:\x1b[0m device_info\r\n" +
			"hardware_ver        : 13\r\n" +
			"firmware_version    : 0.95.1\r\n" +
			"charge.level        : 87\r\n" +
			"\x1b[31;1m>:\x1b[0m ")
	got := parseFlipperKV(input)
	want := map[string]string{
		"hardware_ver":     "13",
		"firmware_version": "0.95.1",
		"charge.level":     "87",
	}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("parseFlipperKV[%q] = %q, want %q", k, got[k], v)
		}
	}
}

func TestParseFlipperKVIgnoresPromptAndBlankLines(t *testing.T) {
	input := []byte(
		"\r\n" +
			">: device_info\r\n" +
			"\r\n" +
			"name : banana\r\n" +
			">: \r\n")
	got := parseFlipperKV(input)
	if got["name"] != "banana" {
		t.Errorf("name: got %q, want %q", got["name"], "banana")
	}
	if _, ok := got[""]; ok {
		t.Errorf("empty-key entry leaked through: %q", got)
	}
}

func TestParseFlipperKVHandlesPaddedValues(t *testing.T) {
	input := []byte("hardware_ver        : 13\n")
	got := parseFlipperKV(input)
	if got["hardware_ver"] != "13" {
		t.Errorf("padded value: got %q, want %q", got["hardware_ver"], "13")
	}
}
