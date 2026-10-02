package cockpit

import "testing"

func TestDefaultRegistryProvidesDistinctXWingAndTIEWindshields(t *testing.T) {
	registry := DefaultRegistry()
	xwing, xwingOK := registry.ForDefinition("builtin/x-wing")
	tie, tieOK := registry.ForDefinition("builtin/tie-fighter")
	if !xwingOK || !tieOK || len(tie.Windshield.Segments) == 0 {
		t.Fatalf("windshield layouts xwing=%+v tie=%+v", xwing.Windshield, tie.Windshield)
	}
	if tie.Windshield.LineWidth <= 0 || tie.Windshield.Color.A == 0 {
		t.Fatalf("windshield styles xwing=%+v tie=%+v", xwing.Windshield, tie.Windshield)
	}
	if len(xwing.Windshield.OpaquePolygons) != 1 || xwing.Windshield.OpaqueColor.A != 255 {
		t.Fatalf("X-wing cockpit is missing its single opaque enclosure: %+v", xwing.Windshield)
	}
	if len(xwing.Windshield.OpaqueOutlineSegments) != 9 || xwing.Windshield.OpaqueOutlineColor.A == 0 || xwing.Windshield.OpaqueOutlineWidth <= 0 {
		t.Fatalf("X-wing cockpit has no restrained external frame outline: %+v", xwing.Windshield)
	}
	if len(xwing.Windshield.Segments) != 0 || len(xwing.Windshield.InstrumentSegments) != 0 {
		t.Fatalf("X-wing cockpit has unreviewed line art: %+v", xwing.Windshield)
	}
	if xwing.Cannons[0].Y >= 100 || xwing.Cannons[2].Y <= 380 || xwing.Cannons[0].X != 36 || xwing.Cannons[1].X != 924 {
		t.Fatalf("X-wing cannons overlap its opaque coaming: %+v", xwing.Cannons)
	}
	if len(tie.Windshield.OpaquePolygons) != 0 {
		t.Fatal("X-wing and TIE received indistinguishable windshield geometry")
	}
}

func TestUnspecifiedCockpitLayoutHasNoWindshield(t *testing.T) {
	interceptor, ok := DefaultRegistry().ForDefinition("builtin/tie-interceptor")
	if !ok || len(interceptor.Windshield.Segments) != 0 {
		t.Fatalf("interceptor windshield=%+v, want no unreviewed placeholder frame", interceptor.Windshield)
	}
}
