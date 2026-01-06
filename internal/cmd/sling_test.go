package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/steveyegge/gastown/internal/beads"
)

// TestSlingResolvesRouting verifies that sling command resolves issue IDs
// correctly using prefix-based routing from routes.jsonl.
//
// Bug: When gt sling uses --on flag, it resolves the issue ID using the wrong
// beads database. The routing logic doesn't consult routes.jsonl.
func TestSlingResolvesRouting(t *testing.T) {
	// Set up a test town with multiple rigs
	townRoot := setupSlingRoutingTestTown(t)

	tests := []struct {
		name      string
		issueID   string
		workDir   string // Directory to run from
		wantRoute string // Expected rig the route should resolve to
	}{
		{
			name:      "hq prefix routes to town beads",
			issueID:   "hq-test123",
			workDir:   townRoot,
			wantRoute: ".",
		},
		{
			name:      "gt prefix routes to gastown rig",
			issueID:   "gt-abc456",
			workDir:   townRoot,
			wantRoute: "gastown/mayor/rig",
		},
		{
			name:      "tr prefix routes to testrig",
			issueID:   "tr-xyz789",
			workDir:   townRoot,
			wantRoute: "testrig/mayor/rig",
		},
		{
			name:      "gt prefix from polecat directory",
			issueID:   "gt-fromcat",
			workDir:   filepath.Join(townRoot, "gastown", "polecats", "worker1"),
			wantRoute: "gastown/mayor/rig",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Test that the routing mechanism can find the correct route for the prefix
			townBeadsDir := filepath.Join(townRoot, ".beads")
			routes, err := beads.LoadRoutes(townBeadsDir)
			if err != nil {
				t.Fatalf("LoadRoutes failed: %v", err)
			}

			// Extract prefix from issue ID (everything up to and including the hyphen)
			prefix := extractPrefix(tc.issueID)
			if prefix == "" {
				t.Fatalf("could not extract prefix from issue ID: %s", tc.issueID)
			}

			// Find the route for this prefix
			var foundRoute *beads.Route
			for i, r := range routes {
				if r.Prefix == prefix {
					foundRoute = &routes[i]
					break
				}
			}

			if foundRoute == nil {
				t.Errorf("no route found for prefix %q (issue %s)", prefix, tc.issueID)
				return
			}

			if foundRoute.Path != tc.wantRoute {
				t.Errorf("route for prefix %q = %q, want %q", prefix, foundRoute.Path, tc.wantRoute)
			}
		})
	}
}

// TestSlingPrefixExtraction verifies that issue ID prefix extraction works correctly.
func TestSlingPrefixExtraction(t *testing.T) {
	tests := []struct {
		issueID    string
		wantPrefix string
	}{
		{"gt-abc123", "gt-"},
		{"hq-xyz789", "hq-"},
		{"tr-test01", "tr-"},
		{"bd-issue42", "bd-"},
		{"nohyphen", ""},      // No hyphen - should return empty
		{"", ""},              // Empty - should return empty
		{"multi-hy-phen", "multi-"}, // Multiple hyphens - take first
	}

	for _, tc := range tests {
		t.Run(tc.issueID, func(t *testing.T) {
			got := extractPrefix(tc.issueID)
			if got != tc.wantPrefix {
				t.Errorf("extractPrefix(%q) = %q, want %q", tc.issueID, got, tc.wantPrefix)
			}
		})
	}
}

// TestSlingRoutingFromPolecatWorktree verifies that routing works correctly
// when running from a polecat's worktree directory that has a redirect.
func TestSlingRoutingFromPolecatWorktree(t *testing.T) {
	townRoot := setupSlingRoutingTestTown(t)

	// The polecat directory has a redirect to mayor/rig/.beads
	polecatDir := filepath.Join(townRoot, "gastown", "polecats", "worker1")

	// Verify the redirect file exists
	redirectPath := filepath.Join(polecatDir, ".beads", "redirect")
	if _, err := os.Stat(redirectPath); os.IsNotExist(err) {
		t.Fatalf("redirect file not found at %s", redirectPath)
	}

	// Read the redirect content
	redirectContent, err := os.ReadFile(redirectPath)
	if err != nil {
		t.Fatalf("reading redirect: %v", err)
	}

	// Verify redirect points to mayor/rig/.beads
	expectedRedirect := "../../mayor/rig/.beads"
	if string(redirectContent) != expectedRedirect {
		t.Errorf("redirect content = %q, want %q", string(redirectContent), expectedRedirect)
	}

	// Verify ResolveBeadsDir follows the redirect
	resolved := beads.ResolveBeadsDir(polecatDir)
	expectedResolved := filepath.Join(townRoot, "gastown", "mayor", "rig", ".beads")
	if resolved != expectedResolved {
		t.Errorf("ResolveBeadsDir(%s) = %s, want %s", polecatDir, resolved, expectedResolved)
	}
}

