package finder

import (
	"testing"
)

func TestNormalizeMAC(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"aa:bb:cc:dd:ee:ff", "AA:BB:CC:DD:EE:FF"},
		{"AA:BB:CC:DD:EE:FF", "AA:BB:CC:DD:EE:FF"},
		{"aa-bb-cc-dd-ee-ff", "AA:BB:CC:DD:EE:FF"},
		{"aabbccddeeff", "AA:BB:CC:DD:EE:FF"},
		{"AABBCCDDEEFF", "AA:BB:CC:DD:EE:FF"},
		{"not-a-mac", "not-a-mac"},
		{"192.168.1.1", "192.168.1.1"},
		{"", ""},
		{"Samsung", "Samsung"},
	}

	for _, tc := range tests {
		got := normalizeMAC(tc.input)
		if got != tc.want {
			t.Errorf("normalizeMAC(%q) = %q, want %q", tc.input, got, tc.want)
		}
	}
}

func TestIsHexString(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"aabbccddeeff", true},
		{"AABBCCDDEEFF", true},
		{"0123456789ab", true},
		{"xyz", false},
		{"aabbccddeegg", false},
		{"", true}, // empty string has no non-hex chars
	}

	for _, tc := range tests {
		got := isHexString(tc.input)
		if got != tc.want {
			t.Errorf("isHexString(%q) = %v, want %v", tc.input, got, tc.want)
		}
	}
}

func TestPrepareSearchPattern(t *testing.T) {
	tests := []struct {
		query string
		opts  SearchOptions
		want  string
	}{
		{"samsung", SearchOptions{ExactMatch: false}, "%samsung%"},
		{"samsung", SearchOptions{ExactMatch: true}, "samsung"},
		// MAC normalization happens before wildcard wrapping
		{"aabbccddeeff", SearchOptions{ExactMatch: false}, "%AA:BB:CC:DD:EE:FF%"},
		{"aa:bb:cc:dd:ee:ff", SearchOptions{ExactMatch: true}, "AA:BB:CC:DD:EE:FF"},
	}

	for _, tc := range tests {
		got := prepareSearchPattern(tc.query, tc.opts)
		if got != tc.want {
			t.Errorf("prepareSearchPattern(%q, %+v) = %q, want %q", tc.query, tc.opts, got, tc.want)
		}
	}
}

func TestDefaultSearchOptions(t *testing.T) {
	opts := DefaultSearchOptions()
	if opts.ExactMatch {
		t.Error("default ExactMatch should be false")
	}
	if opts.CaseSensitive {
		t.Error("default CaseSensitive should be false")
	}
	if opts.Limit != 100 {
		t.Errorf("default Limit = %d, want 100", opts.Limit)
	}
}

func TestGlobalFindEmptyQuery(t *testing.T) {
	_, err := GlobalFind("", DefaultSearchOptions())
	if err == nil {
		t.Error("expected error for empty query, got nil")
	}
}
