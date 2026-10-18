// telemetry-compare compares equivalent verified replay telemetry trials.
package main

import (
	"encoding/csv"
	"errors"
	"flag"
	"fmt"
	"io"
	"math"
	"os"
	"sort"
	"strconv"
	"strings"
)

type pathList []string

func (paths *pathList) String() string { return strings.Join(*paths, ";") }

func (paths *pathList) Set(value string) error {
	if value == "" {
		return errors.New("telemetry path must not be empty")
	}
	*paths = append(*paths, value)
	return nil
}

type metric struct {
	column string
	label  string
	unit   string
}

var metrics = []metric{
	{column: "draw_ms_avg", label: "Draw", unit: "ms/frame"},
	{column: "overlay_ms_avg", label: "Overlay", unit: "ms/frame"},
	{column: "geometry_ms_avg", label: "Geometry", unit: "ms/frame"},
	{column: "surface_ms_avg", label: "Surface", unit: "ms/frame"},
	{column: "depth_raster_ms_avg", label: "Depth raster", unit: "ms/frame"},
	{column: "vector_submit_ms_avg", label: "Vector submit", unit: "ms/frame"},
	{column: "opaque_submit_ms_avg", label: "Opaque submit", unit: "ms/frame"},
	{column: "update_ms_avg", label: "Update", unit: "ms/frame"},
	{column: "actual_fps_avg", label: "FPS", unit: "fps"},
	{column: "output_edges_avg", label: "Output edges", unit: "edges/frame"},
}

var requiredColumns = append([]string{
	"session_kind", "recording_id", "replay_verified", "simulation_tick_start", "simulation_tick_end", "frames", "view", "realism",
}, metricColumns()...)

func metricColumns() []string {
	columns := make([]string, 0, len(metrics))
	for _, metric := range metrics {
		columns = append(columns, metric.column)
	}
	return columns
}

type sample struct {
	start, end uint64
	frames     float64
	values     map[string]float64
}

type trial struct {
	path        string
	recordingID string
	view        string
	realism     string
	samples     []sample
	start, end  uint64
}

type trialSummary struct {
	path        string
	values      map[string]float64
	worstDrawMS float64
}

func main() {
	var baselinePaths, candidatePaths pathList
	flag.Var(&baselinePaths, "baseline", "baseline replay telemetry CSV (repeat for each trial)")
	flag.Var(&candidatePaths, "candidate", "candidate replay telemetry CSV (repeat for each trial)")
	flag.Parse()
	if err := compare(baselinePaths, candidatePaths, os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "telemetry comparison:", err)
		os.Exit(2)
	}
}

func compare(baselinePaths, candidatePaths []string, output io.Writer) error {
	if len(baselinePaths) == 0 || len(candidatePaths) == 0 {
		return errors.New("supply at least one -baseline and one -candidate CSV")
	}
	baselines, err := loadTrials(baselinePaths)
	if err != nil {
		return fmt.Errorf("load baseline: %w", err)
	}
	candidates, err := loadTrials(candidatePaths)
	if err != nil {
		return fmt.Errorf("load candidate: %w", err)
	}
	allTrials := append(append([]trial{}, baselines...), candidates...)
	recordingID, view, realism, err := sharedMetadata(allTrials)
	if err != nil {
		return err
	}
	start, end, err := sharedRange(allTrials)
	if err != nil {
		return err
	}
	baselineSummaries, err := summarizeTrials(baselines, start, end)
	if err != nil {
		return fmt.Errorf("summarize baseline: %w", err)
	}
	candidateSummaries, err := summarizeTrials(candidates, start, end)
	if err != nil {
		return fmt.Errorf("summarize candidate: %w", err)
	}

	fmt.Fprintf(output, "Verified replay benchmark comparison\n")
	fmt.Fprintf(output, "  Recording: %s\n", recordingID)
	fmt.Fprintf(output, "  Shared simulation tick interval: [%d, %d]\n", start, end)
	fmt.Fprintf(output, "  Presentation: view=%s realism=%s\n", view, realism)
	fmt.Fprintf(output, "  Trials: baseline=%d candidate=%d\n\n", len(baselines), len(candidates))
	fmt.Fprintln(output, "Metric                  Baseline median     Candidate median    Change")
	for _, metric := range metrics {
		baselineMedian := median(summariesForMetric(baselineSummaries, metric.column))
		candidateMedian := median(summariesForMetric(candidateSummaries, metric.column))
		change := candidateMedian - baselineMedian
		percent := 0.0
		if baselineMedian != 0 {
			percent = change / baselineMedian * 100
		}
		fmt.Fprintf(output, "%-22s %10.3f %-10s %10.3f %-10s %+7.3f (%+5.1f%%)\n", metric.label, baselineMedian, metric.unit, candidateMedian, metric.unit, change, percent)
	}
	fmt.Fprintln(output)
	fmt.Fprintf(output, "Worst sustained draw row: baseline %.3f ms/frame; candidate %.3f ms/frame\n", median(worstDraws(baselineSummaries)), median(worstDraws(candidateSummaries)))
	fmt.Fprintln(output, "Averages are weighted by frame count within the shared tick interval; samples crossing its boundary are weighted proportionally.")
	return nil
}

