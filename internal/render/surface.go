package render

import (
	"image/color"
	"math"
	"sort"

	"github.com/edwardwillis/starwars-vector-game/internal/math3d"
	"github.com/edwardwillis/starwars-vector-game/internal/model"
)

// FlatTriangle is a screen-space filled primitive derived from already
// transformed, back-face-classified, and near/far-clipped model geometry.
type FlatTriangle struct {
	A, B, C     Point
	Color       color.RGBA
	TextureID   string
	UVs         [3]model.UV
	Translucent bool
}

func (triangle FlatTriangle) averageDepth() float64 {
	return (triangle.A.Depth + triangle.B.Depth + triangle.C.Depth) / 3
}

// AppendFlatTriangles reuses prepared surface results; it performs no model
// traversal, transformation, face classification, or clipping.
func AppendFlatTriangles(destination []FlatTriangle, geometry *PreparedGeometry, fill color.RGBA) []FlatTriangle {
	return AppendOpaqueTriangles(destination, geometry, fill, "")
}

// AppendOpaqueTriangles preserves source UVs for textured materials while
// retaining the existing white-pixel path for untextured fills.
func AppendOpaqueTriangles(destination []FlatTriangle, geometry *PreparedGeometry, fill color.RGBA, textureID string) []FlatTriangle {
	if geometry == nil || fill.A != 255 {
		return destination
	}
	return appendFilledTriangles(destination, geometry, fill, textureID, false)
}

// DirectionalLight is a camera-space, fixed directional light used for
// intentionally flat per-face shading. It is not a dynamic lighting system:
// callers provide the transformed world-space direction once per candidate,
// and all triangles belonging to a face retain one stable tint.
type DirectionalLight struct {
	Direction        math3d.Vec3
	Ambient, Diffuse float64
}

func (light DirectionalLight) Enabled() bool {
	return light.Direction.Length() > 1e-9 && (light.Ambient > 0 || light.Diffuse > 0)
}

// AppendDirectionalOpaqueTriangles retains the normal opaque submission path
// while tinting each face from a fixed directional light. It preserves the
// texture and batch identity, so shaded and unshaded surfaces still share the
// existing GPU texture batches.
func AppendDirectionalOpaqueTriangles(destination []FlatTriangle, geometry *PreparedGeometry, fill color.RGBA, textureID string, light DirectionalLight) []FlatTriangle {
	if geometry == nil || fill.A != 255 {
		return destination
	}
	if !light.Enabled() {
		return appendFilledTriangles(destination, geometry, fill, textureID, false)
	}
	light.Direction = light.Direction.Normalize()
	for _, triangle := range geometry.Triangles {
		if triangle.Face < 0 || triangle.Face >= len(geometry.Faces) || geometry.Faces[triangle.Face].OccluderOnly {
			continue
		}
		destination = append(destination, FlatTriangle{
			A: triangle.A, B: triangle.B, C: triangle.C,
			Color:     shadeFaceColor(fill, geometry.Faces[triangle.Face].Normal, light),
			TextureID: textureID, UVs: triangle.UVs,
		})
	}
	return destination
}

// AppendTranslucentTriangles reuses the same prepared geometry as opaque
// surfaces. Alpha blending happens only in the final ordered draw pass.
func AppendTranslucentTriangles(destination []FlatTriangle, geometry *PreparedGeometry, fill color.RGBA, textureID string) []FlatTriangle {
	if geometry == nil || fill.A == 0 || fill.A == 255 {
		return destination
	}
	return appendFilledTriangles(destination, geometry, fill, textureID, true)
}

func appendFilledTriangles(destination []FlatTriangle, geometry *PreparedGeometry, fill color.RGBA, textureID string, translucent bool) []FlatTriangle {
	for _, triangle := range geometry.Triangles {
		if triangle.Face < 0 || triangle.Face >= len(geometry.Faces) || geometry.Faces[triangle.Face].OccluderOnly {
			continue
		}
		destination = append(destination, FlatTriangle{
			A: triangle.A, B: triangle.B, C: triangle.C, Color: fill,
			TextureID: textureID, UVs: triangle.UVs, Translucent: translucent,
		})
	}
	return destination
}

func shadeFaceColor(fill color.RGBA, normal math3d.Vec3, light DirectionalLight) color.RGBA {
	intensity := light.Ambient
	if facing := normal.Normalize().Dot(light.Direction); facing > 0 {
		intensity += light.Diffuse * facing
	}
	return color.RGBA{
		R: scaleColorChannel(fill.R, intensity),
		G: scaleColorChannel(fill.G, intensity),
		B: scaleColorChannel(fill.B, intensity),
		A: fill.A,
	}
}

func scaleColorChannel(channel uint8, intensity float64) uint8 {
	return uint8(math.Min(255, math.Max(0, math.Round(float64(channel)*intensity))))
}

// SortFlatTriangles applies a stable painter order to explicitly filled
// geometry. Opaque and translucent triangles must share the same order so a
// near opaque surface can cover a farther translucent surface.
func SortFlatTriangles(triangles []FlatTriangle) {
	sort.SliceStable(triangles, func(first, second int) bool {
		firstDepth, secondDepth := triangles[first].averageDepth(), triangles[second].averageDepth()
		if firstDepth == secondDepth && triangles[first].Translucent != triangles[second].Translucent {
			return !triangles[first].Translucent
		}
		return firstDepth > secondDepth
	})
}
