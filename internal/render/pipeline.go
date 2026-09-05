// Package render transforms wireframe models into clipped screen-space lines.
package render

import (
	"github.com/edwardwillis/starwars-vector-game/internal/math3d"
	"github.com/edwardwillis/starwars-vector-game/internal/model"
	"math"
	"strings"
)

// Line is a visible screen-space line segment.
type Line struct {
	X1 float64
	Y1 float64
	X2 float64
	Y2 float64
}

type Point struct {
	X     float64
	Y     float64
	Depth float64
}

// Stats records work performed by one or more Render calls. A pointer can be
// attached to Pipeline during development; nil keeps the hot path lightweight.
type Stats struct {
	InputVertices, TransformedVertices int
	InputEdges, OutputEdges            int
	TinyEdges                          int
	InputFaces                         int
	BackfaceRejected, PolicyRejected  int
	DepthRejected, ClippedEdges        int
	ObjectsInput, ObjectsCulled        int
	ObjectsVisible                     int
}

// Ray describes a world-space half-line produced by a screen-space aim point.
type Ray struct {
	Origin    math3d.Vec3
	Direction math3d.Vec3
}

// Culler can remove edges after world and camera transformations. A nil Culler
// keeps every edge, matching the original arcade-like wireframe appearance.
type Culler interface {
	Cull(verts []math3d.Vec3, edges []model.Edge) []model.Edge
}

// Stage is a composable topology stage. Stages run after world/view
// transformation and before clipping and projection.
type Stage interface {
	Name() string
	Process(verts []math3d.Vec3, mesh model.Model, edges []model.Edge) []model.Edge
}

type stageFunc struct {
	name string
	fn   func([]math3d.Vec3, model.Model, []model.Edge) []model.Edge
}

func (s stageFunc) Name() string { return s.name }
func (s stageFunc) Process(v []math3d.Vec3, m model.Model, e []model.Edge) []model.Edge {
	return s.fn(v, m, e)
}

// ProfileNames are the progressively more realistic built-in render modes.
const (
	ProfileArcade     = "arcade"
	ProfileCulled     = "culled"
	ProfileHiddenLine = "hidden-line"
	ProfileDepthCue   = "depth-cue"
	ProfileMaximum    = "maximum"
)

func StagesForProfile(name string) []Stage {
	name = strings.ToLower(name)
	name = strings.TrimPrefix(name, "builtin/")
	switch name {
	case ProfileArcade:
		return []Stage{BackfaceStage()}
	case ProfileCulled:
		return []Stage{BackfaceStage()}
	case ProfileHiddenLine:
		return []Stage{BackfaceStage(), HiddenLineStage()}
	case ProfileDepthCue:
		return []Stage{BackfaceStage(), HiddenLineStage(), DepthCueStage()}
	case ProfileMaximum:
		return []Stage{BackfaceStage(), HiddenLineStage(), DepthCueStage()}
	default:
		return nil
	}
}

func BackfaceStage() Stage { return stageFunc{"backface-culling", backfaceCull} }
func HiddenLineStage() Stage {
	return stageFunc{"hidden-line-removal", removeInternalEdges}
}
func DepthCueStage() Stage {
	return stageFunc{"depth-cue", func(v []math3d.Vec3, m model.Model, e []model.Edge) []model.Edge { return e }}
}

// removeInternalEdges removes edges between coplanar faces. These are usually
// triangulation/construction seams and do not contribute to a clean vector
// silhouette. True inter-object hidden-line removal is added by the later depth
// resolver; this cheap policy is useful at all higher profiles.
func removeInternalEdges(v []math3d.Vec3, mesh model.Model, edges []model.Edge) []model.Edge {
	out := make([]model.Edge, 0, len(edges))
	for _, edge := range edges {
		if edge.Kind == model.EdgeInternal { continue }
		out = append(out, edge)
	}
	return out
}

