// Package environment defines flyable local spaces attached to large objects.
package environment

import (
	"fmt"
	"math"
	"sort"
	"strconv"

	"github.com/edwardwillis/starwars-vector-game/internal/collision"
	"github.com/edwardwillis/starwars-vector-game/internal/kinematics"
	"github.com/edwardwillis/starwars-vector-game/internal/math3d"
	"github.com/edwardwillis/starwars-vector-game/internal/model"
	"github.com/edwardwillis/starwars-vector-game/internal/scene"
	"github.com/edwardwillis/starwars-vector-game/internal/view"
)

type Volume struct{ Center, HalfExtents math3d.Vec3 }

func (volume Volume) Contains(point math3d.Vec3) bool {
	d := point.Sub(volume.Center)
	return abs(d.X) <= volume.HalfExtents.X && abs(d.Y) <= volume.HalfExtents.Y && abs(d.Z) <= volume.HalfExtents.Z
}

func (volume Volume) Validate() error {
	if volume.HalfExtents.X <= 0 || volume.HalfExtents.Y <= 0 || volume.HalfExtents.Z <= 0 {
		return fmt.Errorf("transition volume half-extents must be positive")
	}
	return nil
}

type Transition struct {
	Name                string
	Source, Destination scene.FrameID
	Trigger             Volume
	Duration            float64
	EntryPose, ExitPose kinematics.Pose
	// Local doorway transfers may keep the craft's heading and lateral
	// position. EntryOffset nudges it clear of the opposite trigger.
	PreservePose bool
	EntryOffset  math3d.Vec3
	// ApproachDirection is expressed in Source coordinates; zero accepts
	// either direction, while a nonzero vector gates a one-way opening.
	ApproachDirection math3d.Vec3
}

type TileCoordinate struct{ X, Z int }

// RenderBounds is a conservative sphere in the containing geometry's local
// coordinates. It is presentation metadata only: collision and targeting keep
// using their authoritative planes and boxes even when visuals are culled.
type RenderBounds struct {
	Center math3d.Vec3
	Radius float64
}

func (bounds RenderBounds) Valid() bool { return bounds.Radius > 0 }

type Tile struct {
	Coordinate TileCoordinate
	Parts      []scene.Part
	// PortalParts optionally excludes geometry duplicated by a neighboring
	// room shell when this tile is viewed through that room's open portal.
	// Nil means all ordinary Parts are eligible.
	PortalParts []scene.Part
	Features    []Feature
	Planes      []collision.FinitePlane
	Boxes       []collision.OrientedBox
	Bounds      RenderBounds
}

// Feature describes an addressable installation generated with a tile. The
// local model and collider share one pose so a later authoritative object can
// be spawned without reconstructing placement from renderer data.
type Feature struct {
	ID   string
	Kind string
	Team scene.TeamID
	Pose kinematics.Pose
	// Scale turns shared immutable prototype meshes into differently sized
	// feature instances without baking a fresh mesh for every streamed tile.
	// The zero value means unit scale for compatibility with existing features.
	Scale math3d.Vec3
	Parts []scene.Part
	// Muzzles are immutable feature-local emitter anchors transformed with
	// the same pose/scale as the visible installation geometry.
	Muzzles []math3d.Vec3
	// TurretPart is the index of an articulated part, or zero for a fixed
	// installation. TurretPivot is in feature-local coordinates. The simulation
	// supplies its yaw/pitch; neither target acquisition nor articulation is
	// driven by rendering or tile visibility.
	TurretPart  int
	TurretPivot math3d.Vec3
	BarrelPart  int
	BarrelPivot math3d.Vec3
	// WreckParts replace Parts after destruction. A visual-only remnant may
	// persist without retaining the intact installation's collision boxes.
	WreckParts []scene.Part
	Boxes      []collision.OrientedBox
	Bounds     RenderBounds
	Detail     scene.DetailTier
	Targetable bool
	Hittable   bool
	HitPoints  int
	// DisableAfter is the number of hits that stops an active emplacement
	// before final destruction. Zero means it remains active until destroyed.
	DisableAfter int
}

