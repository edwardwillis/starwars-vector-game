package telemetry

import (
	"encoding/csv"
	"os"
	"testing"
	"time"
)

func TestRecorderAggregatesFramesIntoOneSecondSamples(t *testing.T) {
	file, err := os.CreateTemp(t.TempDir(), "surface-flight-*.csv")
	if err != nil {
		t.Fatal(err)
	}
	path := file.Name()
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	recorder, err := NewCSV(path)
	if err != nil {
		t.Fatal(err)
	}
	start := time.Unix(100, 0)
	recorder.Observe(Frame{At: start, SessionKind: "replay", RecordingID: "fixture", ReplayVerified: true, SimulationTick: 120, ReplayInputCount: 120, DrawDuration: 10 * time.Millisecond, UpdateDuration: 4 * time.Millisecond, UpdateCalls: 1, PrepareDuration: time.Millisecond, BackgroundDuration: 2 * time.Millisecond, SurfaceDuration: 3 * time.Millisecond, OverlayDuration: 4 * time.Millisecond, CockpitStatic: time.Millisecond, CockpitDynamic: 2 * time.Millisecond, Reticle: 3 * time.Millisecond, MissionDirector: 4 * time.Millisecond, HUD: 5 * time.Millisecond, ActualFPS: 60, ActualTPS: 60, Flow: "playing", MissionPhase: "surface-assault", View: "Cockpit", Realism: 4, ActiveTiles: 25, DepthTested: 60000})
	recorder.Observe(Frame{At: start.Add(500 * time.Millisecond), SessionKind: "replay", RecordingID: "fixture", ReplayVerified: true, SimulationTick: 150, ReplayInputCount: 150, DrawDuration: 30 * time.Millisecond, UpdateDuration: 8 * time.Millisecond, UpdateCalls: 2, PrepareDuration: 2 * time.Millisecond, BackgroundDuration: 4 * time.Millisecond, SurfaceDuration: 6 * time.Millisecond, OverlayDuration: 8 * time.Millisecond, CockpitStatic: 2 * time.Millisecond, CockpitDynamic: 4 * time.Millisecond, Reticle: 6 * time.Millisecond, MissionDirector: 8 * time.Millisecond, HUD: 10 * time.Millisecond, ActualFPS: 59, ActualTPS: 58, Flow: "playing", MissionPhase: "surface-assault", View: "Cockpit", Realism: 4, ActiveTiles: 27, DepthTested: 80000})
	recorder.Observe(Frame{At: start.Add(1100 * time.Millisecond), SessionKind: "replay", RecordingID: "fixture", ReplayVerified: true, SimulationTick: 180, ReplayInputCount: 180, DrawDuration: 12 * time.Millisecond, UpdateDuration: 12 * time.Millisecond, UpdateCalls: 1, PrepareDuration: 3 * time.Millisecond, BackgroundDuration: 6 * time.Millisecond, SurfaceDuration: 9 * time.Millisecond, OverlayDuration: 12 * time.Millisecond, ActualFPS: 58, ActualTPS: 56, Flow: "playing", MissionPhase: "surface-assault", View: "Cockpit", Realism: 4, PlayerShield: 7, Collisions: 2, SurfaceAttackers: 4, HostileProjectiles: 3, SurfaceEntryGrace: 1.25, ActiveTiles: 29, DepthTested: 100000})
	recorder.Observe(Frame{At: start.Add(1300 * time.Millisecond), SessionKind: "replay", RecordingID: "fixture", ReplayVerified: true, SimulationTick: 210, ReplayInputCount: 210, DrawDuration: 15 * time.Millisecond, UpdateDuration: 4 * time.Millisecond, UpdateCalls: 1, ActualFPS: 57, ActualTPS: 55, Flow: "playing", MissionPhase: "surface-assault", View: "Cockpit", Realism: 4, ActiveTiles: 31, DepthTested: 120000})
	if err := recorder.Close(); err != nil {
		t.Fatal(err)
	}

	opened, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer opened.Close()
	records, err := csv.NewReader(opened).ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	if len(records) != 3 { // header, one full interval, one final partial interval
		t.Fatalf("records=%d, want 3: %v", len(records), records)
	}
	columns := make(map[string]int, len(records[0]))
	for index, name := range records[0] {
		columns[name] = index
	}
	value := func(name string) string { return records[1][columns[name]] }
	if got := value("frames"); got != "3" {
		t.Fatalf("first sample frames=%q, want 3", got)
	}
	if got := value("active_tiles_avg"); got != "27.000" {
		t.Fatalf("first sample active tiles=%q, want average 27", got)
	}
	if got := value("depth_pixels_tested_avg"); got != "80000.000" {
		t.Fatalf("first sample depth tested=%q, want average 80000", got)
	}
	if got := value("updates_avg"); got != "1.333" {
		t.Fatalf("first sample updates=%q, want average 1.333", got)
	}
	if got := value("update_ms_per_update_avg"); got != "6.000" {
		t.Fatalf("first sample update milliseconds=%q, want average 6", got)
	}
	if got := value("background_ms_avg"); got != "4.000" {
		t.Fatalf("first sample background milliseconds=%q, want average 4", got)
	}
	if got := value("cockpit_dynamic_ms_avg"); got != "2.000" {
		t.Fatalf("first sample cockpit dynamic milliseconds=%q, want average 2", got)
	}
	if got := value("reticle_ms_avg"); got != "3.000" {
		t.Fatalf("first sample reticle milliseconds=%q, want average 3", got)
	}
	if got := value("player_shield"); got != "7" {
		t.Fatalf("first sample player shield=%q, want 7", got)
	}
	if got := value("surface_attackers"); got != "4" {
		t.Fatalf("first sample surface attackers=%q, want 4", got)
	}
	if got := value("surface_entry_grace_s"); got != "1.250" {
		t.Fatalf("first sample entry grace=%q, want 1.250", got)
	}
	if got := value("session_kind"); got != "replay" {
		t.Fatalf("first sample session kind=%q, want replay", got)
	}
	if got := value("recording_id"); got != "fixture" {
		t.Fatalf("first sample recording ID=%q, want fixture", got)
	}
	if got := value("replay_verified"); got != "true" {
		t.Fatalf("first sample replay verification=%q, want true", got)
	}
	if got := value("simulation_tick_start"); got != "120" {
		t.Fatalf("first sample simulation tick start=%q, want 120", got)
	}
	if got := value("simulation_tick_end"); got != "180" {
		t.Fatalf("first sample simulation tick end=%q, want 180", got)
	}
	if got := value("replay_input_start"); got != "120" {
		t.Fatalf("first sample replay input start=%q, want 120", got)
	}
	if got := value("replay_input_end"); got != "180" {
		t.Fatalf("first sample replay input end=%q, want 180", got)
	}
	if got := records[2][columns["simulation_tick_start"]]; got != "210" {
		t.Fatalf("final sample simulation tick start=%q, want 210", got)
	}
	if got := records[2][columns["simulation_tick_end"]]; got != "210" {
		t.Fatalf("final sample simulation tick end=%q, want 210", got)
	}
}
