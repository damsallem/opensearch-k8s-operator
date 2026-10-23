package helpers

import (
	"testing"

	opsterv1 "opensearch.opster.io/api/v1"
)

func countUpgrader(statuses []opsterv1.ComponentStatus, pool string) int {
	count := 0
	for _, s := range statuses {
		if s.Component == "Upgrader" && s.Description == pool {
			count++
		}
	}
	return count
}

// START backport #1551 (upstream v3.0.0)

// Upgrading -> Upgraded replayed on a list that already has Upgraded (stale cache snapshot at
// reconcile start, or RetryOnConflict re-running the closure after a conflict) must not add
// a second entry, otherwise a leftover makes the next upgrade skip the pool.
func TestReplaceReplayedTransitionDoesNotDuplicate(t *testing.T) {
	upgrading := opsterv1.ComponentStatus{Component: "Upgrader", Description: "data", Status: "Upgrading"}
	upgraded := opsterv1.ComponentStatus{Component: "Upgrader", Description: "data", Status: "Upgraded"}
	statuses := []opsterv1.ComponentStatus{upgraded}

	statuses = Replace(upgrading, upgraded, statuses)

	if n := countUpgrader(statuses, "data"); n != 1 {
		t.Errorf("expected 1 Upgrader entry for data after replayed transition, got %d: %v", n, statuses)
	}
}

func TestReplaceSelfHealsDuplicates(t *testing.T) {
	remove := opsterv1.ComponentStatus{Component: "Upgrader", Description: "data", Status: "Upgrading"}
	add := opsterv1.ComponentStatus{Component: "Upgrader", Description: "data", Status: "Upgraded"}
	other := opsterv1.ComponentStatus{Component: "Scaler", Description: "data", Status: "Running"}
	statuses := []opsterv1.ComponentStatus{
		{Component: "Upgrader", Description: "data", Status: "Upgraded"},
		other,
		{Component: "Upgrader", Description: "data", Status: "Upgrading"},
	}

	statuses = Replace(remove, add, statuses)

	if len(statuses) != 2 || statuses[0].Component != "Scaler" || countUpgrader(statuses, "data") != 1 {
		t.Errorf("expected Scaler entry and a single Upgrader entry for data, got %v", statuses)
	}
}

func TestRemoveItDoesNotMutateInput(t *testing.T) {
	statuses := []opsterv1.ComponentStatus{
		{Component: "Scaler", Description: "data", Status: "Running"},
		{Component: "Scaler", Description: "masters", Status: "Running"},
	}
	statusesCopy := append([]opsterv1.ComponentStatus(nil), statuses...)

	_ = RemoveIt(opsterv1.ComponentStatus{Component: "Scaler", Description: "data", Status: "Running"}, statuses)

	for i := range statuses {
		if statuses[i].Description != statusesCopy[i].Description {
			t.Fatalf("input slice was mutated: %v", statuses)
		}
	}
}

// END backport #1551

// START backport #1473 (upstream v3.0.0)

// Leftovers from previous upgrades (duplicates, node pools removed from the spec) must all be
// removed, while other components are kept.
func TestClearUpgraderComponentStatuses(t *testing.T) {
	scaler := opsterv1.ComponentStatus{Component: "Scaler", Description: "data", Status: "Running"}
	statuses := []opsterv1.ComponentStatus{
		{Component: "Upgrader", Description: "data", Status: "Upgraded"},
		{Component: "Upgrader", Description: "data", Status: "Upgraded"},
		scaler,
		{Component: "Upgrader", Description: "removed-pool", Status: "Upgrading"},
	}

	statuses = ClearUpgraderComponentStatuses(statuses)

	if len(statuses) != 1 || statuses[0].Component != "Scaler" {
		t.Errorf("expected only the Scaler entry to be kept, got %v", statuses)
	}
	if UpgradeInProgress(opsterv1.ClusterStatus{ComponentsStatus: statuses}) {
		t.Errorf("expected no upgrade in progress after clearing Upgrader entries")
	}
}

// END backport #1473