// Matrix returns the feature-local transform used by every visual pass.
func (feature Feature) Matrix() math3d.Mat4 {
	scale := feature.Scale
	if scale == (math3d.Vec3{}) {
		scale = math3d.Vec3{X: 1, Y: 1, Z: 1}
	}
	return feature.Pose.Matrix().Mul(math3d.Scaling(scale.X, scale.Y, scale.Z))
}

// TurretMatrix articulates a feature-local part about its mounting pivot.
// The same transform is used for the visible turret and its muzzle anchors.
func (feature Feature) TurretMatrix(yaw, pitch float64) math3d.Mat4 {
	pivot := feature.TurretPivot
	yawMatrix := math3d.Translation(pivot.X, pivot.Y, pivot.Z).
		Mul(math3d.QuaternionFromAxisAngle(math3d.Vec3{Y: 1}, yaw).Matrix()).
		Mul(math3d.Translation(-pivot.X, -pivot.Y, -pivot.Z))
	if pitch == 0 {
		return yawMatrix
	}
	barrel := feature.BarrelPivot
	return yawMatrix.Mul(math3d.Translation(barrel.X, barrel.Y, barrel.Z)).
		Mul(math3d.QuaternionFromAxisAngle(math3d.Vec3{X: 1}, pitch).Matrix()).
		Mul(math3d.Translation(-barrel.X, -barrel.Y, -barrel.Z))
}

type Definition struct {
	Name           string
	Frame          scene.FrameID
	HostDefinition string
	// LinkedFrames are other host-bound local environments reachable from this
	// definition. Unlisted frame IDs remain absolute (e.g. shared/static rooms).
	LinkedFrames []scene.FrameID
	LocalPose    kinematics.Pose
	Bounds       Volume
	ExitVolume   Volume
	TileSize     float64
	TileRadius   int
	// HorizonTileRadius optionally extends presentation-only terrain beyond the
	// physical tile stream. HorizonTile must return geometry without collision
	// or addressable features; this keeps a distant surface cheap while nearby
	// Tile geometry remains authoritative.
	HorizonTileRadius int
	HorizonTile       func(TileCoordinate) Tile
	// DepthProxy optionally supplies a small set of solid, local-space meshes
	// for the CPU hidden-line prepass covering the currently active tiles.
	// It is deliberately independent from visual tile detail: the ordinary tile
	// Parts still supply vector lines, fills, collision and feature placement.
	DepthProxy       func([]TileCoordinate) []model.Model
	DetailThresholds scene.DetailThresholds
	// LevelUp is the local horizon reference for optional manual-flight
	// assistance. Zero deliberately disables leveling for interiors and other
	// environments without a meaningful open-surface horizon.
	LevelUp     math3d.Vec3
	Transitions []Transition
	Tile        func(TileCoordinate) Tile
}

