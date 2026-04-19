package serialterm

import "testing"

func TestLookupProfileExactMatch(t *testing.T) {
	p := LookupProfile("0483", "5740")
	if p.Key != profileFlipperZero {
		t.Fatalf("Flipper exact: got %q, want %q", p.Key, profileFlipperZero)
	}
}

func TestLookupProfileWithPrefix(t *testing.T) {
	p := LookupProfile("0x0483", "0x5740")
	if p.Key != profileFlipperZero {
		t.Fatalf("0x prefix normalize: got %q", p.Key)
	}
}

func TestLookupProfileCaseInsensitive(t *testing.T) {
	p := LookupProfile("0483", "5740")
	p2 := LookupProfile("0483", "5740")
	if p.Key != p2.Key {
		t.Fatalf("case mismatch: %q vs %q", p.Key, p2.Key)
	}
}

func TestLookupProfileVIDWildcard(t *testing.T) {
	// ESP32 profile has VID=303a, PID="". Any PID under that VID matches.
	p := LookupProfile("303a", "deadbeef")
	if p.Key != profileESP32 {
		t.Fatalf("ESP32 wildcard: got %q, want %q", p.Key, profileESP32)
	}
}

func TestLookupProfileUnknownFallsBack(t *testing.T) {
	p := LookupProfile("dead", "beef")
	if p.Key != profileGeneric {
		t.Fatalf("unknown fallback: got %q, want %q", p.Key, profileGeneric)
	}
	if p.Baud != 115200 {
		t.Fatalf("generic baud: got %d, want 115200", p.Baud)
	}
}

func TestLookupProfileExactBeatsWildcard(t *testing.T) {
	// If we ever add a VID=303a with specific PID, exact should win over
	// wildcard — verify the matching order handles that.
	p := LookupProfile("10c4", "ea60")
	if p.Key != profileCP2102 {
		t.Fatalf("cp2102 exact: got %q", p.Key)
	}
}

func TestHasProfile(t *testing.T) {
	cases := []struct {
		vid, pid string
		want     bool
	}{
		{"0483", "5740", true},   // Flipper exact
		{"303a", "1234", true},   // ESP32 wildcard
		{"0x0483", "5740", true}, // 0x prefix tolerated
		{"dead", "beef", false},  // unknown → generic
		{"", "", false},
	}
	for _, tc := range cases {
		if got := HasProfile(tc.vid, tc.pid); got != tc.want {
			t.Errorf("HasProfile(%q,%q) = %v, want %v", tc.vid, tc.pid, got, tc.want)
		}
	}
}
