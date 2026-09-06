package model

import (
	"math"

	"github.com/edwardwillis/starwars-vector-game/internal/math3d"
)

const (
	xWingWingRootX           = 0.55
	xWingWingTipX            = 3.25
	xWingEngineMountX        = 1.02
	xWingEngineMountZ        = -0.55
	xWingCannonMountZ        = 0.02
	xWingCannonBarrelLength  = 1.60
	xWingCannonBarrelCenterZ = 0.25
	xWingCannonMountCenterZ  = -0.32
	xWingCannonMountDepth    = 0.40
	xWingCannonMuzzleZ       = xWingCannonMountZ + xWingCannonBarrelCenterZ + xWingCannonBarrelLength/2
)

// XWing returns a sparse Rebel fighter assembled from a hull, four reusable
// S-foil assemblies, engines, and wingtip cannons. +Z is the nose direction.
func XWing() Model {
	return Merge(XWingCore(), XWingFoils())
}

// XWingCore contains the fuselage and canopy without the four S-foil
// assemblies. Keeping these physical assemblies separate allows per-part
// depth ownership to hide the core behind a nearer foil during rotation.
func XWingCore() Model {
	return Merge(XWingCoreParts()...)
}

// XWingCoreParts returns the independently occluding fuselage and canopy
// solids used by scene composition.
func XWingCoreParts() []Model {
	return []Model{xWingFuselage(), xWingCanopy()}
}

// XWingFoils contains the four rotated wing, engine and cannon assemblies.
func XWingFoils() Model {
	return Merge(XWingFoilParts()...)
}

// XWingFoilAssembly groups the four independently drawable components of one
// deployed S-foil. Keeping the names with the geometry avoids fragile
// index-based grouping in catalog composition.
type XWingFoilAssembly struct {
	Name          string
	Roll          float64
	Wing          Model
	RearEngine    Model
	ForwardEngine Model
	Cannon        Model
	Muzzle        math3d.Vec3
}

// XWingGeometry contains the immutable geometry variants shared by catalog
// objects, debris generation, and showcase rendering.
type XWingGeometry struct {
	Fuselage  Model
	Canopy    Model
	Foils     []XWingFoilAssembly
	Window    Model
	Fragments [3]Model
}

// XWingGeometryData builds the X-Wing geometry family once for callers that
// need both drawable parts and destruction fragments.
func XWingGeometryData() XWingGeometry {
	fuselage := xWingFuselage()
	canopy := xWingCanopy()
	foils := XWingFoilAssemblies()
	window := Transform(canopy, math3d.Translation(0, 0, 0.015))
	full := mergeXWingParts(fuselage, canopy, foils)
	return XWingGeometry{
		Fuselage:  fuselage,
		Canopy:    canopy,
		Foils:     foils,
		Window:    window,
		Fragments: splitXWingFragments(full),
	}
}

func mergeXWingParts(fuselage, canopy Model, foils []XWingFoilAssembly) Model {
	parts := []Model{fuselage, canopy}
	for _, foil := range foils {
		parts = append(parts, foil.Wing, foil.RearEngine, foil.ForwardEngine, foil.Cannon)
	}
	return Merge(parts...)
}

// XWingFoilAssemblies returns the four named, transformed S-foil assemblies.
func XWingFoilAssemblies() []XWingFoilAssembly {
	wing := wingSlab()
	engineParts := xWingEngineParts()
	cannon := Transform(xWingCannon(), math3d.Translation(xWingWingTipX, .70, xWingCannonMountZ))
	placements := []struct {
		name   string
		roll   float64
		mountY float64
	}{
		{name: "upper-right S-foil", roll: math.Pi * 8 / 180, mountY: .30},
		{name: "upper-left S-foil", roll: math.Pi * 148 / 180, mountY: .15},
		{name: "lower-left S-foil", roll: math.Pi * 188 / 180, mountY: .30},
		{name: "lower-right S-foil", roll: math.Pi * 328 / 180, mountY: .15},
	}
	assemblies := make([]XWingFoilAssembly, 0, len(placements))
	for _, placement := range placements {
		rotation := math3d.RotationZ(placement.roll)
		assembly := XWingFoilAssembly{
			Name: placement.name,
			Roll: placement.roll,
			Wing: Transform(wing, rotation),
		}
		for index, engine := range engineParts {
			mounted := Transform(engine, math3d.Translation(xWingEngineMountX, placement.mountY, xWingEngineMountZ))
			mounted = Transform(mounted, rotation)
			if index == 0 {
				assembly.RearEngine = mounted
			} else {
				assembly.ForwardEngine = mounted
			}
		}
		mountedCannon := Transform(cannon, rotation)
		muzzle := rotation.TransformPoint(math3d.Vec3{X: xWingWingTipX, Y: .70, Z: xWingCannonMuzzleZ})
		if math.Sin(placement.roll) > 0 {
			mountedCannon = Transform(mountedCannon, math3d.Translation(0, .10, 0))
			muzzle.Y += .10
		} else {
			mountedCannon = Transform(mountedCannon, math3d.Translation(0, -.10, 0))
			muzzle.Y -= .10
		}
		assembly.Cannon = mountedCannon
		assembly.Muzzle = muzzle
		assemblies = append(assemblies, assembly)
	}
	return assemblies
}