// PrepareTile compiles immutable model topology and aggregate render bounds
// once when a tile enters the stream. Candidate preparation can then reject a
// tile or feature without walking and transforming each contained mesh.
func PrepareTile(source Tile) Tile {
	tile := source
	var tileBounds RenderBounds
	for index := range tile.Parts {
		tile.Parts[index].Mesh = model.Prepare(tile.Parts[index].Mesh)
		tileBounds = mergeRenderBounds(tileBounds, modelRenderBounds(tile.Parts[index].Mesh, math3d.Identity()))
	}
	for index := range tile.PortalParts {
		tile.PortalParts[index].Mesh = model.Prepare(tile.PortalParts[index].Mesh)
	}
	for featureIndex := range tile.Features {
		feature := &tile.Features[featureIndex]
		var featureBounds RenderBounds
		for partIndex := range feature.Parts {
			feature.Parts[partIndex].Mesh = model.Prepare(feature.Parts[partIndex].Mesh)
			featureBounds = mergeRenderBounds(featureBounds, modelRenderBounds(feature.Parts[partIndex].Mesh, math3d.Identity()))
		}
		for partIndex := range feature.WreckParts {
			feature.WreckParts[partIndex].Mesh = model.Prepare(feature.WreckParts[partIndex].Mesh)
			featureBounds = mergeRenderBounds(featureBounds, modelRenderBounds(feature.WreckParts[partIndex].Mesh, math3d.Identity()))
		}
		if feature.TurretPart > 0 && feature.TurretPart < len(feature.Parts) {
			// The turret can point anywhere in its allowed arc. A pivot-centred
			// sphere conservatively bounds every orientation without per-frame
			// tile-bound recomputation.
			part := feature.Parts[feature.TurretPart].Mesh
			pivotRadius := 0.0
			for _, vertex := range part.Verts {
				pivotRadius = max(pivotRadius, vertex.Sub(feature.TurretPivot).Length())
			}
			featureBounds = mergeRenderBounds(featureBounds, RenderBounds{Center: feature.TurretPivot, Radius: pivotRadius})
		}
		if feature.BarrelPart > 0 && feature.BarrelPart < len(feature.Parts) {
			part := feature.Parts[feature.BarrelPart].Mesh
			pivotRadius := 0.0
			for _, vertex := range part.Verts {
				pivotRadius = max(pivotRadius, vertex.Sub(feature.TurretPivot).Length())
			}
			featureBounds = mergeRenderBounds(featureBounds, RenderBounds{Center: feature.TurretPivot, Radius: pivotRadius})
		}
		feature.Bounds = featureBounds
		tileBounds = mergeRenderBounds(tileBounds, transformRenderBounds(featureBounds, feature.Matrix()))
	}
	tile.Bounds = tileBounds
	return tile
}

func modelRenderBounds(mesh model.Model, transform math3d.Mat4) RenderBounds {
	prepared := model.Prepare(mesh)
	if prepared.Topology == nil || prepared.Topology.BoundsRadius <= 0 {
		return RenderBounds{}
	}
	return transformRenderBounds(RenderBounds{Center: prepared.Topology.BoundsCenter, Radius: prepared.Topology.BoundsRadius}, transform)
}

func transformRenderBounds(bounds RenderBounds, transform math3d.Mat4) RenderBounds {
	if !bounds.Valid() {
		return RenderBounds{}
	}
	scale := max(
		transform.TransformDirection(math3d.Vec3{X: 1}).Length(),
		transform.TransformDirection(math3d.Vec3{Y: 1}).Length(),
		transform.TransformDirection(math3d.Vec3{Z: 1}).Length(),
	)
	return RenderBounds{Center: transform.TransformPoint(bounds.Center), Radius: bounds.Radius * scale}
}

func mergeRenderBounds(first, second RenderBounds) RenderBounds {
	if !first.Valid() {
		return second
	}
	if !second.Valid() {
		return first
	}
	delta := second.Center.Sub(first.Center)
	distance := delta.Length()
	if first.Radius >= distance+second.Radius {
		return first
	}
	if second.Radius >= distance+first.Radius {
		return second
	}
	radius := (distance + first.Radius + second.Radius) * 0.5
	center := first.Center
	if distance > 1e-12 {
		center = center.Add(delta.Scale((radius - first.Radius) / distance))
	}
	return RenderBounds{Center: center, Radius: radius}
}

// Bound is one environment definition attached to one concrete host object.
// The instance frame includes the host ID so multiple Death Stars, capital
// ships, or stations never share a coordinate space accidentally.
type Bound struct {
	Definition Definition
	HostID     scene.ObjectID
	FrameID    scene.FrameID
}

// Portal is a local-space opening from one room frame into another. Boundary
// is the ordered planar aperture used for destination-view clipping; merely
// registering a portal does not make the containing room's background opaque
// or visible outside that aperture.
type Portal struct {
	Name        string
	Destination scene.FrameID
	Boundary    []math3d.Vec3
}