func loadTrials(paths []string) ([]trial, error) {
	trials := make([]trial, 0, len(paths))
	for _, path := range paths {
		trial, err := loadTrial(path)
		if err != nil {
			return nil, err
		}
		trials = append(trials, trial)
	}
	return trials, nil
}

func loadTrial(path string) (trial, error) {
	file, err := os.Open(path)
	if err != nil {
		return trial{}, fmt.Errorf("open %q: %w", path, err)
	}
	defer file.Close()
	records, err := csv.NewReader(file).ReadAll()
	if err != nil {
		return trial{}, fmt.Errorf("read %q: %w", path, err)
	}
	if len(records) < 2 {
		return trial{}, fmt.Errorf("%q contains no telemetry samples", path)
	}
	columns := make(map[string]int, len(records[0]))
	for index, name := range records[0] {
		columns[name] = index
	}
	for _, name := range requiredColumns {
		if _, ok := columns[name]; !ok {
			return trial{}, fmt.Errorf("%q is missing required %q column", path, name)
		}
	}
	result := trial{path: path, start: math.MaxUint64}
	for rowIndex, row := range records[1:] {
		if len(row) != len(records[0]) {
			return trial{}, fmt.Errorf("%q row %d has %d fields, want %d", path, rowIndex+2, len(row), len(records[0]))
		}
		value := func(name string) string { return row[columns[name]] }
		if value("session_kind") != "replay" || value("replay_verified") != "true" {
			return trial{}, fmt.Errorf("%q row %d is not a verified replay sample", path, rowIndex+2)
		}
		if result.recordingID == "" {
			result.recordingID, result.view, result.realism = value("recording_id"), value("view"), value("realism")
			if result.recordingID == "" {
				return trial{}, fmt.Errorf("%q row %d has an empty recording ID", path, rowIndex+2)
			}
		} else if result.recordingID != value("recording_id") || result.view != value("view") || result.realism != value("realism") {
			return trial{}, fmt.Errorf("%q changes recording or presentation metadata between samples", path)
		}
		start, err := parseUint(path, rowIndex+2, "simulation_tick_start", value("simulation_tick_start"))
		if err != nil {
			return trial{}, err
		}
		end, err := parseUint(path, rowIndex+2, "simulation_tick_end", value("simulation_tick_end"))
		if err != nil {
			return trial{}, err
		}
		if end < start {
			return trial{}, fmt.Errorf("%q row %d has descending simulation tick range [%d, %d]", path, rowIndex+2, start, end)
		}
		frames, err := parseFloat(path, rowIndex+2, "frames", value("frames"))
		if err != nil || frames <= 0 {
			return trial{}, fmt.Errorf("%q row %d has invalid frame count %q", path, rowIndex+2, value("frames"))
		}
		values := make(map[string]float64, len(metrics))
		for _, metric := range metrics {
			parsed, err := parseFloat(path, rowIndex+2, metric.column, value(metric.column))
			if err != nil {
				return trial{}, err
			}
			values[metric.column] = parsed
		}
		result.samples = append(result.samples, sample{start: start, end: end, frames: frames, values: values})
		result.start = min(result.start, start)
		result.end = max(result.end, end)
	}
	return result, nil
}

