package render

import (
	"math"
	"testing"

	"github.com/edwardwillis/starwars-vector-game/internal/math3d"
	"github.com/edwardwillis/starwars-vector-game/internal/model"
)

func TestRasterizeDepthRecordsNearestCubeSurface(t *testing.T) {
	pipeline := NewPipeline(100, 100, math.Pi/2, 0.1, 100)
	buffer := NewDepthBuffer(100, 100)
	pipeline.RasterizeDepth(model.Cube(2), math3d.Translation(0, 0, -5), buffer)
	depth := buffer.depthAt(50, 50)
	if math.IsInf(depth, 1) {
		t.Fatal("cube did not write a center depth sample")
	}
	if math.Abs(depth-4) > 0.15 {
		t.Fatalf("center depth=%v, want approximately 4", depth)
	}
}

func TestFineDepthCountersCanBeDisabled(t *testing.T) {
	pipeline := NewPipeline(100, 100, math.Pi/2, 0.1, 100)
	stats := Stats{}
	pipeline.Stats = &stats
	pipeline.RasterizeDepth(model.Cube(2), math3d.Translation(0, 0, -5), NewDepthBuffer(100, 100))
	if stats.DepthPixelsTested != 0 || stats.DepthPixelsWritten != 0 {
		t.Fatalf("disabled fine counters=%+v", stats)
	}
	pipeline.FineStats = true
	pipeline.RasterizeDepth(model.Cube(2), math3d.Translation(0, 0, -5), NewDepthBuffer(100, 100))
	if stats.DepthPixelsTested == 0 || stats.DepthPixelsWritten == 0 {
		t.Fatalf("enabled fine counters=%+v", stats)
	}

	line := model.Model{
		Verts: []math3d.Vec3{{X: -.5, Z: -6}, {X: .5, Z: -6}},
		Edges: []model.Edge{{A: 0, B: 1, Kind: model.EdgeDecorative}},
	}
	depth := NewDepthBuffer(100, 100)
	pipeline.RasterizeDepth(model.Cube(2), math3d.Translation(0, 0, -5), depth)
	pipeline.FineStats = false
	stats.LineDepthSamples = 0
	pipeline.RenderWithDepth(line, math3d.Identity(), depth)
	if stats.LineDepthSamples != 0 {
		t.Fatalf("disabled fine line counters=%+v", stats)
	}
	pipeline.FineStats = true
	pipeline.RenderWithDepth(line, math3d.Identity(), depth)
	if stats.LineDepthSamples == 0 {
		t.Fatalf("enabled fine line counters=%+v", stats)
	}
}

func TestDepthBufferClearOnlyResetsWrittenPixels(t *testing.T) {
	buffer := NewDepthBuffer(16, 16)
	if !buffer.writeOwned(3, 4, 7, 9) || len(buffer.touched) != 1 {
		t.Fatalf("first write touched %d pixels", len(buffer.touched))
	}
	if !buffer.writeOwned(3, 4, 6, 10) || len(buffer.touched) != 1 {
		t.Fatalf("replacement write touched %d pixels, want one", len(buffer.touched))
	}
	buffer.Clear()
	if len(buffer.touched) != 0 || !math.IsInf(buffer.depthAt(3, 4), 1) || buffer.Owners[4*buffer.Width+3] != 0 {
		t.Fatalf("sparse clear left stale depth state: depth=%v owner=%d touched=%d", buffer.depthAt(3, 4), buffer.Owners[4*buffer.Width+3], len(buffer.touched))
	}
}

func TestRenderWithDepthRejectsLineBehindSurface(t *testing.T) {
	pipeline := NewPipeline(100, 100, math.Pi/2, 0.1, 100)
	buffer := NewDepthBuffer(100, 100)
	pipeline.RasterizeDepth(model.Cube(2), math3d.Translation(0, 0, -5), buffer)
	line := model.Model{
		Verts: []math3d.Vec3{{X: -.5, Z: -6}, {X: .5, Z: -6}},
		Edges: []model.Edge{{A: 0, B: 1, Kind: model.EdgeDecorative}},
	}
	if got := pipeline.RenderWithDepth(line, math3d.Identity(), buffer); len(got) != 0 {
		t.Fatalf("behind-surface line produced %d segments", len(got))
	}
	line.Verts[0].Z, line.Verts[1].Z = -3, -3
	if got := pipeline.RenderWithDepth(line, math3d.Identity(), buffer); len(got) != 1 {
		t.Fatalf("front line produced %d segments, want 1", len(got))
	}
}

func TestClipPolygonZClipsNearPlane(t *testing.T) {
	polygon := []math3d.Vec3{{X: -1, Z: -.5}, {X: 1, Z: -2}, {Y: 1, Z: -2}}
	clipped := clipPolygonZ(polygon, -1, true)
	if len(clipped) != 4 {
		t.Fatalf("clipped polygon has %d vertices, want 4", len(clipped))
	}
	for _, vertex := range clipped {
		if vertex.Z > -1+1e-9 {
			t.Fatalf("vertex remains behind near plane: %+v", vertex)
		}
	}
}

func TestRenderWithDepthSplitsPartiallyOccludedLine(t *testing.T) {
	pipeline := NewPipeline(100, 100, math.Pi/2, 0.1, 100)
	buffer := NewDepthBuffer(100, 100)
	buffer.write(50, 50, 4)
	line := model.Model{
		Verts: []math3d.Vec3{{X: -4, Z: -6}, {X: 4, Z: -6}},
		Edges: []model.Edge{{A: 0, B: 1, Kind: model.EdgeDecorative}},
	}
	if got := pipeline.RenderWithDepth(line, math3d.Identity(), buffer); len(got) != 2 {
		t.Fatalf("partially occluded line produced %d segments, want 2", len(got))
	}
}

func TestOwnedDepthDoesNotSelfOccludeStructuralEdges(t *testing.T) {
	pipeline := NewPipeline(100, 100, math.Pi/2, 0.1, 100)
	buffer := NewDepthBuffer(100, 100)
	pipeline.RasterizeDepthOwned(model.Cube(2), math3d.Translation(0, 0, -5), buffer, 7)
	line := model.Model{Verts: []math3d.Vec3{{X: -.5, Z: -6}, {X: .5, Z: -6}}, Edges: []model.Edge{{A: 0, B: 1}}}
	if got := pipeline.RenderWithDepthOwned(line, math3d.Identity(), buffer, 7); len(got) != 1 {
		t.Fatalf("same-owner line produced %d segments, want 1", len(got))
	}
	if got := pipeline.RenderWithDepthOwned(line, math3d.Identity(), buffer, 8); len(got) != 0 {
		t.Fatalf("other-owner line produced %d segments, want 0", len(got))
	}
}