// CrossingPoint reports where a segment actually passes through a convex
// portal. It is direction-independent, so the same opening can transfer
// projectiles between either pair of linked flight environments.
func CrossingPoint(boundary []math3d.Vec3, from, to math3d.Vec3) (math3d.Vec3, bool) {
	if len(boundary) < 3 {
		return math3d.Vec3{}, false
	}
	origin := boundary[0]
	var normal math3d.Vec3
	for index := 2; index < len(boundary); index++ {
		normal = boundary[index-1].Sub(origin).Cross(boundary[index].Sub(origin))
		if normal.Length() > 1e-9 {
			break
		}
	}
	if normal.Length() <= 1e-9 {
		return math3d.Vec3{}, false
	}
	normal = normal.Normalize()
	first, last := normal.Dot(from.Sub(origin)), normal.Dot(to.Sub(origin))
	if math.Abs(last) < 1e-8 || first*last > 0 || (math.Abs(first) < 1e-8 && math.Abs(last) < 1e-8) {
		return math3d.Vec3{}, false
	}
	fraction := first / (first - last)
	if fraction < 0 || fraction >= 1 {
		return math3d.Vec3{}, false
	}
	hit := from.Add(to.Sub(from).Scale(fraction))
	for index, a := range boundary {
		b := boundary[(index+1)%len(boundary)]
		if b.Sub(a).Cross(hit.Sub(a)).Dot(normal) < -1e-7 {
			return math3d.Vec3{}, false
		}
	}
	return hit, true
}

func (portal Portal) Validate() error {
	if portal.Name == "" || portal.Destination == "" {
		return fmt.Errorf("portal requires name and destination")
	}
	if len(portal.Boundary) < 3 {
		return fmt.Errorf("portal %q requires at least three boundary vertices", portal.Name)
	}
	for _, point := range portal.Boundary {
		if math.IsNaN(point.X) || math.IsInf(point.X, 0) || math.IsNaN(point.Y) || math.IsInf(point.Y, 0) || math.IsNaN(point.Z) || math.IsInf(point.Z, 0) {
			return fmt.Errorf("portal %q has a non-finite boundary vertex", portal.Name)
		}
	}
	origin := portal.Boundary[0]
	var normal math3d.Vec3
	for index := 2; index < len(portal.Boundary); index++ {
		normal = portal.Boundary[index-1].Sub(origin).Cross(portal.Boundary[index].Sub(origin))
		if normal.Length() > 1e-9 {
			break
		}
	}
	if normal.Length() <= 1e-9 {
		return fmt.Errorf("portal %q has a degenerate boundary", portal.Name)
	}
	normal = normal.Normalize()
	for _, point := range portal.Boundary[1:] {
		if math.Abs(normal.Dot(point.Sub(origin))) > 1e-7 {
			return fmt.Errorf("portal %q boundary is not planar", portal.Name)
		}
	}
	for index, current := range portal.Boundary {
		previous := portal.Boundary[(index+len(portal.Boundary)-1)%len(portal.Boundary)]
		next := portal.Boundary[(index+1)%len(portal.Boundary)]
		if previous.Sub(current).Cross(next.Sub(current)).Dot(normal) > 1e-7 {
			return fmt.Errorf("portal %q boundary must be convex", portal.Name)
		}
	}
	return nil
}

// Room supplies view-level policy for an enclosed environment frame. Room
// geometry remains ordinary prepared scene geometry; this metadata only
// selects its background and declares future clipping apertures.
type Room struct {
	Name       string
	Frame      scene.FrameID
	Background view.Background
	// Parts are immutable room-local geometry. They enter the same prepared
	// candidate pipeline as objects and streamed surface tiles.
	Parts   []scene.Part
	Bounds  RenderBounds
	Portals []Portal
}

func (room Room) Validate() error {
	if room.Name == "" || room.Frame == "" {
		return fmt.Errorf("room requires name and frame")
	}
	if err := room.Background.Validate(); err != nil {
		return fmt.Errorf("room %q: %w", room.Name, err)
	}
	for index, part := range room.Parts {
		if err := part.Validate(); err != nil {
			return fmt.Errorf("room %q part %d: %w", room.Name, index, err)
		}
	}
	seen := make(map[string]bool, len(room.Portals))
	for _, portal := range room.Portals {
		if err := portal.Validate(); err != nil {
			return fmt.Errorf("room %q: %w", room.Name, err)
		}
		if seen[portal.Name] {
			return fmt.Errorf("room %q has duplicate portal %q", room.Name, portal.Name)
		}
		seen[portal.Name] = true
	}
	return nil
}

