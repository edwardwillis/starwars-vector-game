package render

import (
	"math"

	"github.com/edwardwillis/starwars-vector-game/internal/math3d"
	"github.com/edwardwillis/starwars-vector-game/internal/model"
)

// DepthBuffer is a small CPU depth surface used only by surface-aware vector
// profiles. Infinite values represent pixels with no occluding surface;
// Owners lets compound objects avoid erasing their own structural edges.
type DepthBuffer struct {
	Width, Height int
	Values        []float64
	Owners        []uint64
	touched       []int
}

func NewDepthBuffer(width, height int) *DepthBuffer {
	size := maxInt(0, width*height)
	buffer := &DepthBuffer{Width: width, Height: height, Values: make([]float64, size), Owners: make([]uint64, size)}
	for index := range buffer.Values {
		buffer.Values[index] = math.Inf(1)
	}
	return buffer
}

func (buffer *DepthBuffer) Clear() {
	// Local depth domains reuse one full-size buffer. Reset only pixels written
	// by the preceding domain so several small objects do not each incur a
	// full-screen clear.
	for _, index := range buffer.touched {
		buffer.Values[index] = math.Inf(1)
		buffer.Owners[index] = 0
	}
	buffer.touched = buffer.touched[:0]
}

func (buffer *DepthBuffer) depthAt(x, y int) float64 {
	if buffer == nil || x < 0 || y < 0 || x >= buffer.Width || y >= buffer.Height {
		return math.Inf(1)
	}
	return buffer.Values[y*buffer.Width+x]
}

// nearestAt returns the closest finite sample in a small neighborhood. Vector
// edges frequently lie exactly on polygon boundaries, where a single-pixel
// depth raster can otherwise leave alternating holes as the camera moves.
func (buffer *DepthBuffer) nearestAt(x, y, radius int) float64 {
	nearest := math.Inf(1)
	for offsetY := -radius; offsetY <= radius; offsetY++ {
		for offsetX := -radius; offsetX <= radius; offsetX++ {
			value := buffer.depthAt(x+offsetX, y+offsetY)
			if value < nearest {
				nearest = value
			}
		}
	}
	return nearest
}

func (buffer *DepthBuffer) nearestOtherAt(x, y, radius int, owner uint64) float64 {
	nearest := math.Inf(1)
	for offsetY := -radius; offsetY <= radius; offsetY++ {
		for offsetX := -radius; offsetX <= radius; offsetX++ {
			px, py := x+offsetX, y+offsetY
			if px < 0 || py < 0 || px >= buffer.Width || py >= buffer.Height {
				continue
			}
			index := py*buffer.Width + px
			if owner != 0 && buffer.Owners[index] == owner {
				continue
			}
			if buffer.Values[index] < nearest {
				nearest = buffer.Values[index]
			}
		}
	}
	return nearest
}

func (buffer *DepthBuffer) write(x, y int, depth float64) {
	buffer.writeOwned(x, y, depth, 0)
}

func (buffer *DepthBuffer) writeOwned(x, y int, depth float64, owner uint64) bool {
	if x < 0 || y < 0 || x >= buffer.Width || y >= buffer.Height {
		return false
	}
	index := y*buffer.Width + x
	if depth < buffer.Values[index] {
		if math.IsInf(buffer.Values[index], 1) {
			buffer.touched = append(buffer.touched, index)
		}
		buffer.Values[index], buffer.Owners[index] = depth, owner
		return true
	}
	return false
}

// RasterizeDepth adds visible model faces to the depth surface. It deliberately
// uses only already-authored faces; decorative line art cannot occlude.
func (p Pipeline) RasterizeDepth(mesh model.Model, world math3d.Mat4, buffer *DepthBuffer) {
	p.RasterizeDepthOwned(mesh, world, buffer, 0)
}

func (p Pipeline) RasterizeDepthOwned(mesh model.Model, world math3d.Mat4, buffer *DepthBuffer, owner uint64) {
	if buffer == nil || len(mesh.Faces) == 0 {
		return
	}
	geometry := p.PrepareGeometry(mesh, world, true)
	p.RasterizePreparedDepthOwned(&geometry, buffer, owner)
}

