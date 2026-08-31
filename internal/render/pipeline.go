// Package render transforms wireframe models into clipped screen-space lines.
package render

import (
	"github.com/edwardwillis/starwars-vector-game/internal/math3d"
	"github.com/edwardwillis/starwars-vector-game/internal/model"
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

// Pipeline contains the configurable stages of the wireframe renderer.
type Pipeline struct {
	Width      int
	Height     int
	Near       float64
	View       math3d.Mat4
	Projection math3d.Mat4
	Culler     Culler
}

func NewPipeline(width, height int, verticalFOV, near, far float64) Pipeline {
	return Pipeline{
		Width:      width,
		Height:     height,
		Near:       near,
		View:       math3d.Identity(),
		Projection: math3d.Perspective(verticalFOV, float64(width)/float64(height), near, far),
	}
}

// Render transforms a model into visible screen-space lines. The camera uses a
// right-handed coordinate system and looks down negative Z.
func (p Pipeline) Render(mesh model.Model, world math3d.Mat4) []Line {
	if p.Width <= 0 || p.Height <= 0 || p.Near <= 0 {
		return nil
	}

	viewWorld := p.View.Mul(world)
	verts := make([]math3d.Vec3, len(mesh.Verts))
	for index, vertex := range mesh.Verts {
		verts[index] = viewWorld.TransformPoint(vertex)
	}

	edges := mesh.Edges
	if p.Culler != nil {
		edges = p.Culler.Cull(verts, edges)
	}

	lines := make([]Line, 0, len(edges))
	for _, edge := range edges {
		if edge.A < 0 || edge.A >= len(verts) || edge.B < 0 || edge.B >= len(verts) {
			continue
		}

		a, b, visible := clipNear(verts[edge.A], verts[edge.B], p.Near)
		if !visible {
			continue
		}

		a = p.Projection.TransformPoint(a)
		b = p.Projection.TransformPoint(b)
		line := Line{
			X1: (a.X + 1) * 0.5 * float64(p.Width),
			Y1: (1 - a.Y) * 0.5 * float64(p.Height),
			X2: (b.X + 1) * 0.5 * float64(p.Width),
			Y2: (1 - b.Y) * 0.5 * float64(p.Height),
		}
		if clipped, ok := clipScreen(line, float64(p.Width), float64(p.Height)); ok {
			lines = append(lines, clipped)
		}
	}
	return lines
}

// ProjectPoint transforms a world-space point through the active camera and
// perspective matrices. It reports false for points behind the near plane or
// outside the screen.
func (p Pipeline) ProjectPoint(world math3d.Vec3) (Point, bool) {
	if p.Width <= 0 || p.Height <= 0 || p.Near <= 0 {
		return Point{}, false
	}
	cameraPoint := p.View.TransformPoint(world)
	if cameraPoint.Z > -p.Near {
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