func backfaceCull(verts []math3d.Vec3, mesh model.Model, edges []model.Edge) []model.Edge {
	if len(mesh.Faces) == 0 {
		return edges
	}
	prepared := model.Prepare(mesh)
	front := make([]bool, len(mesh.Faces))
	for faceIndex, face := range prepared.Faces {
		if len(face.Vertices) < 3 {
			continue
		}
		n := face.Normal
		if n.Length() <= 1e-9 {
			a, b, c := verts[face.Vertices[0]], verts[face.Vertices[1]], verts[face.Vertices[2]]
			n = b.Sub(a).Cross(c.Sub(a)).Normalize()
		}
		center := math3d.Vec3{}
		for _, vertex := range face.Vertices { center = center.Add(verts[vertex]) }
		center = center.Scale(1 / float64(len(face.Vertices)))
		front[faceIndex] = n.Dot(center.Scale(-1)) > 1e-9 || face.DoubleSided
	}
	out := make([]model.Edge, 0, len(prepared.Topology.Edges))
	for _, edge := range prepared.Topology.Edges {
		// Internal diagonals exist only to make non-planar authored surfaces
		// explicit triangles. They must never become visible vector strokes,
		// including in the deliberately sparse arcade profile.
		if edge.Kind == model.EdgeInternal {
			continue
		}
		adjacent := edge.AdjacentFaces
		if len(adjacent) == 0 && edge.FaceA < 0 && edge.FaceB < 0 {
			out = append(out, edge)
			continue
		}
		visible := false
		for _, faceIndex := range adjacent {
			if faceIndex >= 0 && faceIndex < len(front) && front[faceIndex] {
				visible = true
				break
			}
		}
		if len(adjacent) == 0 {
			visible = (edge.FaceA >= 0 && front[edge.FaceA]) || (edge.FaceB >= 0 && front[edge.FaceB])
		}
		if visible {
			out = append(out, edge)
		}
	}
	return out
}

// Pipeline contains the configurable stages of the wireframe renderer.
type Pipeline struct {
	Width      int
	Height     int
	Near       float64
	Far        float64
	View       math3d.Mat4
	Projection math3d.Mat4
	Culler     Culler
	Stages     []Stage
	MinLinePixels float64
	DepthBias  float64
	Stats      *Stats
}

func NewPipeline(width, height int, verticalFOV, near, far float64) Pipeline {
	return Pipeline{
		Width:      width,
		Height:     height,
		Near:       near,
		Far:        far,
		View:       math3d.Identity(),
		Projection: math3d.Perspective(verticalFOV, float64(width)/float64(height), near, far),
		DepthBias:  0.08,
	}
}

// Render transforms a model into visible screen-space lines. The camera uses a
// right-handed coordinate system and looks down negative Z.
func (p Pipeline) Render(mesh model.Model, world math3d.Mat4) []Line {
	return p.renderLines(mesh, world, nil, 0)
}

// RenderWithDepth applies the same vector pipeline while rejecting line
// segments whose midpoint is behind a previously rasterized surface.
func (p Pipeline) RenderWithDepth(mesh model.Model, world math3d.Mat4, depth *DepthBuffer) []Line {
	return p.RenderWithDepthOwned(mesh, world, depth, 0)
}

// RenderWithDepthOwned leaves an object's own depth samples out of the
// occlusion query. Callers can assign distinct owners to separate physical
// parts of one object, allowing a nearer panel to hide a rear body while a
// part's own structural edges remain stable.
func (p Pipeline) RenderWithDepthOwned(mesh model.Model, world math3d.Mat4, depth *DepthBuffer, owner uint64) []Line {
	return p.renderLines(mesh, world, depth, owner)
}

func (p Pipeline) renderLines(mesh model.Model, world math3d.Mat4, depth *DepthBuffer, owner uint64) []Line {
	if p.Width <= 0 || p.Height <= 0 || p.Near <= 0 {
		return nil
	}

	viewWorld := p.View.Mul(world)
	prepared := model.Prepare(mesh)
	stageMesh := prepared
	if len(prepared.Faces) > 0 {
		stageMesh.Faces = append([]model.Face(nil), prepared.Faces...)
		for index := range stageMesh.Faces {
			stageMesh.Faces[index].Normal = viewWorld.TransformDirection(prepared.Faces[index].Normal).Normalize()
		}
	}
	verts := make([]math3d.Vec3, len(prepared.Verts))
	if p.Stats != nil { p.Stats.InputVertices += len(mesh.Verts); p.Stats.TransformedVertices += len(mesh.Verts) }
	if p.Stats != nil { p.Stats.InputFaces += len(mesh.Faces) }
	for index, vertex := range prepared.Verts {
		verts[index] = viewWorld.TransformPoint(vertex)
	}

	edges := mesh.Edges
	if p.Stats != nil { p.Stats.InputEdges += len(edges) }
	if p.Culler != nil {
		edges = p.Culler.Cull(verts, edges)
	}
	for _, stage := range p.Stages {
		if stage != nil {
			before := len(edges)
			edges = stage.Process(verts, stageMesh, edges)
			if p.Stats != nil && before > len(edges) {
				switch stage.Name() {
				case "backface-culling":
					p.Stats.BackfaceRejected += before - len(edges)
				default:
					p.Stats.PolicyRejected += before - len(edges)
				}
			}
		}
	}

	lines := make([]Line, 0, len(edges))
	for _, edge := range edges {
		if edge.A < 0 || edge.A >= len(verts) || edge.B < 0 || edge.B >= len(verts) {
			continue
		}

		originalA, originalB := verts[edge.A], verts[edge.B]
		a, b, visible := clipNear(originalA, originalB, p.Near)
		if !visible {
			continue
		}
		a, b, visible = clipFar(a, b, p.Far)
		if !visible {
			continue
		}
		if p.Stats != nil && (a != originalA || b != originalB) {
			p.Stats.ClippedEdges++
		}
		depthA, depthB := -a.Z, -b.Z

		a = p.Projection.TransformPoint(a)
		b = p.Projection.TransformPoint(b)
		line := Line{
			X1: (a.X + 1) * 0.5 * float64(p.Width),
			Y1: (1 - a.Y) * 0.5 * float64(p.Height),
			X2: (b.X + 1) * 0.5 * float64(p.Width),
			Y2: (1 - b.Y) * 0.5 * float64(p.Height),
		}
		if clipped, ok := clipScreen(line, float64(p.Width), float64(p.Height)); ok {
			segments := []Line{clipped}
			if depth != nil {
				segments = visibleDepthSegments(clipped, depthA, depthB, depth, owner, p.DepthBias)
				if len(segments) == 0 && p.Stats != nil { p.Stats.DepthRejected++ }
			}
			for _, segment := range segments {
				if p.MinLinePixels > 0 && math.Hypot(segment.X2-segment.X1, segment.Y2-segment.Y1) < p.MinLinePixels {
					if p.Stats != nil { p.Stats.TinyEdges++ }
					continue
				}
				lines = append(lines, segment)
			}
		}
	}
	if p.Stats != nil { p.Stats.OutputEdges += len(lines) }
	return lines
}

