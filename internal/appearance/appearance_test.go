package appearance

import "testing"

func TestDeathStarArcadeProgressivelyRevealsStableDetail(t *testing.T) {
	billboard := DeathStarArcade().Billboard
	far := billboard.Lines(0)
	near := billboard.Lines(1)
	if len(far) >= len(near) {
		t.Fatalf("far=%d near=%d", len(far), len(near))
	}
	first := billboard.Lines(0.5)
	second := billboard.Lines(0.5)
	if len(first) != len(second) {
		t.Fatal("detail reveal is not deterministic")
	}
	for index := range first {
		if first[index] != second[index] {
			t.Fatal("detail reveal changed between identical inputs")
		}
	}
}

func TestAppearanceRegistrySelectsByLogicalObject(t *testing.T) {
	definition, ok := DefaultRegistry().ForObject("builtin/death-star", "")
	if !ok || definition.Name != DeathStarArcadeName {
		t.Fatalf("definition=%+v ok=%t", definition, ok)
	}
}
