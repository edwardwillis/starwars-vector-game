package appearance

import (
	"testing"

	"github.com/edwardwillis/starwars-vector-game/internal/catalog"
)

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

func TestDeathStarArcadeUsesSmoothOrbitalApproachScale(t *testing.T) {
	billboard := DeathStarArcade().Billboard
	far := billboard.RadiusScale(billboard.FarDepth + 100)
	middle := billboard.RadiusScale((billboard.NearDepth + billboard.FarDepth) / 2)
	near := billboard.RadiusScale(billboard.NearDepth - 1)
	if far != billboard.FarScale || !(far < middle && middle < near) || near != 1 {
		t.Fatalf("approach scale far=%v middle=%v near=%v, config=%+v", far, middle, near, billboard)
	}
	if plain := (Billboard{}).RadiusScale(1000); plain != 1 {
		t.Fatalf("ordinary billboard scale=%v, want 1", plain)
	}
}

func TestAppearanceRegistrySelectsByLogicalObject(t *testing.T) {
	definition, ok := DefaultRegistry().ForObject("builtin/death-star", "")
	if !ok || definition.Name != DeathStarArcadeName {
		t.Fatalf("definition=%+v ok=%t", definition, ok)
	}
}

func TestAppearanceRegistryIncludesFactionLaserStyles(t *testing.T) {
	registry := DefaultRegistry()
	for _, name := range []string{catalog.RebelLaserBoltAppearance, catalog.ImperialLaserBoltAppearance} {
		definition, err := registry.Lookup(name)
		if err != nil || definition.ObjectDefinition != catalog.LaserBoltName || definition.Kind != "model-3d" {
			t.Fatalf("laser appearance %q=%+v err=%v", name, definition, err)
		}
	}
}

func TestAppearanceRegistryIncludesTIEInterceptorModel(t *testing.T) {
	definition, err := DefaultRegistry().Lookup(catalog.TIEInterceptorAppearance)
	if err != nil {
		t.Fatal(err)
	}
	if definition.ObjectDefinition != catalog.TIEInterceptorName || definition.Kind != "model-3d" {
		t.Fatalf("definition=%+v", definition)
	}
}