// visibleDepthSegments samples a projected edge against the CPU depth surface
// and returns visible intervals. Sampling keeps final drawing vector-based while
// allowing a line to disappear only where it passes behind another surface.
func visibleDepthSegments(line Line, depthA, depthB float64, depth *DepthBuffer, owner uint64, baseBias float64) []Line {
	length := math.Hypot(line.X2-line.X1, line.Y2-line.Y1)
	samples := int(math.Ceil(length / 6))
	if samples < 4 { samples = 4 }
	if samples > 64 { samples = 64 }
	visibleAt := func(t float64) bool {
		x := int(math.Round(line.X1 + (line.X2-line.X1)*t))
		y := int(math.Round(line.Y1 + (line.Y2-line.Y1)*t))
		// Perspective-correct interpolation matches the reciprocal-depth
		// interpolation used by RasterizeDepth. Linear world-depth interpolation
		// would make oblique edges appear artificially behind their own surfaces.
		lineDepth := 1 / ((1/depthA)*(1-t) + (1/depthB)*t)
		// Use a small relative bias: the line is normally coplanar with the
		// surface that produced the depth sample, and numerical/raster coverage
		// error should not make that structural edge sparkle or disappear.
		bias := math.Max(baseBias, lineDepth*0.003)
		return depth.nearestOtherAt(x, y, 1, owner)+bias >= lineDepth
	}
	segments := make([]Line, 0, samples)
	runStart := 0.0
	running := false
	for index := 0; index <= samples; index++ {
		t := float64(index) / float64(samples)
		visible := visibleAt(t)
		if visible && !running {
			runStart, running = t, true
		}
		if !visible && running {
			segments = append(segments, interpolateLine(line, runStart, t))
			running = false
		}
	}
	if running { segments = append(segments, interpolateLine(line, runStart, 1)) }
	return segments
}

func interpolateLine(line Line, start, end float64) Line {
	return Line{
		X1: line.X1 + (line.X2-line.X1)*start,
		Y1: line.Y1 + (line.Y2-line.Y1)*start,
		X2: line.X1 + (line.X2-line.X1)*end,
		Y2: line.Y1 + (line.Y2-line.Y1)*end,
	}
}

// ProjectPoint transforms a world-space point through the active camera and
// perspective matrices. It reports false for points behind the near plane or
// outside the screen.
func (p Pipeline) ProjectPoint(world math3d.Vec3) (Point, bool) {
	if p.Width <= 0 || p.Height <= 0 || p.Near <= 0 {
		return Point{}, false
	}
	cameraPoint := p.View.TransformPoint(world)
	if cameraPoint.Z > -p.Near || (p.Far > p.Near && cameraPoint.Z < -p.Far) {
		return Point{}, false
	}
	projected := p.Projection.TransformPoint(cameraPoint)
	if projected.X < -1 || projected.X > 1 || projected.Y < -1 || projected.Y > 1 {
		return Point{}, false
	}
	return Point{
		X:     (projected.X + 1) * 0.5 * float64(p.Width),
		Y:     (1 - projected.Y) * 0.5 * float64(p.Height),
		Depth: -cameraPoint.Z,
	}, true
}

