package model

import "testing"

func TestCockpitConsoleIsValid(t *testing.T) {
	console := CockpitConsole()
	if len(console.Verts) != 14 || len(console.Edges) != 11 {
		t.Fatalf("console has %d vertices and %d edges, want 14 and 11", len(console.Verts), len(console.Edges))
	}
	if err := console.Validate(); err != nil {
		t.Fatalf("CockpitConsole returned an invalid model: %v", err)
	}
}

func TestCockpitCannonsAreValid(t *testing.T) {
	cannons := CockpitCannons()
	if len(cannons.Verts) != 16 || len(cannons.Edges) != 24 {
		t.Fatalf("cannons have %d vertices and %d edges, want 16 and 24", len(cannons.Verts), len(cannons.Edges))
	}
	if err := cannons.Validate(); err != nil {
		t.Fatalf("CockpitCannons returned an invalid model: %v", err)
	}
}