func Bind(registry *Registry, objects []scene.Object) []Bound {
	if registry == nil {
		return nil
	}
	var bound []Bound
	for _, definition := range registry.Definitions() {
		for _, object := range objects {
			if object.Definition != definition.HostDefinition {
				continue
			}
			bound = append(bound, Bound{
				Definition: definition,
				HostID:     object.ID,
				FrameID:    scene.FrameID(string(definition.Frame) + "/" + strconv.FormatUint(uint64(object.ID), 10)),
			})
		}
	}
	return bound
}

func (bound Bound) ResolveFrame(frame scene.FrameID) scene.FrameID {
	if frame == bound.Definition.Frame {
		return bound.FrameID
	}
	for _, linked := range bound.Definition.LinkedFrames {
		if frame == linked {
			return scene.FrameID(string(frame) + "/" + strconv.FormatUint(uint64(bound.HostID), 10))
		}
	}
	return frame
}

type Registry struct {
	definitions map[string]Definition
	rooms       map[scene.FrameID]Room
}

func NewRegistry() *Registry {
	return &Registry{
		definitions: make(map[string]Definition),
		rooms:       make(map[scene.FrameID]Room),
	}
}
func (registry *Registry) Register(def Definition) error {
	if registry == nil {
		return fmt.Errorf("environment registry is nil")
	}
	if def.Name == "" || def.Frame == "" || def.HostDefinition == "" || def.Tile == nil {
		return fmt.Errorf("environment definition requires name, frame, host, and tile factory")
	}
	if def.TileSize <= 0 || def.TileRadius < 0 {
		return fmt.Errorf("environment %q requires a positive tile size and non-negative tile radius", def.Name)
	}
	if def.HorizonTileRadius < 0 || (def.HorizonTileRadius > 0 && def.HorizonTile == nil) {
		return fmt.Errorf("environment %q has an invalid horizon tile configuration", def.Name)
	}
	if def.HorizonTileRadius > 0 && def.HorizonTileRadius <= def.TileRadius {
		return fmt.Errorf("environment %q horizon tile radius must exceed its physical tile radius", def.Name)
	}
	if def.DetailThresholds.MediumPixels < 0 || def.DetailThresholds.NearPixels < 0 ||
		(def.DetailThresholds.NearPixels > 0 && def.DetailThresholds.NearPixels < def.DetailThresholds.MediumPixels) {
		return fmt.Errorf("environment %q has invalid detail thresholds", def.Name)
	}
	if def.LevelUp != (math3d.Vec3{}) {
		if math.IsNaN(def.LevelUp.X) || math.IsInf(def.LevelUp.X, 0) || math.IsNaN(def.LevelUp.Y) || math.IsInf(def.LevelUp.Y, 0) || math.IsNaN(def.LevelUp.Z) || math.IsInf(def.LevelUp.Z, 0) {
			return fmt.Errorf("environment %q has a non-finite level-up vector", def.Name)
		}
		def.LevelUp = def.LevelUp.Normalize()
	}
	if err := def.Bounds.Validate(); err != nil {
		return fmt.Errorf("environment %q bounds: %w", def.Name, err)
	}
	if def.ExitVolume.HalfExtents == (math3d.Vec3{}) {
		def.ExitVolume = def.Bounds
	}
	if err := def.ExitVolume.Validate(); err != nil {
		return fmt.Errorf("environment %q exit volume: %w", def.Name, err)
	}
	for index, transition := range def.Transitions {
		if transition.Name == "" || transition.Source == "" || transition.Destination == "" {
			return fmt.Errorf("environment %q transition %d requires name and frames", def.Name, index)
		}
		if transition.Duration < 0 {
			return fmt.Errorf("environment %q transition %q has negative duration", def.Name, transition.Name)
		}
		if transition.PreservePose && transition.Duration > 0 {
			return fmt.Errorf("environment %q transition %q cannot combine pose-preserving portal transfer with a timed cinematic", def.Name, transition.Name)
		}
		for _, component := range []float64{transition.ApproachDirection.X, transition.ApproachDirection.Y, transition.ApproachDirection.Z,
			transition.EntryOffset.X, transition.EntryOffset.Y, transition.EntryOffset.Z} {
			if math.IsNaN(component) || math.IsInf(component, 0) {
				return fmt.Errorf("environment %q transition %q has a non-finite portal direction or offset", def.Name, transition.Name)
			}
		}
		if err := transition.Trigger.Validate(); err != nil {
			return fmt.Errorf("environment %q transition %q: %w", def.Name, transition.Name, err)
		}
	}
	if _, exists := registry.definitions[def.Name]; exists {
		return fmt.Errorf("environment %q already registered", def.Name)
	}
	registry.definitions[def.Name] = def
	return nil
}