func parseUint(path string, row int, name, value string) (uint64, error) {
	parsed, err := strconv.ParseUint(value, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("%q row %d has invalid %s %q", path, row, name, value)
	}
	return parsed, nil
}

func parseFloat(path string, row int, name, value string) (float64, error) {
	parsed, err := strconv.ParseFloat(value, 64)
	if err != nil || math.IsNaN(parsed) || math.IsInf(parsed, 0) {
		return 0, fmt.Errorf("%q row %d has invalid %s %q", path, row, name, value)
	}
	return parsed, nil
}

func sharedMetadata(trials []trial) (recordingID, view, realism string, err error) {
	first := trials[0]
	for _, trial := range trials[1:] {
		if trial.recordingID != first.recordingID {
			return "", "", "", fmt.Errorf("recording IDs differ: %q uses %s, but %q uses %s", first.path, first.recordingID, trial.path, trial.recordingID)
		}
		if trial.view != first.view || trial.realism != first.realism {
			return "", "", "", fmt.Errorf("presentation differs: %q is view=%s realism=%s, but %q is view=%s realism=%s", first.path, first.view, first.realism, trial.path, trial.view, trial.realism)
		}
	}
	return first.recordingID, first.view, first.realism, nil
}

func sharedRange(trials []trial) (uint64, uint64, error) {
	start, end := uint64(0), uint64(math.MaxUint64)
	for index, trial := range trials {
		if index == 0 || trial.start > start {
			start = trial.start
		}
		if trial.end < end {
			end = trial.end
		}
	}
	if end <= start {
		return 0, 0, fmt.Errorf("trials have no shared advancing simulation-tick interval")
	}
	return start, end, nil
}

func summarizeTrials(trials []trial, start, end uint64) ([]trialSummary, error) {
	summaries := make([]trialSummary, 0, len(trials))
	for _, trial := range trials {
		summary, err := summarizeTrial(trial, start, end)
		if err != nil {
			return nil, err
		}
		summaries = append(summaries, summary)
	}
	return summaries, nil
}

func summarizeTrial(trial trial, start, end uint64) (trialSummary, error) {
	weighted := make(map[string]float64, len(metrics))
	weight := 0.0
	worst := -math.MaxFloat64
	for _, sample := range trial.samples {
		overlap := overlapFraction(sample.start, sample.end, start, end)
		if overlap == 0 {
			continue
		}
		sampleWeight := sample.frames * overlap
		weight += sampleWeight
		for _, metric := range metrics {
			weighted[metric.column] += sample.values[metric.column] * sampleWeight
		}
		worst = max(worst, sample.values["draw_ms_avg"])
	}
	if weight == 0 {
		return trialSummary{}, fmt.Errorf("%q has no samples within shared tick range [%d, %d]", trial.path, start, end)
	}
	for _, metric := range metrics {
		weighted[metric.column] /= weight
	}
	return trialSummary{path: trial.path, values: weighted, worstDrawMS: worst}, nil
}

func overlapFraction(sampleStart, sampleEnd, start, end uint64) float64 {
	if sampleEnd <= start || sampleStart >= end {
		return 0
	}
	overlapStart, overlapEnd := max(sampleStart, start), min(sampleEnd, end)
	if sampleEnd == sampleStart {
		return 1
	}
	return float64(overlapEnd-overlapStart) / float64(sampleEnd-sampleStart)
}

func summariesForMetric(summaries []trialSummary, metric string) []float64 {
	values := make([]float64, 0, len(summaries))
	for _, summary := range summaries {
		values = append(values, summary.values[metric])
	}
	return values
}

func worstDraws(summaries []trialSummary) []float64 {
	values := make([]float64, 0, len(summaries))
	for _, summary := range summaries {
		values = append(values, summary.worstDrawMS)
	}
	return values
}

func median(values []float64) float64 {
	sorted := append([]float64(nil), values...)
	sort.Float64s(sorted)
	middle := len(sorted) / 2
	if len(sorted)%2 == 1 {
		return sorted[middle]
	}
	return (sorted[middle-1] + sorted[middle]) / 2
}

func min[T ~uint64 | ~float64](first, second T) T {
	if first < second {
		return first
	}
	return second
}

func max[T ~uint64 | ~float64](first, second T) T {
	if first > second {
		return first
	}
	return second
}
