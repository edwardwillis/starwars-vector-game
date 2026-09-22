// Package telemetry records low-overhead development performance samples.
package telemetry

import (
	"encoding/csv"
	"fmt"
	"os"
	"strconv"
	"time"
)

// Frame is one completed presentation frame. Counter fields are existing
// renderer instrumentation snapshots, not additional per-pixel probes.
type Frame struct {
	At                 time.Time
	DrawDuration       time.Duration
	UpdateDuration     time.Duration
	UpdateCalls        int
	PrepareDuration    time.Duration
	BackgroundDuration time.Duration
	SurfaceDuration    time.Duration
	OverlayDuration    time.Duration
	ActualFPS          float64
	ActualTPS          float64
	Flow               string
	MissionPhase       string
	View               string
	Realism            int
	ActiveTiles        int
	Features           int
	Candidates         int
	DepthDomains       int
	DepthFaces         int
	DepthTriangles     int
	DepthTested        int
	DepthWritten       int
	LineSamples        int
	DepthMS            float64
	GeometryMS         float64
	OpaqueMS           float64
	VectorMS           float64
	OutputEdges        int
	RenderJobs         int
}

// Recorder aggregates frame observations into one CSV row per interval. File
// writes are therefore outside the per-frame render hot path.
type Recorder struct {
	file     *os.File
	writer   *csv.Writer
	interval time.Duration
	started  time.Time
	last     time.Time
	frames   int
	drawSum  time.Duration
	drawMax  time.Duration
	latest   Frame
	sums     frameSums
	err      error
}

type frameSums struct {
	activeTiles, features, candidates, depthDomains                    int64
	depthFaces, depthTriangles, depthTested, depthWritten, lineSamples int64
	depthMS, geometryMS, opaqueMS, vectorMS                            float64
	outputEdges, renderJobs                                            int64
	updateCalls                                                        int64
	updateDuration, prepareDuration                                    time.Duration
	backgroundDuration, surfaceDuration, overlayDuration               time.Duration
	actualFPS, actualTPS                                               float64
}

// NewCSV creates a recorder with one-second samples. Existing files are
// replaced intentionally: each run produces one self-contained trace.
func NewCSV(path string) (*Recorder, error) {
	file, err := os.Create(path)
	if err != nil {
		return nil, fmt.Errorf("create telemetry file: %w", err)
	}
	recorder := &Recorder{
		file: file, writer: csv.NewWriter(file), interval: time.Second,
	}
	if err := recorder.writer.Write([]string{
		"elapsed_s", "interval_s", "frames", "draw_ms_avg", "draw_ms_max", "updates_avg", "update_ms_avg", "update_ms_per_update_avg", "actual_fps_avg", "actual_tps_avg",
		"flow", "mission_phase", "view", "realism",
		"active_tiles_avg", "features_avg", "candidates_avg", "depth_domains_avg",
		"depth_faces_avg", "depth_triangles_avg", "depth_pixels_tested_avg", "depth_pixels_written_avg", "line_depth_samples_avg",
		"prepare_ms_avg", "background_ms_avg", "depth_raster_ms_avg", "geometry_ms_avg", "surface_ms_avg", "opaque_submit_ms_avg", "vector_submit_ms_avg", "overlay_ms_avg",
		"output_edges_avg", "render_jobs_avg",
	}); err != nil {
		_ = file.Close()
		return nil, fmt.Errorf("write telemetry header: %w", err)
	}
	return recorder, nil
}

// Observe accepts one completed frame. It performs arithmetic only except at
// the configured sample boundary.
func (r *Recorder) Observe(frame Frame) {
	if r == nil || r.err != nil {
		return
	}
	if frame.At.IsZero() {
		frame.At = time.Now()
	}
	if r.started.IsZero() {
		r.started, r.last = frame.At, frame.At
	}
	r.frames++
	r.drawSum += frame.DrawDuration
	if frame.DrawDuration > r.drawMax {
		r.drawMax = frame.DrawDuration
	}
	r.latest = frame
	r.sums.add(frame)
	if frame.At.Sub(r.last) >= r.interval {
		r.writeSample(frame.At)
	}
}

// Close flushes a final partial sample and closes the output file.
func (r *Recorder) Close() error {
	if r == nil || r.file == nil {
		return nil
	}
	if r.frames > 0 && r.err == nil {
		r.writeSample(r.latest.At)
	}
	r.writer.Flush()
	if r.err == nil {
		r.err = r.writer.Error()
	}
	closeErr := r.file.Close()
	r.file = nil
	if r.err != nil {
		return r.err
	}
	return closeErr
}