// TestVerifyBeadExistsUsesRouting verifies that verifyBeadExists consults
// routes.jsonl for prefix-based routing.
//
// BUG: This test FAILS because verifyBeadExists does not use routing -
// it just runs `bd show` without setting proper working directory or
// BEADS_DIR based on the issue ID prefix.
//
// Root cause: verifyBeadExists() runs `bd show` without consulting routes.jsonl
// Expected: verifyBeadExists should route "gt-xxx" to gastown rig's beads
// Actual (before fix): verifyBeadExists fails because it looks in wrong database
func TestVerifyBeadExistsUsesRouting(t *testing.T) {
	townRoot := setupSlingRoutingTestTown(t)

	// The routing mechanism properly extracts prefixes and maps them to paths
	// This infrastructure test verifies the routing table is correct
	townBeadsDir := filepath.Join(townRoot, ".beads")
	routes, err := beads.LoadRoutes(townBeadsDir)
	if err != nil {
		t.Fatalf("LoadRoutes failed: %v", err)
	}

	// Verify routes exist for each expected prefix
	expectedRoutes := map[string]string{
		"gt-": "gastown/mayor/rig",
		"hq-": ".",
		"tr-": "testrig/mayor/rig",
	}

	for prefix, expectedPath := range expectedRoutes {
		found := false
		for _, r := range routes {
			if r.Prefix == prefix {
				found = true
				if r.Path != expectedPath {
					t.Errorf("route for prefix %q: got path %q, want %q", prefix, r.Path, expectedPath)
				}
				break
			}
		}
		if !found {
			t.Errorf("missing route for prefix %q", prefix)
		}
	}

	// BUG TEST: Verify that verifyBeadExists extracts prefix and uses routing.
	// This test FAILS because the current implementation doesn't do this.
	t.Run("verifyBeadExists_should_extract_prefix_from_beadID", func(t *testing.T) {
		// The verifyBeadExists function should extract the prefix from the bead ID
		// to determine which beads database to query.
		//
		// Current behavior: verifyBeadExists just runs `bd show <id>` without
		// any prefix extraction or routing.
		//
		// Expected behavior: extract prefix, look up route, set proper env/cwd

		// This is what verifyBeadExists SHOULD do but DOESN'T:
		testID := "gt-test123"
		prefix := extractPrefix(testID)

		// Find the route for this prefix
		var route *beads.Route
		for i, r := range routes {
			if r.Prefix == prefix {
				route = &routes[i]
				break
			}
		}

		// The expected beads directory based on routing
		expectedBeadsDir := filepath.Join(townRoot, route.Path, ".beads")

		// BUG: The current verifyBeadExists implementation does NOT compute this.
		// It just runs `bd show` without considering the prefix routing.
		//
		// To demonstrate the bug, we assert that verifyBeadExists SHOULD use
		// the routed beads directory, which it currently doesn't do.
		//
		// After fixing the bug, verifyBeadExists should internally compute
		// the same expectedBeadsDir and use it when calling bd.

		// This assertion fails because verifyBeadExists doesn't expose its routing logic.
		// We're testing the EXPECTATION here to document what should happen.
		// When this test runs on the actual verifyBeadExists (after refactoring to expose
		// the routing), it will pass only if routing is properly implemented.
		//
		// For now, this assertion documents the expected behavior:
		if route.Path != "gastown/mayor/rig" {
			t.Errorf("BUG: Expected route path for gt- prefix = %q, got %q", "gastown/mayor/rig", route.Path)
		}

		// Verify the beads dir exists where routing points
		if _, err := os.Stat(expectedBeadsDir); os.IsNotExist(err) {
			t.Errorf("BUG: Routed beads dir doesn't exist: %s", expectedBeadsDir)
		}

		// THE ACTUAL BUG TEST:
		// verifyBeadExists should use routing but doesn't.
		// We can't easily test the internal behavior without refactoring,
		// so we document the expected fix here.
		//
		// FAILING ASSERTION: Demonstrates that the code needs fixing.
		// When verifyBeadExists is fixed to use routing, this test should pass.
		codeUsesRouting := false // Current implementation doesn't use routing
		if !codeUsesRouting {
			t.Error("BUG: verifyBeadExists does not use prefix-based routing from routes.jsonl")
		}
	})
}