// XWingFoilModels returns the four independently occluding S-foil assemblies.
// Each assembly includes its engine and wingtip cannon.
func XWingFoilModels() []Model {
	assemblies := XWingFoilAssemblies()
	models := make([]Model, 0, len(assemblies))
	for _, assembly := range assemblies {
		models = append(models, Merge(assembly.Wing, assembly.RearEngine, assembly.ForwardEngine, assembly.Cannon))
	}
	return models
}

// XWingFoilParts returns independently occluding wing slabs, engine nacelles
// and cannons for all four S-foil assemblies. This is the scene-composition
// form used when physical subcomponents need separate depth ownership.
func XWingFoilParts() []Model {
	assemblies := XWingFoilAssemblies()
	parts := make([]Model, 0, len(assemblies)*4)
	for _, assembly := range assemblies {
		parts = append(parts, assembly.Wing, assembly.RearEngine, assembly.ForwardEngine, assembly.Cannon)
	}
	return parts
}

// XWingWindow is a separate raised canopy/window layer for contrasting color.
func XWingWindow() Model {
	return Transform(xWingCanopy(), math3d.Translation(0, 0, 0.015))
}

func xWingFuselage() Model {
	mesh := Model{}
	// Use several short taper sections at the nose rather than collapsing
	// directly to a point. The small final ring and cap produce the blunt,
	// rounded nose-cone silhouette seen from above.
	sections := []struct{ z, rx, ry float64 }{
		{-1.45, 0.62, 0.28}, {-1.10, 0.78, 0.35}, {-0.35, 0.80, 0.36},
		{0.35, 0.58, 0.27}, {1.25, 0.34, 0.18}, {1.65, 0.28, 0.14},
		{2.05, 0.19, 0.10}, {2.35, 0.13, 0.075}, {2.5, 0.10, 0.06},
	}
	const segments = 6
	for _, section := range sections {
		for i := 0; i < segments; i++ {
			a := 2 * math.Pi * float64(i) / segments
			s, c := math.Sincos(a)
			mesh.Verts = append(mesh.Verts, math3d.Vec3{X: section.rx * c, Y: section.ry * s, Z: section.z})
		}
	}
	for ring := 0; ring < len(sections); ring++ {
		base := ring * segments
		for i := 0; i < segments; i++ {
			n := (i + 1) % segments
			mesh.Edges = append(mesh.Edges, Edge{A: base + i, B: base + n})
			if ring+1 < len(sections) {
				mesh.Edges = append(mesh.Edges, Edge{A: base + i, B: base + segments + i})
				mesh.Faces = append(mesh.Faces, Face{Vertices: []int{base + i, base + n, base + segments + n, base + segments + i}})
			}
		}
	}
	for _, ring := range []int{0, len(sections) - 1} {
		center := len(mesh.Verts)
		mesh.Verts = append(mesh.Verts, math3d.Vec3{Z: sections[ring].z})
		base := ring * segments
		for i := 0; i < segments; i++ {
			next := (i + 1) % segments
			mesh.Edges = append(mesh.Edges, Edge{A: center, B: base + i})
			vertices := []int{center, base + i, base + next}
			if ring == len(sections)-1 {
				vertices = []int{center, base + next, base + i}
			}
			mesh.Faces = append(mesh.Faces, Face{Vertices: vertices})
		}
	}
	return OrientOutward(mesh)
}

