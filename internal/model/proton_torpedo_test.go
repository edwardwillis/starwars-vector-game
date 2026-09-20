package model

import "testing"

func TestProtonTorpedoIsValidSparseLineArt(t *testing.T) {
	mesh := ProtonTorpedo()
	if err := mesh.Validate(); err != nil {
		t.Fatalf("ProtonTorpedo returned invalid geometry: %v", err)
	}
	if len(mesh.Faces) != 0 || len(mesh.Edges) != 9 || !mesh.SkipDepth {
		t.Fatalf("unexpected torpedo geometry: faces=%d edges=%d skipDepth=%t", len(mesh.Faces), len(mesh.Edges), mesh.SkipDepth)
	}
}
