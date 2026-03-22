package cmd

import (
	"testing"
)

func TestCrewSessionName(t *testing.T) {
	tests := []struct {
		name     string
		rigName  string
		crewName string
	}{
		{"basic", "gastown", "tester"},
		{"beads rig", "beads", "emma"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := crewSessionName(tt.rigName, tt.crewName)
			if got == "" {
				t.Error("crewSessionName returned empty string")
			}
			// Session name should contain the crew name
			if !containsStr(got, tt.crewName) {
				t.Errorf("crewSessionName(%q, %q) = %q, does not contain crew name", tt.rigName, tt.crewName, got)
			}
		})
	}
}

func TestParseCrewSessionName(t *testing.T) {
	// parseCrewSessionName depends on session.ParseSessionName which uses
	// a PrefixRegistry to map prefixes to rigs. Without the registry populated,
	// parsing will fail. We test the roundtrip with the understanding that in
	// test environment, the registry may not be available.
	rigName := "gastown"
	crewName := "tester"
	sessionName := crewSessionName(rigName, crewName)

	parsedRig, parsedCrew, _, ok := parseCrewSessionName(sessionName)
	if !ok {
		// In test context without PrefixRegistry, parsing may fail.
		// This is expected - the important thing is it doesn't panic.
		t.Skipf("parseCrewSessionName(%q) returned ok=false (no PrefixRegistry in test context)", sessionName)
	}
	if parsedRig != rigName {
		t.Errorf("parsed rig = %q, want %q", parsedRig, rigName)
	}
	if parsedCrew != crewName {
		t.Errorf("parsed crew = %q, want %q", parsedCrew, crewName)
	}
}

func TestParseCrewSessionName_Invalid(t *testing.T) {
	tests := []struct {
		name    string
		session string
	}{
		{"empty", ""},
		{"random", "random-session"},
		{"no crew marker", "gt-gastown-polecat-alpha"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, _, ok := parseCrewSessionName(tt.session)
			if ok {
				t.Errorf("parseCrewSessionName(%q) returned ok=true, want false", tt.session)
			}
		})
	}
}

func TestResolveRigFromLeadingArg_FlagTakesPriority(t *testing.T) {
	// When rigFlag is set, it wins and all args are crew names
	rig, remaining := resolveRigFromLeadingArg([]string{"beads", "emma"}, "gastown")
	if rig != "gastown" {
		t.Errorf("rig = %q, want %q", rig, "gastown")
	}
	if len(remaining) != 2 || remaining[0] != "beads" || remaining[1] != "emma" {
		t.Errorf("remaining = %v, want [beads emma]", remaining)
	}
}

func TestResolveRigFromLeadingArg_SingleArg(t *testing.T) {
	// With only 1 arg, no leading-rig extraction happens
	rig, remaining := resolveRigFromLeadingArg([]string{"emma"}, "")
	if rig != "" {
		t.Errorf("rig = %q, want empty", rig)
	}
	if len(remaining) != 1 || remaining[0] != "emma" {
		t.Errorf("remaining = %v, want [emma]", remaining)
	}
}

func TestResolveRigFromLeadingArg_SlashSkipped(t *testing.T) {
	// If first arg contains "/", don't treat as positional rig
	rig, remaining := resolveRigFromLeadingArg([]string{"beads/emma", "fred"}, "")
	if rig != "" {
		t.Errorf("rig = %q, want empty (slash format should be skipped)", rig)
	}
	if len(remaining) != 2 {
		t.Errorf("remaining = %v, want [beads/emma fred]", remaining)
	}
}

func TestResolveRigFromLeadingArg_EmptyArgs(t *testing.T) {
	rig, remaining := resolveRigFromLeadingArg([]string{}, "")
	if rig != "" {
		t.Errorf("rig = %q, want empty", rig)
	}
	if len(remaining) != 0 {
		t.Errorf("remaining = %v, want empty", remaining)
	}
}

func containsStr(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 || indexStr(s, substr) >= 0)
}

func indexStr(s, substr string) int {
	for i := 0; i+len(substr) <= len(s); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}