// RasterizePreparedDepthOwned consumes the shared projected face triangles;
// it performs no model traversal, camera transform, clipping, or projection.
func (p Pipeline) RasterizePreparedDepthOwned(geometry *PreparedGeometry, buffer *DepthBuffer, owner uint64) {
	if geometry == nil || buffer == nil {
		return
	}
	lastFace := -1
	for _, triangle := range geometry.Triangles {
		if triangle.Face != lastFace {
			if p.Stats != nil {
				p.Stats.DepthFacesSubmitted++
			}
			lastFace = triangle.Face
		}
		p.rasterizePreparedTriangle(triangle.A, triangle.B, triangle.C, buffer, owner)
	}
}

// clipPolygonZ clips a camera-space polygon against a depth plane. The
// generated fragments are used only by the invisible depth pass, so clipping
// never introduces visible wireframe diagonals.
func clipPolygonZ(polygon []math3d.Vec3, plane float64, keepBelow bool) []math3d.Vec3 {
	if len(polygon) == 0 {
		return nil
	}
	inside := func(point math3d.Vec3) bool {
		if keepBelow {
			return point.Z <= plane
		}
		return point.Z >= plane
	}
	out := make([]math3d.Vec3, 0, len(polygon)+1)
	previous := polygon[len(polygon)-1]
	previousInside := inside(previous)
	for _, current := range polygon {
		currentInside := inside(current)
		if currentInside != previousInside {
			t := (plane - previous.Z) / (current.Z - previous.Z)
			out = append(out, previous.Add(current.Sub(previous).Scale(t)))
		}
		if currentInside {
			out = append(out, current)
		}
		previous, previousInside = current, currentInside
	}
	return out
}

func (p Pipeline) rasterizeTriangle(a, b, c math3d.Vec3, buffer *DepthBuffer, owner uint64) {
	if a.Z > -p.Near || b.Z > -p.Near || c.Z > -p.Near {
		return
	}
	project := func(vertex math3d.Vec3) Point {
		projected := p.Projection.TransformPoint(vertex)
		return Point{X: (projected.X + 1) * .5 * float64(p.Width), Y: (1 - projected.Y) * .5 * float64(p.Height), Depth: -vertex.Z}
	}
	p.rasterizePreparedTriangle(project(a), project(b), project(c), buffer, owner)
}

func (p Pipeline) rasterizePreparedTriangle(a, b, c Point, buffer *DepthBuffer, owner uint64) {
	ax, ay := a.X, a.Y
	bx, by := b.X, b.Y
	cx, cy := c.X, c.Y
	minX := maxInt(0, int(math.Floor(minFloat(ax, minFloat(bx, cx)))))
	maxX := minInt(buffer.Width-1, int(math.Ceil(maxFloat(ax, maxFloat(bx, cx)))))
	minY := maxInt(0, int(math.Floor(minFloat(ay, minFloat(by, cy)))))
	maxY := minInt(buffer.Height-1, int(math.Ceil(maxFloat(ay, maxFloat(by, cy)))))
	denom := (by-cy)*(ax-cx) + (cx-bx)*(ay-cy)
	if math.Abs(denom) < 1e-9 {
		return
	}
	if p.Stats != nil {
		p.Stats.DepthTrianglesRasterized++
	}
	for y := minY; y <= maxY; y++ {
		for x := minX; x <= maxX; x++ {
			if p.FineStats && p.Stats != nil {
				p.Stats.DepthPixelsTested++
			}
			px, py := float64(x)+.5, float64(y)+.5
			w0 := ((by-cy)*(px-cx) + (cx-bx)*(py-cy)) / denom
			w1 := ((cy-ay)*(px-cx) + (ax-cx)*(py-cy)) / denom
			w2 := 1 - w0 - w1
			if w0 < 0 || w1 < 0 || w2 < 0 {
				continue
			}
			depth := 1 / (w0/a.Depth + w1/b.Depth + w2/c.Depth)
			if buffer.writeOwned(x, y, depth, owner) && p.FineStats && p.Stats != nil {
				p.Stats.DepthPixelsWritten++
			}
		}
	}
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
func minFloat(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}
func maxFloat(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}
