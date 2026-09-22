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
	recorder.Observe(Frame{At: start, DrawDuration: 10 * time.Millisecond, UpdateDuration: 4 * time.Millisecond, UpdateCalls: 1, PrepareDuration: time.Millisecond, BackgroundDuration: 2 * time.Millisecond, SurfaceDuration: 3 * time.Millisecond, OverlayDuration: 4 * time.Millisecond, ActualFPS: 60, ActualTPS: 60, Flow: "playing", MissionPhase: "surface-assault", View: "Cockpit", Realism: 4, ActiveTiles: 25, DepthTested: 60000})
	recorder.Observe(Frame{At: start.Add(500 * time.Millisecond), DrawDuration: 30 * time.Millisecond, UpdateDuration: 8 * time.Millisecond, UpdateCalls: 2, PrepareDuration: 2 * time.Millisecond, BackgroundDuration: 4 * time.Millisecond, SurfaceDuration: 6 * time.Millisecond, OverlayDuration: 8 * time.Millisecond, ActualFPS: 59, ActualTPS: 58, Flow: "playing", MissionPhase: "surface-assault", View: "Cockpit", Realism: 4, ActiveTiles: 27, DepthTested: 80000})
	recorder.Observe(Frame{At: start.Add(1100 * time.Millisecond), DrawDuration: 12 * time.Millisecond, UpdateDuration: 12 * time.Millisecond, UpdateCalls: 1, PrepareDuration: 3 * time.Millisecond, BackgroundDuration: 6 * time.Millisecond, SurfaceDuration: 9 * time.Millisecond, OverlayDuration: 12 * time.Millisecond, ActualFPS: 58, ActualTPS: 56, Flow: "playing", MissionPhase: "surface-assault", View: "Cockpit", Realism: 4, ActiveTiles: 29, DepthTested: 100000})
	recorder.Observe(Frame{At: start.Add(1300 * time.Millisecond), DrawDuration: 15 * time.Millisecond, UpdateDuration: 4 * time.Millisecond, UpdateCalls: 1, ActualFPS: 57, ActualTPS: 55, Flow: "playing", MissionPhase: "surface-assault", View: "Cockpit", Realism: 4, ActiveTiles: 31, DepthTested: 120000})
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
}