// Definitions returns a stable copy suitable for session initialization.
func (registry *Registry) Definitions() []Definition {
	if registry == nil {
		return nil
	}
	definitions := make([]Definition, 0, len(registry.definitions))
	for _, definition := range registry.definitions {
		definitions = append(definitions, definition)
	}
	sort.Slice(definitions, func(i, j int) bool { return definitions[i].Name < definitions[j].Name })
	return definitions
}
func (registry *Registry) Lookup(name string) (Definition, error) {
	if registry == nil {
		return Definition{}, fmt.Errorf("environment registry is nil")
	}
	def, ok := registry.definitions[name]
	if !ok {
		return Definition{}, fmt.Errorf("environment %q is not registered", name)
	}
	return def, nil
}

// RegisterRoom associates enclosed-view policy with a simulation frame.
func (registry *Registry) RegisterRoom(room Room) error {
	if registry == nil {
		return fmt.Errorf("environment registry is nil")
	}
	if room.Background.Kind == "" {
		room.Background.Kind = view.BackgroundNone
	}
	if err := room.Validate(); err != nil {
		return err
	}
	if registry.rooms == nil {
		registry.rooms = make(map[scene.FrameID]Room)
	}
	if _, exists := registry.rooms[room.Frame]; exists {
		return fmt.Errorf("room frame %q already registered", room.Frame)
	}
	room.Parts = append([]scene.Part(nil), room.Parts...)
	room.Bounds = RenderBounds{}
	for index := range room.Parts {
		room.Parts[index].Mesh = model.Prepare(room.Parts[index].Mesh)
		room.Bounds = mergeRenderBounds(room.Bounds, modelRenderBounds(room.Parts[index].Mesh, math3d.Identity()))
	}
	room.Portals = append([]Portal(nil), room.Portals...)
	for index := range room.Portals {
		room.Portals[index].Boundary = append([]math3d.Vec3(nil), room.Portals[index].Boundary...)
	}
	registry.rooms[room.Frame] = room
	return nil
}

// ViewContext resolves a frame through the room provider. The bool is false
// for ordinary exterior/local frames, allowing the caller's default policy.
func (registry *Registry) ViewContext(frame scene.FrameID, matrix math3d.Mat4) (view.Context, bool) {
	if registry == nil {
		return view.Context{}, false
	}
	room, ok := registry.rooms[frame]
	if !ok {
		return view.Context{}, false
	}
	return view.Context{FrameID: frame, ViewMatrix: matrix, Background: room.Background}, true
}

// Room returns a defensive copy of room and portal metadata for the frame.
func (registry *Registry) Room(frame scene.FrameID) (Room, bool) {
	if registry == nil {
		return Room{}, false
	}
	room, ok := registry.rooms[frame]
	if !ok {
		return Room{}, false
	}
	room.Portals = append([]Portal(nil), room.Portals...)
	for index := range room.Portals {
		room.Portals[index].Boundary = append([]math3d.Vec3(nil), room.Portals[index].Boundary...)
	}
	return room, true
}
func abs(value float64) float64 {
	if value < 0 {
		return -value
	}
	return value
}
