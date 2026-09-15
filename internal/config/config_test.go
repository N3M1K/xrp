package config

import "testing"

func TestEffectiveTLD(t *testing.T) {
	c := &Config{
		TLD:         ".localhost",
		ProjectTLDs: map[string]string{"jellyfin": "media", "empty": ""},
	}

	if got := c.EffectiveTLD("jellyfin"); got != "media" {
		t.Errorf("custom TLD = %q, want media", got)
	}
	if got := c.EffectiveTLD("other"); got != "localhost" {
		t.Errorf("default TLD = %q, want localhost", got)
	}
	if got := c.EffectiveTLD("empty"); got != "localhost" {
		t.Errorf("empty override should fall back to default, got %q", got)
	}
}