func clipFar(a, b math3d.Vec3, far float64) (math3d.Vec3, math3d.Vec3, bool) {
	if far <= 0 {
		return a, b, true
	}
	planeZ := -far
	aInside, bInside := a.Z >= planeZ, b.Z >= planeZ
	if !aInside && !bInside {
		return math3d.Vec3{}, math3d.Vec3{}, false
	}
	if aInside && bInside {
		return a, b, true
	}
	t := (planeZ - a.Z) / (b.Z - a.Z)
	intersection := a.Add(b.Sub(a).Scale(t))
	if !aInside {
		a = intersection
	} else {
		b = intersection
	}
	return a, b, true
}

// ScreenRay converts a logical screen position into a world-space camera ray.
// View must be a rigid camera transform and Projection a perspective matrix.
func (p Pipeline) ScreenRay(x, y float64) (Ray, bool) {
	if p.Width <= 0 || p.Height <= 0 || p.Projection[0][0] == 0 || p.Projection[1][1] == 0 {
		return Ray{}, false
	}
	ndcX := 2*x/float64(p.Width) - 1
	ndcY := 1 - 2*y/float64(p.Height)
	cameraDirection := math3d.Vec3{
		X: ndcX / p.Projection[0][0],
		Y: ndcY / p.Projection[1][1],
		Z: -1,
	}.Normalize()

	// A rigid view's inverse rotation is its upper-left transpose.
	worldDirection := math3d.Vec3{
		X: p.View[0][0]*cameraDirection.X + p.View[1][0]*cameraDirection.Y + p.View[2][0]*cameraDirection.Z,
		Y: p.View[0][1]*cameraDirection.X + p.View[1][1]*cameraDirection.Y + p.View[2][1]*cameraDirection.Z,
		Z: p.View[0][2]*cameraDirection.X + p.View[1][2]*cameraDirection.Y + p.View[2][2]*cameraDirection.Z,
	}.Normalize()
	translation := math3d.Vec3{X: p.View[0][3], Y: p.View[1][3], Z: p.View[2][3]}
	origin := math3d.Vec3{
		X: -(p.View[0][0]*translation.X + p.View[1][0]*translation.Y + p.View[2][0]*translation.Z),
		Y: -(p.View[0][1]*translation.X + p.View[1][1]*translation.Y + p.View[2][1]*translation.Z),
		Z: -(p.View[0][2]*translation.X + p.View[1][2]*translation.Y + p.View[2][2]*translation.Z),
	}
	return Ray{Origin: origin, Direction: worldDirection}, true
}

func clipNear(a, b math3d.Vec3, near float64) (math3d.Vec3, math3d.Vec3, bool) {
	planeZ := -near
	aInside := a.Z <= planeZ
	bInside := b.Z <= planeZ
	if !aInside && !bInside {
		return math3d.Vec3{}, math3d.Vec3{}, false
	}
	if aInside && bInside {
		return a, b, true
	}

	t := (planeZ - a.Z) / (b.Z - a.Z)
	intersection := a.Add(b.Sub(a).Scale(t))
	if !aInside {
		a = intersection
	} else {
		b = intersection
	}
	return a, b, true
}

const (
	clipLeft = 1 << iota
	clipRight
	clipTop
	clipBottom
)

func clipScreen(line Line, width, height float64) (Line, bool) {
	for {
		code1 := clipCode(line.X1, line.Y1, width, height)
		code2 := clipCode(line.X2, line.Y2, width, height)
		if code1|code2 == 0 {
			return line, true
		}
		if code1&code2 != 0 {
			return Line{}, false
		}

		code := code1
		if code == 0 {
			code = code2
		}
		x, y := 0.0, 0.0
		switch {
		case code&clipTop != 0:
			y = 0
			x = line.X1 + (line.X2-line.X1)*(y-line.Y1)/(line.Y2-line.Y1)
		case code&clipBottom != 0:
			y = height
			x = line.X1 + (line.X2-line.X1)*(y-line.Y1)/(line.Y2-line.Y1)
		case code&clipRight != 0:
			x = width
			y = line.Y1 + (line.Y2-line.Y1)*(x-line.X1)/(line.X2-line.X1)
		case code&clipLeft != 0:
			x = 0
			y = line.Y1 + (line.Y2-line.Y1)*(x-line.X1)/(line.X2-line.X1)
		}

		if code == code1 {
			line.X1, line.Y1 = x, y
		} else {
			line.X2, line.Y2 = x, y
		}
	}
}

func clipCode(x, y, width, height float64) int {
	code := 0
	if x < 0 {
		code |= clipLeft
	} else if x > width {
		code |= clipRight
	}
	if y < 0 {
		code |= clipTop
	} else if y > height {
		code |= clipBottom
	}
	return code
}
