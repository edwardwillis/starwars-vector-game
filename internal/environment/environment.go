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
	Features   []Feature
	Planes     []collision.FinitePlane
	Boxes      []collision.OrientedBox
	Bounds     RenderBounds
}

// Feature describes an addressable installation generated with a tile. The
// local model and collider share one pose so a later authoritative object can
// be spawned without reconstructing placement from renderer data.
type Feature struct {
	ID   string
	Kind string
	Pose kinematics.Pose
	// Scale turns shared immutable prototype meshes into differently sized
	// feature instances without baking a fresh mesh for every streamed tile.
	// The zero value means unit scale for compatibility with existing features.
	Scale      math3d.Vec3
	Parts      []scene.Part
	Boxes      []collision.OrientedBox
	Bounds     RenderBounds
	Detail     scene.DetailTier
	Targetable bool
	Hittable   bool
}

// Matrix returns the feature-local transform used by every visual pass.
func (feature Feature) Matrix() math3d.Mat4 {
	scale := feature.Scale
	if scale == (math3d.Vec3{}) {
		scale = math3d.Vec3{X: 1, Y: 1, Z: 1}
	}
	return feature.Pose.Matrix().Mul(math3d.Scaling(scale.X, scale.Y, scale.Z))
}

type Definition struct {
	Name             string
	Frame            scene.FrameID
	HostDefinition   string
	LocalPose        kinematics.Pose
	Bounds           Volume
	ExitVolume       Volume
	TileSize         float64
	TileRadius       int
	DetailThresholds scene.DetailThresholds
	Transitions      []Transition
	Tile             func(TileCoordinate) Tile
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
	for featureIndex := range tile.Features {
		feature := &tile.Features[featureIndex]
		var featureBounds RenderBounds
		for partIndex := range feature.Parts {
			feature.Parts[partIndex].Mesh = model.Prepare(feature.Parts[partIndex].Mesh)
			featureBounds = mergeRenderBounds(featureBounds, modelRenderBounds(feature.Parts[partIndex].Mesh, math3d.Identity()))
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
// is the ordered planar aperture used by later portal clipping; merely
// registering a portal does not make the containing room's background opaque
// or visible outside that aperture.
type Portal struct {
	Name        string
	Destination scene.FrameID
	Boundary    []math3d.Vec3
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
		if err := part.Mesh.Validate(); err != nil {
			return fmt.Errorf("room %q part %d: %w", room.Name, index, err)
		}
		if part.LineWidth <= 0 {
			return fmt.Errorf("room %q part %d requires positive line width", room.Name, index)
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
	if def.DetailThresholds.MediumPixels < 0 || def.DetailThresholds.NearPixels < 0 ||
		(def.DetailThresholds.NearPixels > 0 && def.DetailThresholds.NearPixels < def.DetailThresholds.MediumPixels) {
		return fmt.Errorf("environment %q has invalid detail thresholds", def.Name)
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