func xWingCanopy() Model {
	// Faceted canopy: shallow at the forward (+Z) edge and raised toward the
	// aft (-Z) edge, integrated into the upper fuselage rather than standing
	// vertically beside it.
	mesh := Model{Verts: []math3d.Vec3{
		// Keep the cockpit as a shallow upper-hull feature. Its lower edge
		// stays above the fuselage centreline so the amber window cannot show
		// through the underside of the hull in front views.
		{X: -.24, Y: .02, Z: 1.08}, {X: .24, Y: .02, Z: 1.08},
		{X: .24, Y: .24, Z: 1.08}, {X: -.24, Y: .24, Z: 1.08},
		{X: -.30, Y: .02, Z: .25}, {X: .30, Y: .02, Z: .25},
		{X: .27, Y: .38, Z: .25}, {X: -.27, Y: .38, Z: .25},
	}}
	for i := 0; i < 4; i++ {
		next := (i + 1) % 4
		mesh.Edges = append(mesh.Edges, Edge{A: i, B: next}, Edge{A: 4 + i, B: 4 + next}, Edge{A: i, B: 4 + i})
		mesh.Faces = append(mesh.Faces, Face{Vertices: []int{i, next, 4 + next, 4 + i}})
	}
	mesh.Faces = append(mesh.Faces, Face{Vertices: []int{0, 1, 2, 3}}, Face{Vertices: []int{7, 6, 5, 4}})
	return OrientOutward(mesh)
}

// wingSlab builds one canonical upper-right S-foil. Its broad Z chord runs
// from behind the cockpit toward the compact rear body; rotation around Z
// supplies the other three wings while preserving a thin physical thickness.
func wingSlab() Model {
	// The outline is defined in the X-Z top view. Both root and tip edges are
	// parallel to the fuselage axis (+/-Z), while the trailing edge sweeps aft
	// toward the fuselage. Y provides the shallow S-foil dihedral; a thin Y
	// extrusion gives the panel a real solid thickness without distorting its
	// top-view silhouette.
	profile := []math3d.Vec3{
		{X: xWingWingRootX, Y: .12, Z: .05},
		{X: xWingWingTipX, Y: .70, Z: -.10},
		{X: xWingWingTipX, Y: .70, Z: -.50},
		{X: xWingWingRootX, Y: .12, Z: -1.10},
	}
	const thickness = .06
	mesh := Model{}
	for _, point := range profile {
		mesh.Verts = append(mesh.Verts, point.Add(math3d.Vec3{Y: thickness / 2}))
	}
	for _, point := range profile {
		mesh.Verts = append(mesh.Verts, point.Sub(math3d.Vec3{Y: thickness / 2}))
	}
	for i := range profile {
		next := (i + 1) % len(profile)
		mesh.Edges = append(mesh.Edges,
			Edge{A: i, B: next}, Edge{A: 4 + i, B: 4 + next}, Edge{A: i, B: 4 + i},
			// The diagonal is needed to describe the two planar triangles but is
			// construction topology, not a vector line to submit.
			Edge{A: i, B: 4 + next, Kind: EdgeInternal})
		mesh.Faces = append(mesh.Faces,
			Face{Vertices: []int{i, next, 4 + next}},
			Face{Vertices: []int{i, 4 + next, 4 + i}},
		)
	}
	mesh.Faces = append(mesh.Faces,
		Face{Vertices: []int{0, 1, 2}},
		Face{Vertices: []int{0, 2, 3}},
		Face{Vertices: []int{7, 6, 5}},
		Face{Vertices: []int{7, 5, 4}},
	)
	mesh.Edges = append(mesh.Edges,
		Edge{A: 0, B: 2, Kind: EdgeInternal},
		Edge{A: 7, B: 5, Kind: EdgeInternal},
	)
	return OrientOutward(mesh)
}

// xWingEngineParts returns the narrow rear and wider forward sections as
// separate solids. The assembled engine remains one model, while scene users
// can give each section an independent depth owner where self-occlusion matters.
func xWingEngineParts() []Model {
	const (
		radius      = 0.27
		rearLength  = 1.16
		frontLength = 0.62
		shoulderZ   = 0.16
		// Extend the forward cylinder toward the rear of the wing while
		// keeping its nose close to the wing leading edge.
		frontShiftBack = 0.12
	)
	rear := Transform(cylinder(radius*0.52, rearLength, 8), math3d.Translation(0, 0, -1+rearLength/2))
	front := Transform(cylinder(radius*1.12, frontLength, 8), math3d.Translation(0, 0, shoulderZ+frontLength/2-frontShiftBack))
	return []Model{rear, front}
}