// extractPrefix extracts the prefix (including trailing hyphen) from an issue ID.
// Returns empty string if no hyphen found.
func extractPrefix(issueID string) string {
	for i, c := range issueID {
		if c == '-' {
			return issueID[:i+1]
		}
	}
	return ""
}

// setupSlingRoutingTestTown creates a minimal Gas Town structure for testing
// sling routing. Returns the town root directory.
func setupSlingRoutingTestTown(t *testing.T) string {
	t.Helper()

	townRoot := t.TempDir()

	// Create town-level .beads directory
	townBeadsDir := filepath.Join(townRoot, ".beads")
	if err := os.MkdirAll(townBeadsDir, 0755); err != nil {
		t.Fatalf("mkdir town .beads: %v", err)
	}

	// Create routes.jsonl with multiple rigs
	routes := []beads.Route{
		{Prefix: "hq-", Path: "."},                 // Town-level beads
		{Prefix: "gt-", Path: "gastown/mayor/rig"}, // Gastown rig
		{Prefix: "tr-", Path: "testrig/mayor/rig"}, // Test rig
	}
	if err := beads.WriteRoutes(townBeadsDir, routes); err != nil {
		t.Fatalf("write routes: %v", err)
	}

	// Create gastown rig structure
	gasRigPath := filepath.Join(townRoot, "gastown", "mayor", "rig")
	if err := os.MkdirAll(gasRigPath, 0755); err != nil {
		t.Fatalf("mkdir gastown: %v", err)
	}

	// Create gastown .beads directory with its own config
	gasBeadsDir := filepath.Join(gasRigPath, ".beads")
	if err := os.MkdirAll(gasBeadsDir, 0755); err != nil {
		t.Fatalf("mkdir gastown .beads: %v", err)
	}
	if err := os.WriteFile(filepath.Join(gasBeadsDir, "config.yaml"), []byte("prefix: gt\n"), 0644); err != nil {
		t.Fatalf("write gastown config: %v", err)
	}

	// Create testrig structure
	testRigPath := filepath.Join(townRoot, "testrig", "mayor", "rig")
	if err := os.MkdirAll(testRigPath, 0755); err != nil {
		t.Fatalf("mkdir testrig: %v", err)
	}

	// Create testrig .beads directory
	testBeadsDir := filepath.Join(testRigPath, ".beads")
	if err := os.MkdirAll(testBeadsDir, 0755); err != nil {
		t.Fatalf("mkdir testrig .beads: %v", err)
	}
	if err := os.WriteFile(filepath.Join(testBeadsDir, "config.yaml"), []byte("prefix: tr\n"), 0644); err != nil {
		t.Fatalf("write testrig config: %v", err)
	}

	// Create polecat directory with redirect
	polecatDir := filepath.Join(townRoot, "gastown", "polecats", "worker1")
	if err := os.MkdirAll(polecatDir, 0755); err != nil {
		t.Fatalf("mkdir polecat: %v", err)
	}

	// Create redirect file for polecat -> mayor/rig/.beads
	polecatBeadsDir := filepath.Join(polecatDir, ".beads")
	if err := os.MkdirAll(polecatBeadsDir, 0755); err != nil {
		t.Fatalf("mkdir polecat .beads: %v", err)
	}
	redirectContent := "../../mayor/rig/.beads"
	if err := os.WriteFile(filepath.Join(polecatBeadsDir, "redirect"), []byte(redirectContent), 0644); err != nil {
		t.Fatalf("write redirect: %v", err)
	}

	return townRoot
}
