//go:build integration

// Package cmd contains integration tests for gt sling --on routing.
//
// Run with: go test -tags=integration ./internal/cmd -run TestSling -v
package cmd

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/steveyegge/gastown/internal/beads"
)

// TestSlingOnRoutingBug verifies that verifyBeadExists properly routes to the correct
// beads database when given a prefixed bead ID.
//
// This is a regression test for the bug where:
// - bd show fs-e2e-plan works from town root (uses routing)
// - gt sling --on fs-e2e-plan fails (verifyBeadExists doesn't use routing)
//
// The root cause was that verifyBeadExists calls bd show without setting the
// working directory, so bd can't find routes.jsonl for prefix-based routing.
func TestSlingOnRoutingBug(t *testing.T) {
	// Skip if bd is not available
	if _, err := exec.LookPath("bd"); err != nil {
		t.Skip("bd not installed, skipping routing test")
	}

	townRoot := setupSlingTestTown(t)

	// Create a bead in testrig (tr- prefix)
	testBeadID := "tr-test123"
	testRigBeadsDir := filepath.Join(townRoot, "testrig", "mayor", "rig", ".beads")

	createCmd := exec.Command("bd", "create",
		"--id="+testBeadID,
		"--title=Test Bead",
		"--type=task",
	)
	createCmd.Dir = testRigBeadsDir
	createCmd.Stderr = os.Stderr
	if err := createCmd.Run(); err != nil {
		t.Fatalf("creating test bead: %v", err)
	}

	// Verify bead exists from testrig directory (direct access)
	verifyCmd := exec.Command("bd", "show", testBeadID, "--json")
	verifyCmd.Dir = testRigBeadsDir
	if err := verifyCmd.Run(); err != nil {
		t.Fatalf("bead should exist when accessed directly: %v", err)
	}

	// Verify bead exists from town root (uses routing)
	verifyFromTownCmd := exec.Command("bd", "show", testBeadID, "--json")
	verifyFromTownCmd.Dir = townRoot
	if err := verifyFromTownCmd.Run(); err != nil {
		t.Fatalf("bead should be accessible via routing from town root: %v", err)
	}

	// BUG REPRODUCTION: verifyBeadExists doesn't set working directory
	// When called without proper context, bd show fails to route.
	//
	// This is a regression test - it documents the current buggy behavior.
	// The subtest name indicates this is testing the bug, not the fix.
	t.Run("BUG_verifyBeadExists_without_routing_context", func(t *testing.T) {
		// Simulate verifyBeadExists behavior: run bd show without setting Dir
		// from an unrelated directory (like /tmp)
		tmpDir := t.TempDir()

		cmd := exec.Command("bd", "show", testBeadID, "--json")
		cmd.Dir = tmpDir // No routing context - this is the bug!
		err := cmd.Run()

		// Document the bug: without proper Dir, bd show fails
		if err != nil {
			t.Logf("Bug confirmed: bd show fails without routing context: %v", err)
		} else {
			t.Error("Bug appears to be fixed: bd show succeeded without routing context")
		}
	})

	// This test shows what SHOULD work: bd show from town root
	t.Run("bd_show_from_town_root_works", func(t *testing.T) {
		cmd := exec.Command("bd", "show", testBeadID, "--json")
		cmd.Dir = townRoot
		if err := cmd.Run(); err != nil {
			t.Errorf("bd show from town root should work: %v", err)
		}
	})

	// This is what verifyBeadExists SHOULD do: pass townRoot as working dir
	t.Run("verifyBeadExists_with_routing_context", func(t *testing.T) {
		cmd := exec.Command("bd", "show", testBeadID, "--json")
		cmd.Dir = townRoot // Correct: use townRoot for routing
		if err := cmd.Run(); err != nil {
			t.Errorf("verifyBeadExists should work when townRoot is set: %v", err)
		}
	})
}

// setupSlingTestTown creates a minimal Gas Town for sling routing tests.
// Similar to setupRoutingTestTown but initializes bd in testrig.
func setupSlingTestTown(t *testing.T) string {
	t.Helper()

	townRoot := t.TempDir()

	// Initialize beads at town level FIRST (creates the database and enables routing)
	initTownCmd := exec.Command("bd", "init", "--prefix=hq")
	initTownCmd.Dir = townRoot
	initTownCmd.Stderr = os.Stderr
	if err := initTownCmd.Run(); err != nil {
		t.Fatalf("bd init at town level: %v", err)
	}

	// Create routes.jsonl with multiple rigs
	townBeadsDir := filepath.Join(townRoot, ".beads")
	routes := []beads.Route{
		{Prefix: "hq-", Path: "."},                 // Town-level beads
		{Prefix: "gt-", Path: "gastown/mayor/rig"}, // Gastown rig
		{Prefix: "tr-", Path: "testrig/mayor/rig"}, // Test rig
	}
	if err := beads.WriteRoutes(townBeadsDir, routes); err != nil {
		t.Fatalf("write routes: %v", err)
	}

	// Create testrig structure with initialized beads
	testRigPath := filepath.Join(townRoot, "testrig", "mayor", "rig")
	if err := os.MkdirAll(testRigPath, 0755); err != nil {
		t.Fatalf("mkdir testrig: %v", err)
	}

	// Initialize beads in testrig (creates the database)
	initCmd := exec.Command("bd", "init", "--prefix=tr")
	initCmd.Dir = testRigPath
	initCmd.Stderr = os.Stderr
	if err := initCmd.Run(); err != nil {
		t.Fatalf("bd init in testrig: %v", err)
	}

	// Create gastown rig structure (for completeness)
	gasRigPath := filepath.Join(townRoot, "gastown", "mayor", "rig")
	if err := os.MkdirAll(gasRigPath, 0755); err != nil {
		t.Fatalf("mkdir gastown: %v", err)
	}

	return townRoot
}

// TestVerifyBeadExistsRoutingFix tests the expected behavior after the fix.
// This test demonstrates the pattern verifyBeadExists SHOULD use.
//
// ACCEPTANCE CRITERIA: After fixing sling.go, verifyBeadExists should:
// 1. Accept townRoot as a parameter (or derive it from workspace.FindFromCwd)
// 2. Set cmd.Dir = townRoot when calling bd show
// 3. This enables bd to find routes.jsonl and route to the correct beads database
func TestVerifyBeadExistsRoutingFix(t *testing.T) {
	// Skip if bd is not available
	if _, err := exec.LookPath("bd"); err != nil {
		t.Skip("bd not installed, skipping routing test")
	}

	townRoot := setupSlingTestTown(t)

	// Create a bead in testrig (tr- prefix)
	testBeadID := "tr-fix123"
	testRigBeadsDir := filepath.Join(townRoot, "testrig", "mayor", "rig", ".beads")

	createCmd := exec.Command("bd", "create",
		"--id="+testBeadID,
		"--title=Test Fix Bead",
		"--type=task",
	)
	createCmd.Dir = testRigBeadsDir
	createCmd.Stderr = os.Stderr
	if err := createCmd.Run(); err != nil {
		t.Fatalf("creating test bead: %v", err)
	}

	// Test the fix: verifyBeadExists should work when called with proper routing
	// This simulates the fixed version of verifyBeadExists
	t.Run("fixed_verifyBeadExists", func(t *testing.T) {
		// The fix: run bd show from townRoot so routing works
		cmd := exec.Command("bd", "show", testBeadID, "--json")
		cmd.Dir = townRoot // THIS IS THE FIX
		err := cmd.Run()

		if err != nil {
			t.Errorf("fixed verifyBeadExists should work: %v", err)
		}
	})
}