func (r *Recorder) writeSample(now time.Time) {
	if r.frames == 0 || r.err != nil {
		return
	}
	frames := float64(r.frames)
	record := []string{
		formatFloat(now.Sub(r.started).Seconds()), formatFloat(now.Sub(r.last).Seconds()), strconv.Itoa(r.frames),
		formatFloat(float64(r.drawSum) / float64(time.Millisecond) / frames), formatFloat(float64(r.drawMax) / float64(time.Millisecond)),
		formatFloat(float64(r.sums.updateCalls) / frames), formatFloat(float64(r.sums.updateDuration) / float64(time.Millisecond) / frames), formatFloat(durationPerCall(r.sums.updateDuration, r.sums.updateCalls)), formatFloat(r.sums.actualFPS / frames), formatFloat(r.sums.actualTPS / frames),
		r.latest.Flow, r.latest.MissionPhase, r.latest.View, strconv.Itoa(r.latest.Realism),
		formatFloat(float64(r.sums.activeTiles) / frames), formatFloat(float64(r.sums.features) / frames), formatFloat(float64(r.sums.candidates) / frames), formatFloat(float64(r.sums.depthDomains) / frames),
		formatFloat(float64(r.sums.depthFaces) / frames), formatFloat(float64(r.sums.depthTriangles) / frames), formatFloat(float64(r.sums.depthTested) / frames), formatFloat(float64(r.sums.depthWritten) / frames), formatFloat(float64(r.sums.lineSamples) / frames),
		formatFloat(float64(r.sums.prepareDuration) / float64(time.Millisecond) / frames), formatFloat(float64(r.sums.backgroundDuration) / float64(time.Millisecond) / frames), formatFloat(r.sums.depthMS / frames), formatFloat(r.sums.geometryMS / frames), formatFloat(float64(r.sums.surfaceDuration) / float64(time.Millisecond) / frames), formatFloat(r.sums.opaqueMS / frames), formatFloat(r.sums.vectorMS / frames), formatFloat(float64(r.sums.overlayDuration) / float64(time.Millisecond) / frames),
		formatFloat(float64(r.sums.outputEdges) / frames), formatFloat(float64(r.sums.renderJobs) / frames),
	}
	if err := r.writer.Write(record); err != nil {
		r.err = err
		return
	}
	// Sampling is infrequent, so flushing here keeps an interrupted development
	// run useful while avoiding all file I/O in ordinary frame rendering.
	r.writer.Flush()
	if err := r.writer.Error(); err != nil {
		r.err = err
		return
	}
	r.last = now
	r.frames = 0
	r.drawSum, r.drawMax = 0, 0
	r.sums = frameSums{}
}

func (s *frameSums) add(frame Frame) {
	s.updateCalls += int64(frame.UpdateCalls)
	s.updateDuration += frame.UpdateDuration
	s.prepareDuration += frame.PrepareDuration
	s.backgroundDuration += frame.BackgroundDuration
	s.surfaceDuration += frame.SurfaceDuration
	s.overlayDuration += frame.OverlayDuration
	s.actualFPS += frame.ActualFPS
	s.actualTPS += frame.ActualTPS
	s.activeTiles += int64(frame.ActiveTiles)
	s.features += int64(frame.Features)
	s.candidates += int64(frame.Candidates)
	s.depthDomains += int64(frame.DepthDomains)
	s.depthFaces += int64(frame.DepthFaces)
	s.depthTriangles += int64(frame.DepthTriangles)
	s.depthTested += int64(frame.DepthTested)
	s.depthWritten += int64(frame.DepthWritten)
	s.lineSamples += int64(frame.LineSamples)
	s.depthMS += frame.DepthMS
	s.geometryMS += frame.GeometryMS
	s.opaqueMS += frame.OpaqueMS
	s.vectorMS += frame.VectorMS
	s.outputEdges += int64(frame.OutputEdges)
	s.renderJobs += int64(frame.RenderJobs)
}

func formatFloat(value float64) string { return strconv.FormatFloat(value, 'f', 3, 64) }

func durationPerCall(duration time.Duration, calls int64) float64 {
	if calls == 0 {
		return 0
	}
	return float64(duration) / float64(time.Millisecond) / float64(calls)
}