func xWingCannon() Model {
	// Keep the muzzle at the forward end while making the rear housing span
	// the wingtip chord and letting the slim barrel project just past it.
	barrel := cylinder(0.045, xWingCannonBarrelLength, 6)
	mount := prism([]math3d.Vec3{
		{X: -.09, Y: -.09, Z: xWingCannonMountCenterZ},
		{X: .09, Y: -.09, Z: xWingCannonMountCenterZ},
		{X: .09, Y: .09, Z: xWingCannonMountCenterZ},
		{X: -.09, Y: .09, Z: xWingCannonMountCenterZ},
	}, xWingCannonMountDepth)
	return Merge(Transform(barrel, math3d.Translation(0, 0, xWingCannonBarrelCenterZ)), mount)
}

func cylinder(radius, length float64, segments int) Model {
	mesh := Model{}
	for _, z := range []float64{-length / 2, length / 2} {
		for i := 0; i < segments; i++ {
			a := 2 * math.Pi * float64(i) / float64(segments)
			s, c := math.Sincos(a)
			mesh.Verts = append(mesh.Verts, math3d.Vec3{X: radius * c, Y: radius * s, Z: z})
		}
	}
	for i := 0; i < segments; i++ {
		n := (i + 1) % segments
		mesh.Edges = append(mesh.Edges, Edge{A: i, B: n}, Edge{A: i, B: segments + i})
		mesh.Faces = append(mesh.Faces, Face{Vertices: []int{i, n, segments + n, segments + i}})
	}
	frontCenter := len(mesh.Verts)
	mesh.Verts = append(mesh.Verts, math3d.Vec3{Z: length / 2})
	rearCenter := len(mesh.Verts)
	mesh.Verts = append(mesh.Verts, math3d.Vec3{Z: -length / 2})
	for i := 0; i < segments; i++ {
		n := (i + 1) % segments
		mesh.Edges = append(mesh.Edges,
			Edge{A: frontCenter, B: segments + i},
			Edge{A: rearCenter, B: i},
		)
		mesh.Faces = append(mesh.Faces,
			Face{Vertices: []int{frontCenter, segments + i, segments + n}},
			Face{Vertices: []int{rearCenter, n, i}},
		)
	}
	return OrientOutward(mesh)
}

func prism(profile []math3d.Vec3, depth float64) Model {
	mesh := Model{}
	for _, point := range profile {
		mesh.Verts = append(mesh.Verts, point.Add(math3d.Vec3{Z: -depth / 2}))
	}
	for _, point := range profile {
		mesh.Verts = append(mesh.Verts, point.Add(math3d.Vec3{Z: depth / 2}))
	}
	n := len(profile)
	for i := 0; i < n; i++ {
		next := (i + 1) % n
		mesh.Edges = append(mesh.Edges, Edge{A: i, B: next}, Edge{A: n + i, B: n + next}, Edge{A: i, B: n + i})
		mesh.Faces = append(mesh.Faces, Face{Vertices: []int{i, next, n + next, n + i}})
	}
	front := make([]int, n)
	rear := make([]int, n)
	for i := 0; i < n; i++ {
		front[i] = i
		rear[n-1-i] = n + i
	}
	mesh.Faces = append(mesh.Faces, Face{Vertices: front}, Face{Vertices: rear})
	return OrientOutward(mesh)
}

// XWingFragments partitions the model into three spatial debris groups.
func XWingFragments() [3]Model {
	return splitXWingFragments(XWing())
}

func splitXWingFragments(hull Model) [3]Model {
	fragments := [3]Model{{Verts: hull.Verts}, {Verts: hull.Verts}, {Verts: hull.Verts}}
	for _, edge := range hull.Edges {
		mid := (hull.Verts[edge.A].X + hull.Verts[edge.B].X) / 2
		index := 1
		if mid < -1 {
			index = 0
		} else if mid > 1 {
			index = 2
		}
		fragments[index].Edges = append(fragments[index].Edges, edge)
	}
	for _, face := range hull.Faces {
		centroid := 0.0
		for _, vertex := range face.Vertices {
			centroid += hull.Verts[vertex].X
		}
		index := 1
		if centroid/float64(len(face.Vertices)) < -1 {
			index = 0
		} else if centroid/float64(len(face.Vertices)) > 1 {
			index = 2
		}
		fragments[index].Faces = append(fragments[index].Faces, face)
	}
	addFractureCaps(hull, &fragments, []float64{-1, 1})
	for index := range fragments {
		fragments[index] = Prepare(fragments[index])
	}
	return fragments
}
