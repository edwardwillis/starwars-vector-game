package main

import (
	"encoding/csv"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCompareReportsMedianAcrossVerifiedEquivalentTrials(t *testing.T) {
	directory := t.TempDir()
	baselineOne := writeTelemetry(t, directory, "baseline-1.csv", "recording-a", "cockpit", "5", "0,60,60,10,2,3,4,5,6,7,8,9,100,200\n60,120,60,20,4,6,8,10,12,14,16,18,120,220")
	baselineTwo := writeTelemetry(t, directory, "baseline-2.csv", "recording-a", "cockpit", "5", "0,60,60,30,6,9,12,15,18,21,24,27,140,240\n60,120,60,40,8,12,16,20,24,28,32,36,160,260")
	candidate := writeTelemetry(t, directory, "candidate.csv", "recording-a", "cockpit", "5", "0,60,60,5,1,1.5,2,2.5,3,3.5,4,4.5,180,180\n60,120,60,15,3,4.5,6,7.5,9,10.5,12,13.5,200,200")

	var output strings.Builder
	if err := compare([]string{baselineOne, baselineTwo}, []string{candidate}, &output); err != nil {
		t.Fatalf("compare: %v", err)
	}
	report := output.String()
	for _, want := range []string{
		"Recording: recording-a",
		"Shared simulation tick interval: [0, 120]",
		"Trials: baseline=2 candidate=1",
		"Draw                       25.000 ms/frame       10.000 ms/frame",
		"Worst sustained draw row: baseline 30.000 ms/frame; candidate 15.000 ms/frame",
	} {
		if !strings.Contains(report, want) {
			t.Fatalf("report missing %q:\n%s", want, report)
		}
	}
}

func TestCompareRejectsMismatchedRecordingAndUnverifiedTrials(t *testing.T) {
	directory := t.TempDir()
	baseline := writeTelemetry(t, directory, "baseline.csv", "recording-a", "cockpit", "5", "0,60,60,10,2,3,4,5,6,7,8,9,100,200")
	mismatch := writeTelemetry(t, directory, "mismatch.csv", "recording-b", "cockpit", "5", "0,60,60,10,2,3,4,5,6,7,8,9,100,200")
	if err := compare([]string{baseline}, []string{mismatch}, &strings.Builder{}); err == nil || !strings.Contains(err.Error(), "recording IDs differ") {
		t.Fatalf("mismatched recording error = %v", err)
	}
	unverified := filepath.Join(directory, "unverified.csv")
	contents, err := os.ReadFile(baseline)
	if err != nil {
		t.Fatalf("read baseline: %v", err)
	}
	if err := os.WriteFile(unverified, []byte(strings.Replace(string(contents), ",true,", ",false,", 1)), 0o644); err != nil {
		t.Fatalf("write unverified: %v", err)
	}
	if err := compare([]string{baseline}, []string{unverified}, &strings.Builder{}); err == nil || !strings.Contains(err.Error(), "not a verified replay") {
		t.Fatalf("unverified replay error = %v", err)
	}
}

func TestCompareUsesOnlySharedTickRange(t *testing.T) {
	directory := t.TempDir()
	baseline := writeTelemetry(t, directory, "baseline.csv", "recording-a", "cockpit", "5", "0,100,100,100,1,1,1,1,1,1,1,1,1,1\n100,200,100,20,1,1,1,1,1,1,1,1,1,1")
	candidate := writeTelemetry(t, directory, "candidate.csv", "recording-a", "cockpit", "5", "50,150,100,10,1,1,1,1,1,1,1,1,1,1")
	var output strings.Builder
	if err := compare([]string{baseline}, []string{candidate}, &output); err != nil {
		t.Fatalf("compare shared interval: %v", err)
	}
	if !strings.Contains(output.String(), "Shared simulation tick interval: [50, 150]") || !strings.Contains(output.String(), "Draw                       60.000 ms/frame") {
		t.Fatalf("unexpected shared-range report:\n%s", output.String())
	}
}

func writeTelemetry(t *testing.T, directory, name, recordingID, view, realism, rows string) string {
	t.Helper()
	path := filepath.Join(directory, name)
	header := strings.Split("elapsed_s,interval_s,frames,session_kind,recording_id,replay_verified,simulation_tick_start,simulation_tick_end,replay_input_start,replay_input_end,draw_ms_avg,draw_ms_max,updates_avg,update_ms_avg,update_ms_per_update_avg,actual_fps_avg,actual_tps_avg,flow,mission_phase,view,realism,player_shield,collisions_total,surface_attackers,hostile_projectiles,surface_entry_grace_s,active_tiles_avg,features_avg,candidates_avg,depth_domains_avg,depth_faces_avg,depth_triangles_avg,depth_pixels_tested_avg,depth_pixels_written_avg,line_depth_samples_avg,prepare_ms_avg,background_ms_avg,depth_raster_ms_avg,geometry_ms_avg,surface_ms_avg,opaque_submit_ms_avg,vector_submit_ms_avg,overlay_ms_avg,output_edges_avg,render_jobs_avg", ",")
	file, err := os.Create(path)
	if err != nil {
		t.Fatalf("create telemetry: %v", err)
	}
	writer := csv.NewWriter(file)
	if err := writer.Write(header); err != nil {
		t.Fatalf("write header: %v", err)
	}
	columns := make(map[string]int, len(header))
	for index, column := range header {
		columns[column] = index
	}
	for _, values := range strings.Split(rows, "\n") {
		parts := strings.Split(values, ",")
		row := make([]string, len(header))
		set := func(column, value string) { row[columns[column]] = value }
		set("elapsed_s", "1")
		set("interval_s", "1")
		set("frames", parts[2])
		set("session_kind", "replay")
		set("recording_id", recordingID)
		set("replay_verified", "true")
		set("simulation_tick_start", parts[0])
		set("simulation_tick_end", parts[1])
		set("replay_input_start", parts[0])
		set("replay_input_end", parts[1])
		set("draw_ms_avg", parts[3])
		set("update_ms_avg", parts[4])
		set("actual_fps_avg", "60")
		set("actual_tps_avg", "60")
		set("flow", "playing")
		set("mission_phase", "surface")
		set("view", view)
		set("realism", realism)
		set("depth_raster_ms_avg", parts[5])
		set("geometry_ms_avg", parts[6])
		set("surface_ms_avg", parts[7])
		set("opaque_submit_ms_avg", parts[8])
		set("vector_submit_ms_avg", parts[9])
		set("overlay_ms_avg", parts[10])
		set("output_edges_avg", parts[11])
		if err := writer.Write(row); err != nil {
			t.Fatalf("write telemetry row: %v", err)
		}
	}
	writer.Flush()
	if err := writer.Error(); err != nil {
		t.Fatalf("flush telemetry: %v", err)
	}
	if err := file.Close(); err != nil {
		t.Fatalf("close telemetry: %v", err)
	}
	return path
}
