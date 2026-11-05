package config

import "testing"

func TestParseAllowedUserIDs(t *testing.T) {
	m, err := parseAllowedUserIDs("111, 222")
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := m[111]; !ok {
		t.Fatal("missing 111")
	}
	if _, ok := m[222]; !ok {
		t.Fatal("missing 222")
	}

	cfg := &Config{AllowedUserIDs: m}
	if !cfg.IsAllowed(111) || cfg.IsAllowed(999) {
		t.Fatal("allowlist check failed")
	}

	open := &Config{AllowedUserIDs: map[int64]struct{}{}}
	if !open.IsAllowed(999) {
		t.Fatal("empty allowlist should allow all")
	}
}
