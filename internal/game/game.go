package game

import (
	"fmt"
	"image/color"
	"math"
	"runtime"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"github.com/edwardwillis/starwars-vector-game/internal/appearance"
	"github.com/edwardwillis/starwars-vector-game/internal/camera"
	"github.com/edwardwillis/starwars-vector-game/internal/catalog"
	"github.com/edwardwillis/starwars-vector-game/internal/cockpit"
	"github.com/edwardwillis/starwars-vector-game/internal/collision"
	"github.com/edwardwillis/starwars-vector-game/internal/combat"
	"github.com/edwardwillis/starwars-vector-game/internal/control"
	"github.com/edwardwillis/starwars-vector-game/internal/environment"
	"github.com/edwardwillis/starwars-vector-game/internal/kinematics"
	"github.com/edwardwillis/starwars-vector-game/internal/math3d"
	modelpkg "github.com/edwardwillis/starwars-vector-game/internal/model"
	"github.com/edwardwillis/starwars-vector-game/internal/profile"
	"github.com/edwardwillis/starwars-vector-game/internal/render"
	"github.com/edwardwillis/starwars-vector-game/internal/scene"
	"github.com/edwardwillis/starwars-vector-game/internal/sim"
	"github.com/edwardwillis/starwars-vector-game/internal/starfield"
	"github.com/edwardwillis/starwars-vector-game/internal/view"
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/hajimehoshi/ebiten/v2/vector"
)

const (
	ScreenWidth  = 960
	ScreenHeight = 540
	fighterID    = scene.ObjectID(1)
)

var background = color.RGBA{R: 2, G: 4, B: 8, A: 255}

type flightMode int

const swarmInterceptorSlots = 2

const transitionEnvironmentTileRadius = 2

type autonomousRespawn struct {
	readyAt    float64
	definition string
}

type destructionTransient struct {
	remaining      float64
	rootObjectID   scene.ObjectID
	componentIndex int
	stage          scene.DestructionStage
	// sourceOrigin is the component's centre in the catalog model's original
	// local frame. Polygon factories use that same frame, so retaining the
	// origin lets second-stage shards be rebased without a visible jump.
	sourceOrigin math3d.Vec3
}

type surfaceEffect struct {
	remaining float64
}

// Feature damage is authoritative to the bound environment, not the streamed
// tile or the current camera. It survives tile eviction and regeneration.
type featureDamageState struct {
	Hits      int
	Disabled  bool
	Destroyed bool
}

var surfaceImpactMesh = modelpkg.Prepare(modelpkg.Model{
	Verts: []math3d.Vec3{
		{X: -1.1}, {X: 1.1}, {Y: -1.1}, {Y: 1.1}, {Z: -1.1}, {Z: 1.1},
		{X: -0.65, Y: -0.65}, {X: 0.65, Y: 0.65}, {X: -0.65, Z: -0.65}, {X: 0.65, Z: 0.65},
	},
	Edges: []modelpkg.Edge{
		{A: 0, B: 1, Kind: modelpkg.EdgeDecorative},
		{A: 2, B: 3, Kind: modelpkg.EdgeDecorative},
		{A: 4, B: 5, Kind: modelpkg.EdgeDecorative},
		{A: 6, B: 7, Kind: modelpkg.EdgeDecorative},
		{A: 8, B: 9, Kind: modelpkg.EdgeDecorative},
	},
})

var installationShardMesh = modelpkg.Cube(0.35)

// surfaceEncounterState is authoritative fixed-tick state for one bound
// environment instance. It is independent of which visual tiles survive the
// render stream, so encounter timing does not change with camera or LOD.
type surfaceEncounterState struct {
	started       bool
	participant   scene.ObjectID
	nextWaveAt    float64
	wave          int
	missionPhase  sim.MissionPhase
	cannonReadyAt map[string]float64
	cannonAim     map[string]cannonAimState
}

type cannonAimState struct {
	yaw, pitch float64
	shots      uint64
}

type localEnvironment struct {
	bound            environment.Bound
	tiles            map[environment.TileCoordinate]environment.Tile
	horizonTiles     map[environment.TileCoordinate]environment.Tile
	featureStates    map[string]featureDamageState
	detailLevels     map[string]scene.DetailTier
	tileDetailLevels map[environment.TileCoordinate]scene.DetailTier
	encounter        surfaceEncounterState
}

type environmentTransition struct {
	objectID      scene.ObjectID
	destination   scene.FrameID
	anchor        string
	duration      float64
	elapsed       float64
	startPose     kinematics.Pose
	targetPose    kinematics.Pose
	entryPose     kinematics.Pose
	motion        kinematics.Motion
	worldVelocity math3d.Vec3
	previousMode  camera.Mode
	rollRadians   float64
	preservePose  bool
	entryOffset   math3d.Vec3
}

// hyperspaceArrival is a presentation-only orbital-space entry. The
// authoritative fighter remains in the exterior frame throughout the effect;
// the world tick is paused while its pose eases in from a short distance
// behind the normal spawn pose. Surface-frame starts deliberately bypass it.
type hyperspaceArrival struct {
	elapsed      float64
	duration     float64
	from         kinematics.Pose
	to           kinematics.Pose
	motion       kinematics.Motion
	previousMode camera.Mode
}

type worldRenderJob struct {
	geometry      *render.PreparedGeometry
	depth         *render.DepthBuffer
	owner         uint64
	selfOcclusion render.SelfOcclusionMode
	objectID      scene.ObjectID
	color         color.Color
	lineWidth     float32
	portalClip    []render.Point
}

type preparedCandidate struct {
	geometry       *render.PreparedGeometry
	mesh           modelpkg.Model
	world          math3d.Mat4
	owner          uint64
	objectID       scene.ObjectID
	color          color.Color
	lineWidth      float32
	portalClip     []render.Point
	surface        scene.SurfaceMaterial
	selfOcclusion  render.SelfOcclusionMode
	group          depthDomainID
	domain         depthDomainID
	writesDepth    bool
	testsDepth     bool
	pointOccluder  bool
	analyticSphere bool
	object         scene.Object
	billboard      appearance.Billboard
	isBillboard    bool
}

type depthDomainKind uint8

const (
	depthDomainNone depthDomainKind = iota
	depthDomainScene
	depthDomainObject
	depthDomainEnvironment
)

type depthDomainID struct {
	kind depthDomainKind
	id   scene.ObjectID
}

func (domain depthDomainID) active() bool { return domain.kind != depthDomainNone }

type preparedDepthDomain struct {
	id      depthDomainID
	members []int
	writers []int
}

type preparedFrame struct {
	view         view.Context
	frame        scene.FrameID
	candidates   []preparedCandidate
	geometries   []render.PreparedGeometry
	domains      []preparedDepthDomain
	domainLookup map[depthDomainID]int
	unscoped     []int
}

type worldRenderResult struct {
	lines     []render.Line
	stats     render.Stats
	objectID  scene.ObjectID
	color     color.Color
	lineWidth float32
}

func partSelfOcclusionMode(part scene.Part, destruction bool) render.SelfOcclusionMode {
	if destruction || part.SelfOccluding {
		return render.SelfOcclusionAll
	}
	switch part.SelfOcclusion {
	case scene.SelfOcclusionInterior:
		return render.SelfOcclusionInterior
	case scene.SelfOcclusionAll:
		return render.SelfOcclusionAll
	default:
		return render.SelfOcclusionNone
	}
}

func (g *Game) profileRequestsSceneDepth() bool {
	return g.realismLevel >= 3
}

func (g *Game) objectPartVisible(object scene.Object, part scene.Part, detail scene.DetailTier) bool {
	if part.Detail > detail {
		return false
	}
	insideTargetCockpit := g.viewCamera.Mode == camera.Cockpit && object.ID == g.viewCamera.TargetID
	if insideTargetCockpit && !part.VisibleInCockpit {
		return false
	}
	return !(!insideTargetCockpit && part.CockpitOnly)
}

var sceneDepthDomain = depthDomainID{kind: depthDomainScene}

func objectDepthGroup(id scene.ObjectID) depthDomainID {
	return depthDomainID{kind: depthDomainObject, id: id}
}

func environmentDepthGroup(id scene.ObjectID) depthDomainID {
	return depthDomainID{kind: depthDomainEnvironment, id: id}
}

func (g *Game) resetPreparedFrame(frame scene.FrameID) *preparedFrame {
	prepared := &g.prepared
	prepared.view = g.viewContext
	prepared.frame = frame
	prepared.candidates = prepared.candidates[:0]
	prepared.geometries = prepared.geometries[:0]
	prepared.domains = prepared.domains[:0]
	prepared.unscoped = prepared.unscoped[:0]
	if prepared.domainLookup == nil {
		prepared.domainLookup = make(map[depthDomainID]int)
	} else {
		clear(prepared.domainLookup)
	}
	return prepared
}

// prepareCandidateGeometry performs the one per-frame camera transform and
// visibility classification for every concrete candidate. Depth and sparse
// point occlusion request projected surface triangles; line-only candidates
// avoid that additional work.
func (g *Game) prepareCandidateGeometry(prepared *preparedFrame) {
	count := 0
	for index := range prepared.candidates {
		candidate := &prepared.candidates[index]
		candidate.geometry = nil
		if !candidate.isBillboard && !candidate.analyticSphere {
			count++
		}
	}
	if cap(prepared.geometries) < count {
		prepared.geometries = make([]render.PreparedGeometry, count)
	} else {
		prepared.geometries = prepared.geometries[:count]
	}
	geometryIndex := 0
	for index := range prepared.candidates {
		candidate := &prepared.candidates[index]
		if candidate.isBillboard || candidate.analyticSphere {
			continue
		}
		geometry := &prepared.geometries[geometryIndex]
		g.pipeline.PrepareGeometryInto(geometry, candidate.mesh, candidate.world, candidate.writesDepth || candidate.pointOccluder || candidate.surface.Filled())
		if len(candidate.portalClip) > 0 {
			render.ClipPreparedTrianglesToConvex(geometry, candidate.portalClip)
		}
		candidate.geometry = geometry
		geometryIndex++
	}
}

func depthWriteCandidate(part scene.Part) bool {
	return len(part.Mesh.Faces) > 0 && !part.Mesh.SkipDepth && !part.Mesh.DepthTestOnly && !part.Surface.Translucent()
}

func depthTestCandidate(part scene.Part) bool {
	return !part.Mesh.SkipDepth || part.Mesh.DepthTestOnly
}

func pointOcclusionCandidate(part scene.Part) bool {
	return len(part.Mesh.Faces) > 0 && (!part.Mesh.SkipDepth || part.Mesh.PointOccluder) && !part.Surface.Filled()
}

func addPreparedDomain(prepared *preparedFrame, id depthDomainID) *preparedDepthDomain {
	if index, ok := prepared.domainLookup[id]; ok {
		return &prepared.domains[index]
	}
	index := len(prepared.domains)
	if index < cap(prepared.domains) {
		prepared.domains = prepared.domains[:index+1]
		domain := &prepared.domains[index]
		domain.id = id
		domain.members = domain.members[:0]
		domain.writers = domain.writers[:0]
	} else {
		prepared.domains = append(prepared.domains, preparedDepthDomain{id: id})
	}
	prepared.domainLookup[id] = index
	return &prepared.domains[index]
}

func (g *Game) assignPreparedDepth(prepared *preparedFrame) {
	profile := g.profileRequestsSceneDepth()
	physical := false
	for _, candidate := range prepared.candidates {
		physical = physical || candidate.writesDepth
	}
	if profile && physical {
		addPreparedDomain(prepared, sceneDepthDomain)
		for index := range prepared.candidates {
			candidate := &prepared.candidates[index]
			if candidate.writesDepth || candidate.testsDepth {
				candidate.domain = sceneDepthDomain
			}
		}
	} else {
		for _, candidate := range prepared.candidates {
			if candidate.writesDepth && candidate.selfOcclusion != render.SelfOcclusionNone {
				addPreparedDomain(prepared, candidate.group)
			}
		}
		for index := range prepared.candidates {
			candidate := &prepared.candidates[index]
			if _, ok := prepared.domainLookup[candidate.group]; ok && (candidate.writesDepth || candidate.testsDepth) {
				candidate.domain = candidate.group
			}
		}
	}
	for index := range prepared.candidates {
		candidate := &prepared.candidates[index]
		if !candidate.domain.active() {
			prepared.unscoped = append(prepared.unscoped, index)
			continue
		}
		domain := addPreparedDomain(prepared, candidate.domain)
		domain.members = append(domain.members, index)
		if candidate.writesDepth {
			domain.writers = append(domain.writers, index)
		}
	}
	if g.pipeline.Stats != nil {
		g.pipeline.Stats.ActiveDepthDomains = len(prepared.domains)
		g.pipeline.Stats.DepthEnabledByProfile = profile && physical
		for _, candidate := range prepared.candidates {
			if candidate.writesDepth {
				g.pipeline.Stats.DepthCandidateParts++
			}
			if candidate.domain.active() && candidate.selfOcclusion != render.SelfOcclusionNone {
				g.pipeline.Stats.DepthEnabledBySelfOcclusion = true
			}
			if candidate.writesDepth && candidate.domain.active() {
				g.pipeline.Stats.DepthWritingCandidates++
			}
			if candidate.testsDepth && candidate.domain.active() {
				g.pipeline.Stats.DepthTestingCandidates++
			}
		}
	}
}

func (g *Game) prepareGameplayFrame() *preparedFrame {
	prepared := g.resetPreparedFrame(g.viewContext.FrameID)
	for _, object := range g.objects {
		if !g.swarmLaunched {
			if _, autonomous := g.controllers[object.ID]; autonomous {
				continue
			}
		}
		if normalizedObjectFrame(object) != prepared.frame {
			continue
		}
		if g.pipeline.Stats != nil {
			g.pipeline.Stats.ObjectsInput++
		}
		if !g.objectInView(object) {
			if g.pipeline.Stats != nil {
				g.pipeline.Stats.ObjectsCulled++
				g.pipeline.Stats.ObjectsBoundsRejected++
			}
			continue
		}
		definition, hasAppearance := g.appearanceRegistry.ForObject(object.Definition, object.Appearance)
		if hasAppearance && definition.Kind == "vector-billboard" {
			prepared.candidates = append(prepared.candidates, preparedCandidate{
				object: object, billboard: definition.Billboard, isBillboard: true,
				analyticSphere: definition.PointOccluder == "sphere",
			})
			continue
		}
		candidateStart := len(prepared.candidates)
		detail, world := g.objectDetailTier(object), object.WorldMatrix()
		group := objectDepthGroup(object.ID)
		if transient, ok := g.debris[object.ID]; ok && transient.rootObjectID != 0 {
			group = objectDepthGroup(transient.rootObjectID)
		}
		for partIndex, part := range object.Parts {
			if !g.objectPartVisible(object, part, detail) {
				continue
			}
			prepared.candidates = append(prepared.candidates, preparedCandidate{
				mesh: part.Mesh, world: world, owner: renderOwner(object.ID, partIndex), objectID: object.ID,
				color: part.Color, lineWidth: part.LineWidth, surface: part.Surface, group: group,
				selfOcclusion: partSelfOcclusionMode(part, object.DestructionStage >= scene.DestructionComponent),
				writesDepth:   depthWriteCandidate(part), testsDepth: depthTestCandidate(part),
				pointOccluder: pointOcclusionCandidate(part) && !(hasAppearance && definition.PointOccluder == "sphere"),
			})
		}
		if hasAppearance && definition.PointOccluder == "sphere" {
			prepared.candidates = append(prepared.candidates, preparedCandidate{analyticSphere: true, object: object})
		}
		if g.pipeline.Stats != nil {
			for _, candidate := range prepared.candidates[candidateStart:] {
				if candidate.writesDepth {
					g.pipeline.Stats.DepthCandidateObjects++
					break
				}
			}
		}
	}
	for runtimeIndex := range g.environments {
		runtime := &g.environments[runtimeIndex]
		if runtime.bound.FrameID != prepared.frame {
			continue
		}
		for _, tile := range runtime.tiles {
			g.appendEnvironmentTileCandidates(prepared, runtime, tile, math3d.Identity())
		}
		for _, tile := range runtime.horizonTiles {
			g.appendEnvironmentTileCandidates(prepared, runtime, tile, math3d.Identity())
		}
	}
	g.appendRoomCandidates(prepared)
	g.appendPortalCandidates(prepared)
	g.appendTransitionEnvironmentCandidates(prepared)
	g.prepareCandidateGeometry(prepared)
	g.assignPreparedDepth(prepared)
	if g.pipeline.Stats != nil {
		g.pipeline.Stats.CandidatesPrepared = len(prepared.candidates)
	}
	return prepared
}

// portalPass is an opening expressed in the active frame. The same authored
// room portal can be viewed and crossed from either side; neither the renderer
// nor projectile simulation needs to know whether it connects to a surface,
// another room, or a future mission environment.
type portalPass struct {
	destination scene.FrameID
	boundary    []math3d.Vec3
}

func (g *Game) portalPasses(frame scene.FrameID) []portalPass {
	if g.world == nil || g.environmentRegistry == nil {
		return nil
	}
	activePose, err := g.world.FramePose(frame)
	if err != nil {
		return nil
	}
	var passes []portalPass
	for _, runtime := range g.environments {
		room, ok := g.environmentRegistry.Room(runtime.bound.FrameID)
		if !ok {
			continue
		}
		for _, portal := range room.Portals {
			if frame != room.Frame && frame != portal.Destination {
				continue
			}
			pass := portalPass{destination: portal.Destination, boundary: portal.Boundary}
			if frame != room.Frame {
				roomPose, err := g.world.FramePose(room.Frame)
				if err != nil {
					continue
				}
				matrix := kinematics.Relative(activePose, roomPose).Matrix()
				pass.destination = room.Frame
				pass.boundary = make([]math3d.Vec3, len(portal.Boundary))
				for index, point := range portal.Boundary {
					pass.boundary[index] = matrix.TransformPoint(point)
				}
			}
			passes = append(passes, pass)
		}
	}
	return passes
}

// A room can expose a small neighboring environment through an open portal.
// Destination candidates retain the projected opening so their triangles and
// lines are clipped before depth, point occlusion and draw submission.
func (g *Game) appendPortalCandidates(prepared *preparedFrame) {
	if g.world == nil || g.environmentRegistry == nil {
		return
	}
	activePose, err := g.world.FramePose(prepared.frame)
	if err != nil {
		return
	}
	for _, portal := range g.portalPasses(prepared.frame) {
		if len(portal.boundary) == 0 {
			continue
		}
		center := math3d.Vec3{}
		radius := 0.0
		for _, point := range portal.boundary {
			center = center.Add(point)
		}
		center = center.Scale(1 / float64(len(portal.boundary)))
		for _, point := range portal.boundary {
			radius = max(radius, point.Sub(center).Length())
		}
		if visible, _ := g.renderBoundsInView(environment.RenderBounds{Center: center, Radius: radius}, math3d.Identity()); !visible {
			continue
		}
		destinationPose, err := g.world.FramePose(portal.destination)
		if err != nil {
			continue
		}
		aperture := make([]render.Point, len(portal.boundary))
		projectable := true
		for index, point := range portal.boundary {
			aperture[index], projectable = g.pipeline.ProjectPointUnclipped(point)
			if !projectable {
				break
			}
		}
		if !projectable {
			continue
		}
		frameWorld := kinematics.Relative(activePose, destinationPose).Matrix()
		if room, ok := g.environmentRegistry.Room(portal.destination); ok {
			g.appendPortalRoomCandidates(prepared, room, frameWorld, aperture)
		}
		for index := range g.environments {
			runtime := &g.environments[index]
			if runtime.bound.FrameID != portal.destination {
				continue
			}
			for _, tile := range runtime.tiles {
				if tile.PortalParts != nil {
					tile.Parts = tile.PortalParts
				}
				start := len(prepared.candidates)
				g.appendEnvironmentTileCandidates(prepared, runtime, tile, frameWorld)
				for candidateIndex := start; candidateIndex < len(prepared.candidates); candidateIndex++ {
					prepared.candidates[candidateIndex].group = environmentDepthGroup(0)
					prepared.candidates[candidateIndex].portalClip = aperture
				}
			}
		}
		g.appendPortalObjectCandidates(prepared, portal.destination, frameWorld, aperture)
	}
}

func (g *Game) appendPortalRoomCandidates(prepared *preparedFrame, room environment.Room, frameWorld math3d.Mat4, aperture []render.Point) {
	if room.Bounds.Valid() {
		if visible, _ := g.renderBoundsInView(room.Bounds, frameWorld); !visible {
			return
		}
	}
	for partIndex, part := range room.Parts {
		if !g.meshInView(part.Mesh, frameWorld) {
			continue
		}
		prepared.candidates = append(prepared.candidates, preparedCandidate{
			mesh: part.Mesh, world: frameWorld, owner: environmentPartOwner(0, environment.TileCoordinate{}, "portal-room/"+string(room.Frame), partIndex),
			color: part.Color, lineWidth: part.LineWidth, surface: part.Surface, group: environmentDepthGroup(0), portalClip: aperture,
			selfOcclusion: partSelfOcclusionMode(part, false), writesDepth: depthWriteCandidate(part),
			testsDepth: depthTestCandidate(part), pointOccluder: pointOcclusionCandidate(part),
		})
	}
}

func (g *Game) appendPortalObjectCandidates(prepared *preparedFrame, destination scene.FrameID, frameWorld math3d.Mat4, aperture []render.Point) {
	for _, object := range g.objects {
		if normalizedObjectFrame(object) != destination {
			continue
		}
		if !g.swarmLaunched {
			if _, autonomous := g.controllers[object.ID]; autonomous {
				continue
			}
		}
		world := frameWorld.Mul(object.WorldMatrix())
		viewObject := object
		viewObject.Pose.Position = frameWorld.TransformPoint(object.Pose.Position)
		if !g.objectInView(viewObject) {
			continue
		}
		detail := g.objectDetailTier(viewObject)
		group := objectDepthGroup(object.ID)
		for partIndex, part := range object.Parts {
			if !g.objectPartVisible(object, part, detail) {
				continue
			}
			prepared.candidates = append(prepared.candidates, preparedCandidate{
				mesh: part.Mesh, world: world, owner: renderOwner(object.ID, partIndex), objectID: object.ID,
				color: part.Color, lineWidth: part.LineWidth, surface: part.Surface, group: group, portalClip: aperture,
				selfOcclusion: partSelfOcclusionMode(part, object.DestructionStage >= scene.DestructionComponent),
				writesDepth:   depthWriteCandidate(part), testsDepth: depthTestCandidate(part), pointOccluder: pointOcclusionCandidate(part),
			})
		}
	}
}

// appendRoomCandidates gives enclosed environments the same immutable,
// visibility-first preparation path as every other physical scene surface.
// Room authors model wall geometry around openings rather than drawing masks.
func (g *Game) appendRoomCandidates(prepared *preparedFrame) {
	room, ok := g.environmentRegistry.Room(prepared.frame)
	if !ok {
		return
	}
	world := math3d.Identity()
	if room.Bounds.Valid() {
		visible, _ := g.renderBoundsInView(room.Bounds, world)
		if !visible {
			return
		}
	}
	group := environmentDepthGroup(0)
	for partIndex, part := range room.Parts {
		if !g.meshInView(part.Mesh, world) {
			if g.pipeline.Stats != nil {
				g.pipeline.Stats.EnvironmentPartsBoundsRejected++
			}
			continue
		}
		prepared.candidates = append(prepared.candidates, preparedCandidate{
			mesh: part.Mesh, world: world,
			owner: environmentPartOwner(0, environment.TileCoordinate{}, "room/"+string(room.Frame), partIndex),
			color: part.Color, lineWidth: part.LineWidth, surface: part.Surface, group: group,
			selfOcclusion: partSelfOcclusionMode(part, false), writesDepth: depthWriteCandidate(part),
			testsDepth: depthTestCandidate(part), pointOccluder: pointOcclusionCandidate(part),
		})
	}
}

// appendEnvironmentTileCandidates is the common preparation boundary for
// active surface geometry and environment geometry shown during an approach
// presentation. frameWorld expresses the environment frame in the camera's
// current frame; active environments therefore pass identity while exterior
// transition/cut-scene views pass the destination frame's world transform.
func (g *Game) appendEnvironmentTileCandidates(prepared *preparedFrame, runtime *localEnvironment, tile environment.Tile, frameWorld math3d.Mat4) {
	if g.pipeline.Stats != nil {
		g.pipeline.Stats.EnvironmentTilesInput++
	}
	tileDetail := scene.DetailNear
	if tile.Bounds.Valid() {
		visible, projectedRadius := g.renderBoundsInView(tile.Bounds, frameWorld)
		if !visible {
			if g.pipeline.Stats != nil {
				g.pipeline.Stats.EnvironmentTilesBoundsRejected++
			}
			return
		}
		tileDetail = g.environmentTileDetailTier(runtime, tile.Coordinate, projectedRadius)
	}
	if g.pipeline.Stats != nil {
		g.pipeline.Stats.ActiveEnvironmentTiles++
	}
	group := environmentDepthGroup(runtime.bound.HostID)
	for partIndex, part := range tile.Parts {
		if part.Detail > tileDetail {
			if g.pipeline.Stats != nil {
				g.pipeline.Stats.EnvironmentPartsLODRejected++
			}
			continue
		}
		if !g.meshInView(part.Mesh, frameWorld) {
			if g.pipeline.Stats != nil {
				g.pipeline.Stats.EnvironmentPartsBoundsRejected++
			}
			continue
		}
		prepared.candidates = append(prepared.candidates, preparedCandidate{
			mesh: part.Mesh, world: frameWorld,
			owner: environmentPartOwner(runtime.bound.HostID, tile.Coordinate, "tile", partIndex),
			color: part.Color, lineWidth: part.LineWidth, surface: part.Surface, group: group,
			selfOcclusion: partSelfOcclusionMode(part, false), writesDepth: depthWriteCandidate(part),
			testsDepth: depthTestCandidate(part), pointOccluder: pointOcclusionCandidate(part),
		})
	}
	for _, feature := range tile.Features {
		state := runtime.featureStates[feature.ID]
		parts := feature.Parts
		if state.Destroyed {
			parts = feature.WreckParts
		}
		if len(parts) == 0 {
			continue
		}
		if g.pipeline.Stats != nil {
			g.pipeline.Stats.EnvironmentFeaturesInput++
		}
		world := frameWorld.Mul(feature.Matrix())
		detail := scene.DetailNear
		if feature.Bounds.Valid() {
			visible, projectedRadius := g.renderBoundsInView(feature.Bounds, world)
			if !visible {
				if g.pipeline.Stats != nil {
					g.pipeline.Stats.EnvironmentFeaturesBoundsRejected++
				}
				continue
			}
			detail = g.environmentFeatureDetailTier(runtime, feature.ID, projectedRadius)
			if detail < feature.Detail {
				if g.pipeline.Stats != nil {
					g.pipeline.Stats.EnvironmentFeaturesLODRejected++
				}
				continue
			}
		}
		visible := false
		lodEligible := false
		for partIndex, part := range parts {
			if part.Detail > detail {
				if g.pipeline.Stats != nil {
					g.pipeline.Stats.EnvironmentPartsLODRejected++
				}
				continue
			}
			lodEligible = true
			partWorld := world
			if partIndex == feature.TurretPart && feature.TurretPart > 0 && !state.Destroyed {
				aim := runtime.encounter.cannonAim[feature.ID]
				partWorld = world.Mul(feature.TurretMatrix(aim.yaw, 0))
			} else if partIndex == feature.BarrelPart && feature.BarrelPart > 0 && !state.Destroyed {
				aim := runtime.encounter.cannonAim[feature.ID]
				partWorld = world.Mul(feature.TurretMatrix(aim.yaw, aim.pitch))
			}
			if !g.meshInView(part.Mesh, partWorld) {
				if g.pipeline.Stats != nil {
					g.pipeline.Stats.EnvironmentPartsBoundsRejected++
				}
				continue
			}
			visible = true
			lineColor := part.Color
			if state.Hits > 0 && !state.Destroyed {
				lineColor = color.RGBA{R: 255, G: 160, B: 48, A: 255}
			}
			prepared.candidates = append(prepared.candidates, preparedCandidate{
				mesh: part.Mesh, world: partWorld,
				owner: environmentPartOwner(runtime.bound.HostID, tile.Coordinate, feature.ID, partIndex),
				color: lineColor, lineWidth: part.LineWidth, surface: part.Surface, group: group,
				selfOcclusion: partSelfOcclusionMode(part, false), writesDepth: depthWriteCandidate(part),
				testsDepth: depthTestCandidate(part), pointOccluder: pointOcclusionCandidate(part),
			})
		}
		if !visible && lodEligible && g.pipeline.Stats != nil {
			g.pipeline.Stats.EnvironmentFeaturesBoundsRejected++
		} else if visible && g.pipeline.Stats != nil {
			g.pipeline.Stats.EnvironmentInstancesPrepared++
		}
	}
}

// appendTransitionEnvironmentCandidates adds the destination patch visible in
// the controlled craft's approach presentation. The patch is already acquired
// during Update, so preparation remains a read-only classification pass and
// all transition geometry follows the same depth, point-occlusion, clipping,
// batching, and instrumentation path as ordinary world geometry.
func (g *Game) appendTransitionEnvironmentCandidates(prepared *preparedFrame) {
	transition, runtime, ok := g.viewTransitionEnvironment()
	if !ok || runtime.bound.FrameID == prepared.frame {
		return
	}
	framePose, err := g.world.FramePose(transition.destination)
	if err != nil {
		return
	}
	frameWorld := framePose.Matrix()
	for tileX := -transitionEnvironmentTileRadius; tileX <= transitionEnvironmentTileRadius; tileX++ {
		for tileZ := -transitionEnvironmentTileRadius; tileZ <= transitionEnvironmentTileRadius; tileZ++ {
			coordinate := environment.TileCoordinate{X: tileX, Z: tileZ}
			tile, exists := runtime.tiles[coordinate]
			if !exists {
				continue
			}
			g.appendEnvironmentTileCandidates(prepared, runtime, tile, frameWorld)
		}
	}
}

func (g *Game) prepareShowcaseFrame() *preparedFrame {
	prepared := g.resetPreparedFrame(g.viewContext.FrameID)
	for _, object := range g.showcaseObjects {
		if !g.objectInView(object) {
			continue
		}
		world := object.WorldMatrix()
		for partIndex, part := range object.Parts {
			prepared.candidates = append(prepared.candidates, preparedCandidate{mesh: part.Mesh, world: world, owner: renderOwner(object.ID, partIndex), objectID: object.ID, color: part.Color, lineWidth: part.LineWidth, surface: part.Surface, group: objectDepthGroup(object.ID), selfOcclusion: partSelfOcclusionMode(part, false), writesDepth: depthWriteCandidate(part), testsDepth: depthTestCandidate(part), pointOccluder: pointOcclusionCandidate(part)})
		}
	}
	g.prepareCandidateGeometry(prepared)
	g.assignPreparedDepth(prepared)
	if g.pipeline.Stats != nil {
		g.pipeline.Stats.CandidatesPrepared = len(prepared.candidates)
	}
	return prepared
}

const (
	modeAutopilot flightMode = iota
	modeManual
)

const yavinMissionID = "battle-of-yavin"

const (
	// The Imperial screen starts ahead of the player and closes naturally as
	// the player advances. These are mission-design constraints, not visual or
	// collision thresholds: they keep the orbital approach a fight toward the
	// station rather than a free transition trigger.
	yavinApproachClosureFraction = 0.35
	yavinEngagementRange         = 180.0
	yavinEngagementSeconds       = 3.0
)

func (mode flightMode) String() string {
	if mode == modeManual {
		return "Manual"
	}
	return "Autopilot"
}

// Game owns the simulation state and wireframe rendering pipeline.
type Game struct {
	profile                  profile.GameProfile
	controllerRegistry       *control.Registry
	catalogRegistry          *catalog.Registry
	appearanceRegistry       *appearance.Registry
	cockpitRegistry          *cockpit.Registry
	environmentRegistry      *environment.Registry
	environmentDefinitions   []environment.Definition
	environments             []localEnvironment
	transitions              map[scene.ObjectID]environmentTransition
	transitionCommitments    map[scene.ObjectID]bool
	missionLastPosition      map[scene.ObjectID]math3d.Vec3
	objects                  []scene.Object
	pipeline                 render.Pipeline
	initialPose              kinematics.Pose
	autoMotion               kinematics.Motion
	mode                     flightMode
	paused                   bool
	quitPrompt               bool
	flow                     applicationFlow
	selectedMission          int
	selectedDifficulty       int
	curatedDifficulties      []profile.GameProfile
	launchProfile            profile.GameProfile
	swarmLaunched            bool
	showHUD                  bool
	surfaceAutoLevel         bool
	showcaseActive           bool
	showcaseTime             float64
	showcaseObjects          []scene.Object
	showcaseStarField        *starfield.Field
	showcaseDistance         float64
	showcaseRotating         bool
	showcaseTopDown          bool
	showcaseSelected         int
	showcaseSlide            float64
	showcasePreviousMode     camera.Mode
	viewCamera               *camera.Camera
	nextObjectID             scene.ObjectID
	projectiles              map[scene.ObjectID]float64
	owners                   map[scene.ObjectID]scene.ObjectID
	fireCooldown             float64
	torpedoCooldown          float64
	torpedoesRemaining       int
	simulationTime           float64
	fireHistory              []float64
	nextMuzzlePair           int
	laserBeamTime            float64
	laserBeamPair            int
	mouseFlight              bool
	mouseNeutralX            int
	mouseNeutralY            int
	starField                *starfield.Field
	controllers              map[scene.ObjectID]control.Strategy
	controllerTargets        map[scene.ObjectID]scene.ObjectID
	debris                   map[scene.ObjectID]destructionTransient
	surfaceEffects           map[scene.ObjectID]surfaceEffect
	environmentContacts      map[scene.ObjectID]float64
	respawns                 []autonomousRespawn
	respawnSequence          uint64
	playerDestroyed          bool
	playerViewMode           camera.Mode
	kills                    int
	collisions               int
	visibleObjects           int
	renderStats              render.Stats
	depthBuffer              *render.DepthBuffer
	billboardLineCache       map[string]map[int][]appearance.Line
	billboardBatches         []vectorLineBatch
	worldBatches             []vectorLineBatch
	worldJobs                []worldRenderJob
	prepared                 preparedFrame
	viewContext              view.Context
	visibleObjectIDs         map[scene.ObjectID]bool
	whitePixel               *ebiten.Image
	textureRegistry          *render.TextureRegistry
	textureImages            map[string]*ebiten.Image
	starVertices             []ebiten.Vertex
	starIndices              []uint16
	surfaceTriangles         []render.FlatTriangle
	surfaceVertices          []ebiten.Vertex
	surfaceIndices           []uint16
	starPoints               []starfield.Point
	starOccluders            render.PointOccluderSet
	showcaseStarPoints       []starfield.Point
	world                    *sim.World
	detailLevels             map[scene.ObjectID]scene.DetailTier
	shieldStrength           int
	shieldQuietTime          float64
	destructionViewRemaining float64
	destructionVictim        scene.Object
	controlsRemaining        float64
	controlsPinned           bool
	realismLevel             int
	hyperspaceArrival        *hyperspaceArrival
}

var renderingProfiles = []string{"builtin/arcade", "builtin/culled", "builtin/hidden-line", "builtin/depth-cue", "builtin/maximum"}

func New() *Game {
	game, err := NewWithProfile(profile.Pilot())
	if err != nil {
		panic(err)
	}
	return game
}

// NewWithProfile creates a game from a validated snapshot of the supplied
// profile. Subsequent caller changes do not affect the running session.
func NewWithProfile(gameProfile profile.GameProfile) (*Game, error) {
	return NewWithProfileAndRegistries(gameProfile, control.DefaultRegistry(), catalog.DefaultRegistry())
}

// NewWithProfileAndRegistry creates a game with caller-registered controller
// implementations while retaining the same profile validation and simulation
// authority boundaries.
func NewWithProfileAndRegistry(gameProfile profile.GameProfile, registry *control.Registry) (*Game, error) {
	return NewWithProfileAndRegistries(gameProfile, registry, catalog.DefaultRegistry())
}

// NewWithProfileAndRegistries creates a game with caller-registered controllers and object definitions.
func NewWithProfileAndRegistries(gameProfile profile.GameProfile, registry *control.Registry, catalogRegistry *catalog.Registry) (*Game, error) {
	return NewWithAllRegistries(gameProfile, registry, catalogRegistry, environment.DefaultRegistry())
}

// NewWithAllRegistries is the complete customization boundary for a session.
// Environment definitions use the same registration approach as controllers
// and catalog objects, so adding a large-object zone does not modify Game.
func NewWithAllRegistries(gameProfile profile.GameProfile, registry *control.Registry, catalogRegistry *catalog.Registry, environmentRegistry *environment.Registry) (*Game, error) {
	return NewWithRegistriesAndAppearances(gameProfile, registry, catalogRegistry, environmentRegistry, appearance.DefaultRegistry())
}

// NewWithRegistriesAndAppearances is the full customization boundary for
// logical objects, controllers, environments, and visual presentations.
func NewWithRegistriesAndAppearances(gameProfile profile.GameProfile, registry *control.Registry, catalogRegistry *catalog.Registry, environmentRegistry *environment.Registry, appearanceRegistry *appearance.Registry) (*Game, error) {
	gameProfile = gameProfile.Clone()
	if err := gameProfile.Validate(); err != nil {
		return nil, fmt.Errorf("create game: %w", err)
	}
	if registry == nil {
		return nil, fmt.Errorf("create game: controller registry is nil")
	}
	if catalogRegistry == nil {
		return nil, fmt.Errorf("create game: catalog registry is nil")
	}
	if environmentRegistry == nil {
		return nil, fmt.Errorf("create game: environment registry is nil")
	}
	if appearanceRegistry == nil {
		return nil, fmt.Errorf("create game: appearance registry is nil")
	}
	initialPose := gameProfile.Player.InitialPose
	autoMotion := gameProfile.Player.AutopilotMotion
	fighter, err := catalogRegistry.Create(gameProfile.Player.Object, fighterID, initialPose)
	if err != nil {
		return nil, fmt.Errorf("create player object: %w", err)
	}
	fighter.Motion = autoMotion
	fighter.Team = gameProfile.Player.Team
	objects := []scene.Object{fighter}
	controllers := make(map[scene.ObjectID]control.Strategy, gameProfile.Swarm.Count)
	for index, pose := range autonomousFighterPoses(gameProfile.Swarm.InitialPositions) {
		id := scene.ObjectID(index + 2)
		definition := gameProfile.Swarm.Object
		if definition == catalog.TIEFighterName && index < swarmInterceptorSlots {
			definition = catalog.TIEInterceptorName
		}
		autonomous, err := catalogRegistry.Create(definition, id, pose)
		if err != nil {
			return nil, fmt.Errorf("create swarm object: %w", err)
		}
		autonomous.Motion.Speed = gameProfile.Swarm.InitialSpeed + float64(index)*gameProfile.Swarm.SpeedStep
		autonomous.Team = gameProfile.Swarm.Team
		objects = append(objects, autonomous)
		controller, err := registry.Create(gameProfile.Swarm.Controller, uint64(id)*0x9e3779b97f4a7c15, gameProfile.Swarm.Pursuit)
		if err != nil {
			return nil, err
		}
		controllers[id] = controller
	}
	nextObjectID := scene.ObjectID(gameProfile.Swarm.Count + 2)
	for _, placement := range gameProfile.World.Objects {
		object, err := catalogRegistry.Create(placement.Definition, nextObjectID, placement.Pose)
		if err != nil {
			return nil, fmt.Errorf("create world object %q: %w", placement.Definition, err)
		}
		object.Appearance = placement.Appearance
		objects = append(objects, object)
		nextObjectID++
	}
	viewCamera := camera.New(fighterID)
	viewCamera.Mode = camera.Cockpit
	curatedDifficulties := profile.Builtins()
	game := &Game{
		profile:                gameProfile,
		controllerRegistry:     registry,
		catalogRegistry:        catalogRegistry,
		appearanceRegistry:     appearanceRegistry,
		cockpitRegistry:        cockpit.DefaultRegistry(),
		environmentRegistry:    environmentRegistry,
		environmentDefinitions: environmentRegistry.Definitions(),
		pipeline:               render.NewPipeline(ScreenWidth, ScreenHeight, gameProfile.Display.VerticalFOV, gameProfile.Display.NearPlane, gameProfile.Display.FarPlane),
		objects:                objects,
		initialPose:            initialPose,
		autoMotion:             autoMotion,
		viewCamera:             viewCamera,
		nextObjectID:           nextObjectID,
		projectiles:            make(map[scene.ObjectID]float64),
		owners:                 make(map[scene.ObjectID]scene.ObjectID),
		starField:              starfield.New(gameProfile.Starfield.Count, gameProfile.Starfield.Seed, gameProfile.Starfield.Radius, initialPose.Position),
		controllers:            controllers,
		controllerTargets:      make(map[scene.ObjectID]scene.ObjectID),
		debris:                 make(map[scene.ObjectID]destructionTransient),
		surfaceEffects:         make(map[scene.ObjectID]surfaceEffect),
		environmentContacts:    make(map[scene.ObjectID]float64),
		transitions:            make(map[scene.ObjectID]environmentTransition),
		transitionCommitments:  make(map[scene.ObjectID]bool),
		missionLastPosition:    make(map[scene.ObjectID]math3d.Vec3),
		respawnSequence:        uint64(gameProfile.Swarm.Count),
		shieldStrength:         gameProfile.Player.Shield.Maximum,
		torpedoesRemaining:     gameProfile.Combat.Torpedo.Ammunition,
		flow:                   flowTitle,
		selectedDifficulty:     difficultyIndex(curatedDifficulties, gameProfile.Name),
		curatedDifficulties:    curatedDifficulties,
		launchProfile:          gameProfile.Clone(),
		swarmLaunched:          true,
		showHUD:                false,
		surfaceAutoLevel:       gameProfile.Player.AutoLevel.Enabled,
		controlsRemaining:      gameProfile.Display.ControlsDisplayDuration,
		detailLevels:           make(map[scene.ObjectID]scene.DetailTier),
		billboardLineCache:     make(map[string]map[int][]appearance.Line),
		textureRegistry:        render.DefaultTextureRegistry(),
		textureImages:          make(map[string]*ebiten.Image),
	}
	starfieldMode := gameProfile.Starfield.Mode
	if starfieldMode == "" {
		starfieldMode = profile.StarfieldModeSkyfield
	}
	game.starField.SetMode(starfieldMode, gameProfile.Display.FarPlane*0.85)
	game.pipeline.Stages = render.StagesForProfile(gameProfile.Display.RenderingProfile)
	game.showcaseObjects = game.createShowcaseObjects()
	game.showcaseStarField = starfield.New(500, gameProfile.Starfield.Seed+101, 90, math3d.Vec3{})
	game.showcaseStarField.SetMode(starfieldMode, gameProfile.Display.FarPlane*0.85)
	game.showcaseDistance = 16
	game.showcaseRotating = true
	game.showcaseTopDown = false
	world, err := sim.New(objects)
	if err != nil {
		return nil, fmt.Errorf("create simulation world: %w", err)
	}
	game.world = world
	if err := game.installEnvironments(); err != nil {
		return nil, fmt.Errorf("create local environments: %w", err)
	}
	for index, name := range renderingProfiles {
		if name == gameProfile.Display.RenderingProfile {
			game.realismLevel = index
			break
		}
	}
	game.pipeline.MinLinePixels = realismLineThreshold(game.realismLevel)
	game.pipeline.Stats = &game.renderStats
	game.refreshViewContext()
	return game, nil
}

func (g *Game) createShowcaseObjects() []scene.Object {
	objects := make([]scene.Object, 0, 4)
	for index, definition := range []string{catalog.XWingName, catalog.TIEFighterName, catalog.TIEInterceptorName, catalog.MillenniumFalconName} {
		object, err := g.catalogRegistry.Create(definition, scene.ObjectID(900000+index), kinematics.Pose{
			Position: math3d.Vec3{X: float64(index*2-1) * 5.5, Z: -40},
		})
		if err == nil {
			scale := 1.65
			// The Falcon is correctly much larger in gameplay units than a
			// fighter; use a smaller showcase distance scale so its full outline
			// remains readable inside the selector frame.
			if definition == catalog.MillenniumFalconName {
				scale = 0.68
			}
			for partIndex := range object.Parts {
				object.Parts[partIndex].Mesh = modelpkg.Transform(object.Parts[partIndex].Mesh, math3d.Scaling(scale, scale, scale))
			}
			objects = append(objects, object)
		}
	}
	return objects
}

func (g *Game) launchSwarm() {
	if g.swarmLaunched {
		return
	}
	g.swarmLaunched = true
	for id := range g.controllers {
		if fighter := g.objectByID(id); fighter != nil {
			fighter.Physical = true
			fighter.Hittable = true
			fighter.Destructible = true
			fighter.Targetable = true
		}
	}
}

func (g *Game) installEnvironments() error {
	for _, bound := range environment.Bind(g.environmentRegistry, g.objects) {
		if err := g.world.Apply(sim.RegisterFrame{Frame: sim.Frame{
			ID:          bound.FrameID,
			HostID:      bound.HostID,
			Environment: bound.Definition.Name,
			Pose:        bound.Definition.LocalPose,
		}}); err != nil {
			return err
		}
		runtime := localEnvironment{
			bound:            bound,
			tiles:            make(map[environment.TileCoordinate]environment.Tile),
			horizonTiles:     make(map[environment.TileCoordinate]environment.Tile),
			featureStates:    make(map[string]featureDamageState),
			detailLevels:     make(map[string]scene.DetailTier),
			tileDetailLevels: make(map[environment.TileCoordinate]scene.DetailTier),
			encounter: surfaceEncounterState{
				cannonReadyAt: make(map[string]float64),
				cannonAim:     make(map[string]cannonAimState),
			},
		}
		g.environments = append(g.environments, runtime)
		if bound.Definition.Name == environment.DeathStarHangarName {
			room := environment.DeathStarHangarRoom(bound.FrameID, bound.ResolveFrame(environment.DeathStarTrenchFrame))
			if err := g.environmentRegistry.RegisterRoom(room); err != nil {
				return err
			}
		}
	}
	g.objects = g.world.Objects
	return nil
}

func (g *Game) Update() error {
	seconds := g.profile.Simulation.TickSeconds
	questionPressed := questionKeyJustPressed()
	if questionPressed {
		g.toggleControls()
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyC) || (g.showcaseActive && inpututil.IsKeyJustPressed(ebiten.KeyEscape)) {
		g.showcaseActive = !g.showcaseActive
		g.showcaseTime = 0
		if g.showcaseActive {
			g.showcasePreviousMode = g.viewCamera.Mode
			g.viewCamera.Mode = camera.Fixed
			g.showcaseRotating = true
			g.showcaseTopDown = false
		} else {
			g.viewCamera.Mode = g.showcasePreviousMode
		}
	}
	if !g.showcaseActive && g.quitPrompt {
		if inpututil.IsKeyJustPressed(ebiten.KeyY) {
			return ebiten.Termination
		}
		if inpututil.IsKeyJustPressed(ebiten.KeyN) {
			g.quitPrompt = false
			g.paused = false
		}
		if inpututil.IsKeyJustPressed(ebiten.KeyEscape) {
			g.quitPrompt = false
			g.paused = false
		}
		g.refreshViewContext()
		return nil
	}
	if !g.showcaseActive && inpututil.IsKeyJustPressed(ebiten.KeyEscape) {
		g.paused = true
		g.quitPrompt = true
		g.refreshViewContext()
		return nil
	}
	if g.showcaseActive {
		g.handleRealismSliderClick()
		if inpututil.IsKeyJustPressed(ebiten.KeySpace) {
			g.showcaseRotating = !g.showcaseRotating
		}
		if inpututil.IsKeyJustPressed(ebiten.KeyT) {
			g.showcaseTopDown = !g.showcaseTopDown
		}
		if len(g.showcaseObjects) > 0 {
			if inpututil.IsKeyJustPressed(ebiten.KeyArrowLeft) || inpututil.IsKeyJustPressed(ebiten.KeyA) {
				g.showcaseSelected = (g.showcaseSelected + len(g.showcaseObjects) - 1) % len(g.showcaseObjects)
				g.showcaseSlide = -1
				g.showcaseTime = 0
			}
			if inpututil.IsKeyJustPressed(ebiten.KeyArrowRight) || inpututil.IsKeyJustPressed(ebiten.KeyD) {
				g.showcaseSelected = (g.showcaseSelected + 1) % len(g.showcaseObjects)
				g.showcaseSlide = 1
				g.showcaseTime = 0
			}
			g.showcaseSlide *= max(0, 1-seconds*5)
		}
		if inpututil.IsKeyJustPressed(ebiten.KeyBracketLeft) {
			g.setRealismLevel(g.realismLevel - 1)
		}
		if inpututil.IsKeyJustPressed(ebiten.KeyBracketRight) {
			g.setRealismLevel(g.realismLevel + 1)
		}
		_, wheelY := ebiten.Wheel()
		g.showcaseDistance = max(8, min(120, g.showcaseDistance-wheelY*2.5))
		if g.showcaseRotating {
			g.showcaseTime += seconds
		}
		mouseX, mouseY := ebiten.CursorPosition()
		yaw := (float64(mouseX) - ScreenWidth/2) / (ScreenWidth / 2) * 0.65
		pitch := (float64(mouseY) - ScreenHeight/2) / (ScreenHeight / 2) * 0.35
		if g.showcaseTopDown {
			pitch = -math.Pi / 2
		}
		angle := g.showcaseTime * 0.7
		for index := range g.showcaseObjects {
			offset := float64(index - g.showcaseSelected)
			half := float64(len(g.showcaseObjects)) / 2
			for offset > half {
				offset -= float64(len(g.showcaseObjects))
			}
			for offset < -half {
				offset += float64(len(g.showcaseObjects))
			}
			offset += g.showcaseSlide
			g.showcaseObjects[index].Pose.Position = math3d.Vec3{X: offset * 28, Z: -g.showcaseDistance - math.Abs(offset)*18}
			g.showcaseObjects[index].Pose.Orientation = math3d.QuaternionFromYawPitchRoll(angle+offset*0.25+yaw, 0.12+pitch, 0)
		}
		g.refreshViewContext()
		return nil
	}
	if handled, err := g.updateApplicationFlow(seconds); handled || err != nil {
		g.refreshViewContext()
		return err
	}
	g.updateControls(seconds)
	if inpututil.IsKeyJustPressed(ebiten.KeyM) {
		if g.mode == modeAutopilot {
			g.mode = modeManual
		} else {
			g.mode = modeAutopilot
			if fighter := g.objectByID(fighterID); fighter != nil {
				fighter.Motion = g.autoMotion
			}
		}
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyP) {
		g.paused = !g.paused
	}
	if inpututil.IsKeyJustPressed(ebiten.KeySpace) {
		g.showHUD = !g.showHUD
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyBracketLeft) {
		g.setRealismLevel(g.realismLevel - 1)
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyBracketRight) {
		g.setRealismLevel(g.realismLevel + 1)
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyR) {
		g.resetFighter()
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyV) {
		g.viewCamera.Cycle()
	}
	if !questionPressed && (inpututil.IsKeyJustPressed(ebiten.KeyShiftLeft) || inpututil.IsKeyJustPressed(ebiten.KeyShiftRight)) && len(g.controllers) > 0 {
		g.switchToRandomSwarmFollowView()
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyG) {
		g.toggleMouseFlight()
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyL) {
		g.surfaceAutoLevel = !g.surfaceAutoLevel
	}
	if g.hyperspaceArrival != nil {
		if !g.paused {
			g.advanceHyperspaceArrival(seconds)
		}
		g.refreshViewContext()
		return nil
	}
	if g.mode == modeAutopilot && navigationInputPressed() {
		g.mode = modeManual
	}
	if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonRight) && g.viewCamera.Mode == camera.Cockpit {
		g.mode = modeManual
		if g.mouseFlight {
			g.mouseFlight = false
			ebiten.SetCursorMode(ebiten.CursorModeVisible)
		}
	}
	if len(g.transitions) > 0 {
		if !g.paused {
			g.simulationTime += seconds
			// Keep the authoritative tick and unrelated world objects moving while
			// the controlled craft follows its presentation path. The transitioning
			// object's motion was cleared when the transition began.
			g.world.Objects = g.objects
			if err := g.world.Step(seconds * g.profile.Simulation.MotionScale); err != nil {
				return err
			}
			g.objects = g.world.Objects
			g.advanceEnvironmentTransitions(seconds, inpututil.IsKeyJustPressed(ebiten.KeyEscape))
			if err := g.updateYavinMission(); err != nil {
				return err
			}
		}
		g.refreshTransitionEnvironmentTiles()
		g.refreshViewContext()
		return nil
	}

	g.updateZoom()
	g.laserBeamTime = max(0, g.laserBeamTime-seconds)
	if g.mode == modeManual {
		if fighter := g.objectByID(fighterID); fighter != nil {
			intent := g.readIntent()
			motion := control.Apply(fighter.Motion, intent, g.playerFlightConfig(*fighter), seconds)
			fighter.Motion = g.applySurfaceAutoLevel(*fighter, intent, motion)
		}
	}
	if g.paused {
		g.refreshViewContext()
		return nil
	}
	g.simulationTime += seconds
	g.updateShield(seconds)
	g.updateEnvironmentContacts(seconds)
	if g.destructionViewRemaining > 0 {
		g.destructionViewRemaining -= seconds
		if g.destructionViewRemaining <= 0 {
			g.destructionViewRemaining = 0
		}
	}
	g.updateRespawns()
	g.updateSurfaceEncounters()
	g.updateAutonomous(seconds)
	g.fireCooldown = max(0, g.fireCooldown-seconds)
	g.torpedoCooldown = max(0, g.torpedoCooldown-seconds)
	if inpututil.IsKeyJustPressed(ebiten.KeyT) {
		g.fireProtonTorpedo()
	}
	if ebiten.IsKeyPressed(ebiten.KeyF) || ebiten.IsMouseButtonPressed(ebiten.MouseButtonLeft) {
		if g.showHUD && g.handleRealismSliderClick() {
			g.refreshViewContext()
			return nil
		}
		g.launchSwarm()
		g.fireLaser()
	}
	previousPositions := objectPositions(g.objects)
	motionSeconds := seconds * g.profile.Simulation.MotionScale
	// Keep the simulation world authoritative for fixed-tick kinematics. The
	// gameplay adapter may add/remove objects during collision and destruction
	// processing, so synchronize those structural changes before each tick.
	g.world.Objects = g.objects
	if err := g.world.Step(motionSeconds); err != nil {
		return err
	}
	g.objects = g.world.Objects
	if err := g.transferPortalProjectiles(previousPositions); err != nil {
		return err
	}
	if err := g.updateEnvironmentTransitions(); err != nil {
		return err
	}
	g.refreshEnvironmentTiles()
	g.refreshTransitionEnvironmentTiles()
	g.updateDebris(seconds)
	g.updateSurfaceEffects(seconds)
	g.resolveLaserCollisions(previousPositions)
	g.resolveSolidCollisions(previousPositions)
	g.resolveEnvironmentCollisions(previousPositions)
	g.updateProjectiles(seconds)
	if err := g.updateYavinMission(); err != nil {
		return err
	}
	if g.playerDestroyed && g.viewCamera.Mode == camera.Chase && g.objectByID(g.viewCamera.TargetID) == nil {
		g.switchToEnemyDestructionFollowView(g.destructionVictim, 0)
	}
	if reference := g.starfieldReferencePosition(); reference != nil {
		g.starField.Wrap(*reference)
	}
	g.viewCamera.Update(seconds)
	g.refreshViewContext()
	return nil
}

func (g *Game) deathStarSurfaceRuntime(frame scene.FrameID) *localEnvironment {
	for index := range g.environments {
		runtime := &g.environments[index]
		if runtime.bound.FrameID == frame && runtime.bound.Definition.Name == environment.DeathStarTrenchName {
			return runtime
		}
	}
	return nil
}

func (g *Game) startYavinMission(surfaceStart bool) error {
	if g.world == nil || g.world.Mission.Phase != sim.MissionInactive {
		return nil
	}
	hostID := scene.ObjectID(0)
	for index := range g.environments {
		if g.environments[index].bound.Definition.Name == environment.DeathStarTrenchName {
			hostID = g.environments[index].bound.HostID
			break
		}
	}
	if err := g.world.Apply(sim.StartMission{ID: yavinMissionID, PlayerID: fighterID, HostID: hostID}); err != nil {
		return err
	}
	if err := g.initializeYavinOrbitalProgress(); err != nil {
		return err
	}
	if surfaceStart {
		if err := g.world.Apply(
			sim.AdvanceMission{To: sim.MissionApproach, Reason: "surface-development-start"},
			sim.AdvanceMission{To: sim.MissionSurfaceAssault, Reason: "surface-entry"},
		); err != nil {
			return err
		}
	}
	return nil
}

func (g *Game) initializeYavinOrbitalProgress() error {
	if g.world == nil || g.world.Mission.ID != yavinMissionID || g.world.Mission.Phase != sim.MissionOrbitalBattle {
		return nil
	}
	player := g.objectByID(g.world.Mission.PlayerID)
	if player == nil {
		return nil
	}
	g.missionLastPosition[player.ID] = player.Pose.Position
	if g.world.Mission.HostID == 0 {
		return nil
	}
	hostPose, err := g.world.PoseInFrame(g.world.Mission.HostID, normalizedObjectFrame(*player))
	if err != nil {
		return err
	}
	return g.world.Apply(sim.ObserveMission{HostDistance: player.Pose.Position.Sub(hostPose.Position).Length()})
}

// updateYavinMission derives objective progress from authoritative craft and
// environment state. It never consults camera, render visibility or HUD state.
func (g *Game) updateYavinMission() error {
	if g.world == nil {
		return nil
	}
	mission := g.world.Mission
	if mission.ID != yavinMissionID || mission.Phase == sim.MissionInactive || mission.Phase == sim.MissionSucceeded || mission.Phase == sim.MissionFailed {
		return nil
	}
	player := g.objectByID(mission.PlayerID)
	if player == nil || g.playerDestroyed {
		return g.world.Apply(sim.FailMission{Reason: "fighter-destroyed"})
	}
	previous, known := g.missionLastPosition[player.ID]
	if !known {
		previous = player.Pose.Position
	}
	defer func() { g.missionLastPosition[player.ID] = player.Pose.Position }()
	frame := normalizedObjectFrame(*player)
	if g.world.Mission.Phase == sim.MissionOrbitalBattle {
		if err := g.observeOrbitalApproach(*player); err != nil {
			return err
		}
	}
	switch g.world.Mission.Phase {
	case sim.MissionOrbitalBattle:
		transition, approaching := g.transitions[player.ID]
		if !approaching || g.deathStarSurfaceRuntime(transition.destination) == nil {
			if g.deathStarSurfaceRuntime(frame) == nil {
				return nil
			}
		}
		return g.world.Apply(sim.AdvanceMission{To: sim.MissionApproach, Reason: "death-star-approach"})
	case sim.MissionApproach:
		if g.deathStarSurfaceRuntime(frame) == nil {
			return nil
		}
		return g.world.Apply(sim.AdvanceMission{To: sim.MissionSurfaceAssault, Reason: "surface-entry"})
	case sim.MissionSurfaceAssault:
		if g.deathStarSurfaceRuntime(frame) == nil || !environment.DeathStarTrenchEntryContains(player.Pose.Position) || player.Pose.Forward().Dot(math3d.Vec3{Z: 1}) <= 0.2 {
			return nil
		}
		return g.world.Apply(sim.AdvanceMission{To: sim.MissionTrenchRun, Reason: "trench-entry"})
	case sim.MissionTrenchRun:
		checkpoint := g.world.Mission.Progress.TrenchCheckpoint
		if player.Pose.Forward().Dot(math3d.Vec3{Z: 1}) <= 0.2 ||
			!environment.DeathStarTrenchCheckpointCrossed(checkpoint, previous, player.Pose.Position) {
			return nil
		}
		if err := g.world.Apply(sim.ObserveMission{TrenchCheckpoint: checkpoint + 1}); err != nil {
			return err
		}
		if g.world.Mission.Progress.TrenchCheckpoint < len(environment.DeathStarTrenchCheckpoints()) {
			return nil
		}
		return g.world.Apply(sim.AdvanceMission{To: sim.MissionExhaustPortAttack, Reason: "terminal-attack-run"})
	default:
		return nil
	}
}

// observeOrbitalApproach records only simulation-space facts: distance to the
// logical Death Star host and whether an Imperial fighter is actively close
// enough to form the defending screen. Rendering, camera and kills are not
// inputs to the approach rule.
func (g *Game) observeOrbitalApproach(player scene.Object) error {
	mission := g.world.Mission
	if mission.HostID == 0 {
		return nil
	}
	hostPose, err := g.world.PoseInFrame(mission.HostID, normalizedObjectFrame(player))
	if err != nil {
		return err
	}
	underEngagement := false
	for _, object := range g.objects {
		if !object.Targetable || object.Team == player.Team || object.Team == scene.TeamNeutral || normalizedObjectFrame(object) != normalizedObjectFrame(player) {
			continue
		}
		if object.Pose.Position.Sub(player.Pose.Position).Length() <= yavinEngagementRange {
			underEngagement = true
			break
		}
	}
	return g.world.Apply(sim.ObserveMission{
		HostDistance:    player.Pose.Position.Sub(hostPose.Position).Length(),
		UnderEngagement: underEngagement,
	})
}

func (g *Game) yavinApproachReady() bool {
	if g.world == nil || g.world.Mission.ID != yavinMissionID || g.world.Mission.Phase != sim.MissionOrbitalBattle {
		return true
	}
	progress := g.world.Mission.Progress
	if progress.OrbitalInitialHostDistance <= 0 || progress.OrbitalClosestHostDistance <= 0 {
		return false
	}
	closed := progress.OrbitalInitialHostDistance - progress.OrbitalClosestHostDistance
	return closed >= progress.OrbitalInitialHostDistance*yavinApproachClosureFraction &&
		float64(progress.OrbitalEngagementTicks)*g.profile.Simulation.TickSeconds >= yavinEngagementSeconds
}

// transferPortalProjectiles performs an authoritative frame change at the
// actual opening rather than treating a laser as a fighter approach cinematic.
// The remaining swept segment is tested in the destination frame this tick.
func (g *Game) transferPortalProjectiles(previous map[scene.ObjectID]math3d.Vec3) error {
	if g.world == nil {
		return nil
	}
	passesByFrame := make(map[scene.FrameID][]portalPass)
	for _, object := range g.objects {
		if object.CollisionRole != scene.CollisionProjectile {
			continue
		}
		from, ok := previous[object.ID]
		if !ok {
			continue
		}
		source := normalizedObjectFrame(object)
		passes, cached := passesByFrame[source]
		if !cached {
			passes = g.portalPasses(source)
			passesByFrame[source] = passes
		}
		for _, portal := range passes {
			hit, crosses := environment.CrossingPoint(portal.boundary, from, object.Pose.Position)
			if !crosses {
				continue
			}
			sourcePose, err := g.world.FramePose(source)
			if err != nil {
				return err
			}
			destinationPose, err := g.world.FramePose(portal.destination)
			if err != nil {
				return err
			}
			if err := g.world.Apply(sim.Transfer{ObjectID: object.ID, Destination: portal.destination, Anchor: "portal"}); err != nil {
				return err
			}
			previous[object.ID] = kinematics.Relative(destinationPose, sourcePose).Matrix().TransformPoint(hit)
			break
		}
	}
	g.objects = g.world.Objects
	return nil
}

// startInSurfaceMode places the player directly into the first registered
// exterior-to-surface environment. It is a development-friendly start path
// used by the title screen while the near-surface flight presentation evolves;
// normal gameplay continues to use the orbital arrival sequence.
func (g *Game) startInSurfaceMode() bool {
	if g.world == nil || g.objectByID(fighterID) == nil {
		return false
	}
	for runtimeIndex := range g.environments {
		runtime := &g.environments[runtimeIndex]
		for _, transition := range runtime.bound.Definition.Transitions {
			if transition.Source != scene.ExteriorFrame {
				continue
			}
			destination := runtime.bound.ResolveFrame(transition.Destination)
			g.world.Objects = g.objects
			if err := g.world.Apply(sim.Transfer{ObjectID: fighterID, Destination: destination, Anchor: transition.Name}); err != nil {
				continue
			}
			fighter := g.worldObjectByID(fighterID)
			if fighter == nil {
				continue
			}
			fighter.Pose = transition.EntryPose
			fighter.Motion = g.autoMotion
			fighter.Motion.Speed = max(fighter.Motion.Speed, g.profile.Surface.CruiseSpeed)
			g.objects = g.world.Objects
			g.viewCamera.TargetID = fighterID
			g.viewCamera.Mode = camera.Cockpit
			g.hyperspaceArrival = nil
			g.beginSurfaceEncounter(runtime.bound.FrameID, fighterID)
			g.refreshEnvironmentTiles()
			g.refreshViewContext()
			return true
		}
	}
	return false
}

func (g *Game) updateEnvironmentContacts(seconds float64) {
	for id, remaining := range g.environmentContacts {
		remaining -= seconds
		if remaining <= 0 {
			delete(g.environmentContacts, id)
			continue
		}
		g.environmentContacts[id] = remaining
	}
}

func normalizedObjectFrame(object scene.Object) scene.FrameID {
	if object.Frame == "" {
		return scene.ExteriorFrame
	}
	return object.Frame
}

func sameFrame(first, second scene.Object) bool {
	return normalizedObjectFrame(first) == normalizedObjectFrame(second)
}

func (g *Game) pursuesTarget(id scene.ObjectID) bool {
	controller, ok := g.controllers[id]
	if !ok {
		return false
	}
	follower, ok := controller.(control.PursuitFollower)
	return ok && follower.PursuesTarget()
}

// updateTransitionCommitments gives non-pursuit controllers an explicit,
// deterministic way to commit to a nearby surface approach. The commitment is
// deliberately based on heading as well as distance, so passing fighters do
// not get pulled into a transition unexpectedly.
func (g *Game) updateTransitionCommitments() {
	for _, runtime := range g.environments {
		for _, transition := range runtime.bound.Definition.Transitions {
			source := runtime.bound.ResolveFrame(transition.Source)
			if source != scene.ExteriorFrame {
				continue
			}
			for _, object := range g.objects {
				if !object.Physical || object.ID == fighterID ||
					normalizedObjectFrame(object) != source || g.pursuesTarget(object.ID) {
					continue
				}
				pose, err := g.world.PoseInFrame(object.ID, runtime.bound.FrameID)
				if err != nil || g.transitionCommitments[object.ID] {
					continue
				}
				toVolume := transition.Trigger.Center.Sub(pose.Position)
				distance := toVolume.Length()
				if distance > 160 || distance < 1e-6 {
					continue
				}
				velocity := pose.Forward().Scale(object.Motion.Speed).Add(object.Motion.Velocity)
				if velocity.Length() > 1e-6 && velocity.Normalize().Dot(toVolume.Normalize()) >= 0.35 {
					g.transitionCommitments[object.ID] = true
				}
			}
		}
	}
}

// updateEnvironmentTransitions starts per-object approach presentations. The
// authoritative frame transfer occurs only when the declared presentation
// duration completes (or when the player skips it).
func (g *Game) updateEnvironmentTransitions() error {
	g.updateTransitionCommitments()
	for _, runtime := range g.environments {
		for _, transition := range runtime.bound.Definition.Transitions {
			source := runtime.bound.ResolveFrame(transition.Source)
			destination := runtime.bound.ResolveFrame(transition.Destination)
			for index := range g.objects {
				object := g.objects[index]
				if !object.Physical || normalizedObjectFrame(object) != source {
					continue
				}
				pose, err := g.world.PoseInFrame(object.ID, runtime.bound.FrameID)
				if err != nil {
					return err
				}
				// Ordinary approach transitions are player-driven. Autonomous
				// fighters enter only when following a transitioning target or when
				// their controller has explicitly committed to the approach.
				insideTrigger := transition.Trigger.Contains(pose.Position)
				if transition.ApproachDirection != (math3d.Vec3{}) {
					insideTrigger = insideTrigger && object.Pose.Forward().Dot(transition.ApproachDirection.Normalize()) > 0.2
				}
				inApproach := insideTrigger && (object.ID == fighterID || source != scene.ExteriorFrame)
				// The player's first Death Star surface transition is a mission
				// objective, not merely a proximity trigger. Autonomous pursuers
				// still inherit a transition once the player has legitimately begun
				// it, preserving the existing cross-frame combat behavior.
				if inApproach && object.ID == fighterID && source == scene.ExteriorFrame &&
					transition.Name == "approach" && !g.yavinApproachReady() {
					inApproach = false
				}
				// Pursuers inherit the target's transition. This lets a swarm
				// fighter follow the player into a local surface frame even when
				// it is still outside the ordinary approach volume.
				if !inApproach && !transition.PreservePose && object.ID != fighterID && g.pursuesTarget(object.ID) {
					target := g.objectByID(g.controllerTargets[object.ID])
					if target != nil && normalizedObjectFrame(*target) == source {
						if targetTransition, exists := g.transitions[target.ID]; exists {
							inApproach = targetTransition.destination == destination
						} else {
							targetPose, targetErr := g.world.PoseInFrame(target.ID, runtime.bound.FrameID)
							inApproach = targetErr == nil && transition.Trigger.Contains(targetPose.Position)
						}
					}
				}
				if !inApproach && source == scene.ExteriorFrame && g.transitionCommitments[object.ID] {
					inApproach = true
				}
				if !inApproach {
					continue
				}
				if _, exists := g.transitions[object.ID]; exists {
					continue
				}
				framePose, err := g.world.FramePose(destination)
				if err != nil {
					return err
				}
				if transition.Duration <= 0 {
					if err := g.completeEnvironmentTransition(environmentTransition{
						objectID: object.ID, destination: destination, anchor: transition.Name, motion: object.Motion,
						previousMode: g.viewCamera.Mode, preservePose: transition.PreservePose, entryOffset: transition.EntryOffset,
					}, transition.EntryPose); err != nil {
						return err
					}
					continue
				}
				sourceFramePose, err := g.world.FramePose(source)
				if err != nil {
					return err
				}
				g.transitions[object.ID] = environmentTransition{
					objectID: object.ID, destination: destination, anchor: transition.Name,
					duration: transition.Duration, startPose: object.Pose,
					targetPose: kinematics.Compose(framePose, transition.EntryPose), entryPose: transition.EntryPose, motion: object.Motion,
					worldVelocity: sourceFramePose.Orientation.Rotate(object.Motion.Velocity),
					previousMode:  g.viewCamera.Mode,
					rollRadians:   math.Pi,
					preservePose:  transition.PreservePose,
					entryOffset:   transition.EntryOffset,
				}
				delete(g.transitionCommitments, object.ID)
				g.objects[index].Motion = kinematics.Motion{}
				if object.ID == g.viewCamera.TargetID {
					// Keep the player in cockpit view throughout the approach. The
					// interpolated craft orientation rotates the cockpit horizon and
					// starfield as the fighter aligns with the surface.
					g.viewCamera.Mode = camera.Cockpit
				}
			}
		}
		for index := range g.objects {
			object := g.objects[index]
			if !object.Physical || normalizedObjectFrame(object) != runtime.bound.FrameID ||
				runtime.bound.Definition.ExitVolume.Contains(object.Pose.Position) {
				continue
			}
			if err := g.world.Apply(sim.Transfer{ObjectID: object.ID, Destination: scene.ExteriorFrame, Anchor: "exit"}); err != nil {
				return err
			}
		}
	}
	g.objects = g.world.Objects
	return nil
}

func (g *Game) advanceEnvironmentTransitions(seconds float64, skip bool) {
	for id, transition := range g.transitions {
		transition.elapsed += seconds
		amount := transition.elapsed / transition.duration
		if skip {
			amount = 1
		}
		amount = max(0, min(1, amount))
		// Smoothstep gives the roll and approach a gentle start and finish.
		eased := amount * amount * (3 - 2*amount)
		if object := g.objectByID(id); object != nil {
			object.Pose.Position = transition.startPose.Position.Add(transition.targetPose.Position.Sub(transition.startPose.Position).Scale(eased))
			alignment := math3d.Slerp(transition.startPose.Orientation, transition.targetPose.Orientation, eased)
			// Roll through the requested half-turn for cinematic feedback, then
			// settle back to the declared entry orientation. This keeps the
			// fighter upright when surface flight begins instead of leaving it
			// inverted after the presentation roll.
			rollAmount := transition.rollRadians * math.Sin(math.Pi*eased)
			roll := math3d.QuaternionFromAxisAngle(math3d.Vec3{Z: 1}, rollAmount)
			object.Pose.Orientation = alignment.Mul(roll).Normalize()
		}
		if amount < 1 {
			g.transitions[id] = transition
			continue
		}
		if err := g.completeEnvironmentTransition(transition, transition.entryPose); err != nil {
			delete(g.transitions, id)
			continue
		}
		delete(g.transitions, id)
	}
	g.viewCamera.Update(seconds)
	g.objects = g.world.Objects
}

func (g *Game) completeEnvironmentTransition(transition environmentTransition, finalLocalPose kinematics.Pose) error {
	g.world.Objects = g.objects
	if err := g.world.Apply(sim.Transfer{ObjectID: transition.objectID, Destination: transition.destination, Anchor: transition.anchor}); err != nil {
		return err
	}
	object := g.worldObjectByID(transition.objectID)
	if object == nil {
		return fmt.Errorf("transition object %d disappeared", transition.objectID)
	}
	transferredVelocity := object.Motion.Velocity
	if transition.preservePose {
		object.Pose.Position = object.Pose.Position.Add(transition.entryOffset)
	} else {
		object.Pose = finalLocalPose
	}
	object.Motion = transition.motion
	if transition.preservePose {
		object.Motion.Velocity = transferredVelocity
	}
	if transition.objectID == g.viewCamera.TargetID && g.surfaceRuntime(normalizedObjectFrame(*object)) != nil {
		object.Motion.Speed = max(object.Motion.Speed, g.profile.Surface.CruiseSpeed)
	}
	if !transition.preservePose {
		if framePose, err := g.world.FramePose(transition.destination); err == nil {
			object.Motion.Velocity = framePose.Orientation.Conjugate().Rotate(transition.worldVelocity)
		}
	}
	if transition.objectID == g.viewCamera.TargetID {
		g.viewCamera.Mode = transition.previousMode
	}
	g.objects = g.world.Objects
	if object.Team == g.profile.Player.Team && g.surfaceRuntime(normalizedObjectFrame(*object)) != nil {
		g.beginSurfaceEncounter(normalizedObjectFrame(*object), object.ID)
	}
	return nil
}

func (g *Game) refreshEnvironmentTiles() {
	for runtimeIndex := range g.environments {
		runtime := &g.environments[runtimeIndex]
		desired := make(map[environment.TileCoordinate]bool)
		size := runtime.bound.Definition.TileSize
		radius := runtime.bound.Definition.TileRadius
		for _, object := range g.objects {
			if normalizedObjectFrame(object) != runtime.bound.FrameID ||
				(object.ID != g.viewCamera.TargetID && !object.Physical && object.CollisionRole != scene.CollisionProjectile) {
				continue
			}
			streamRadius := radius
			// Swept projectile collision needs only its containing tile and immediate
			// neighbors; nearby solid actors already retain the broader region.
			// This prevents a burst of long-lived surface fire from multiplying a
			// full square of authoritative tiles for every projectile.
			if object.CollisionRole == scene.CollisionProjectile {
				streamRadius = min(1, radius)
			}
			center := environment.TileCoordinate{
				X: int(math.Floor(object.Pose.Position.X/size + 0.5)),
				Z: int(math.Floor(object.Pose.Position.Z/size + 0.5)),
			}
			for offsetX := -streamRadius; offsetX <= streamRadius; offsetX++ {
				for offsetZ := -streamRadius; offsetZ <= streamRadius; offsetZ++ {
					desired[environment.TileCoordinate{X: center.X + offsetX, Z: center.Z + offsetZ}] = true
				}
			}
		}
		if viewed := g.objectByID(g.viewCamera.TargetID); viewed != nil {
			if room, ok := g.environmentRegistry.Room(normalizedObjectFrame(*viewed)); ok {
				roomPose, roomErr := g.world.FramePose(room.Frame)
				destinationPose, destinationErr := g.world.FramePose(runtime.bound.FrameID)
				if roomErr == nil && destinationErr == nil {
					for _, portal := range room.Portals {
						if portal.Destination != runtime.bound.FrameID || len(portal.Boundary) == 0 {
							continue
						}
						center := math3d.Vec3{}
						for _, point := range portal.Boundary {
							center = center.Add(point)
						}
						center = center.Scale(1 / float64(len(portal.Boundary)))
						worldPoint := roomPose.Matrix().TransformPoint(center)
						localPoint := kinematics.Relative(destinationPose, kinematics.Pose{Position: worldPoint, Orientation: math3d.IdentityQuaternion()}).Position
						portalTile := environment.TileCoordinate{
							X: int(math.Floor(localPoint.X/size + 0.5)), Z: int(math.Floor(localPoint.Z/size + 0.5)),
						}
						for dx := -1; dx <= 1; dx++ {
							for dz := -1; dz <= 1; dz++ {
								desired[environment.TileCoordinate{X: portalTile.X + dx, Z: portalTile.Z + dz}] = true
							}
						}
					}
				}
			}
		}
		tiles := make(map[environment.TileCoordinate]environment.Tile, len(desired))
		for coordinate := range desired {
			if tile, exists := runtime.tiles[coordinate]; exists {
				tiles[coordinate] = tile
			} else {
				tile = runtime.bound.Definition.Tile(coordinate)
				if !tile.Bounds.Valid() {
					tile = environment.PrepareTile(tile)
				}
				tiles[coordinate] = tile
			}
		}
		for coordinate, oldTile := range runtime.tiles {
			if desired[coordinate] {
				continue
			}
			delete(runtime.tileDetailLevels, coordinate)
			for _, feature := range oldTile.Features {
				delete(runtime.detailLevels, feature.ID)
			}
		}
		runtime.tiles = tiles

		// The distant visual shell follows only the viewed craft. Physical actors
		// retain the smaller authoritative stream above, so extending the horizon
		// does not multiply collision, targeting, or installation geometry.
		horizonDesired := make(map[environment.TileCoordinate]bool)
		horizonRadius := runtime.bound.Definition.HorizonTileRadius
		if horizonRadius > radius && runtime.bound.Definition.HorizonTile != nil {
			if target := g.objectByID(g.viewCamera.TargetID); target != nil && normalizedObjectFrame(*target) == runtime.bound.FrameID {
				center := environment.TileCoordinate{
					X: int(math.Floor(target.Pose.Position.X/size + 0.5)),
					Z: int(math.Floor(target.Pose.Position.Z/size + 0.5)),
				}
				for offsetX := -horizonRadius; offsetX <= horizonRadius; offsetX++ {
					for offsetZ := -horizonRadius; offsetZ <= horizonRadius; offsetZ++ {
						coordinate := environment.TileCoordinate{X: center.X + offsetX, Z: center.Z + offsetZ}
						if !desired[coordinate] {
							horizonDesired[coordinate] = true
						}
					}
				}
			}
		}
		horizonTiles := make(map[environment.TileCoordinate]environment.Tile, len(horizonDesired))
		for coordinate := range horizonDesired {
			if tile, exists := runtime.horizonTiles[coordinate]; exists {
				horizonTiles[coordinate] = tile
			} else {
				tile := runtime.bound.Definition.HorizonTile(coordinate)
				if !tile.Bounds.Valid() {
					tile = environment.PrepareTile(tile)
				}
				horizonTiles[coordinate] = tile
			}
		}
		for coordinate := range runtime.horizonTiles {
			if !horizonDesired[coordinate] {
				delete(runtime.tileDetailLevels, coordinate)
			}
		}
		runtime.horizonTiles = horizonTiles
	}
}

// viewTransitionEnvironment resolves the one environment presentation visible
// to this camera. Other fighters may transition concurrently, but their
// destination patches must not add invisible preparation or tile-generation
// work to the local view.
func (g *Game) viewTransitionEnvironment() (environmentTransition, *localEnvironment, bool) {
	if g.world == nil || g.viewCamera == nil {
		return environmentTransition{}, nil, false
	}
	transition, exists := g.transitions[g.viewCamera.TargetID]
	if !exists {
		return environmentTransition{}, nil, false
	}
	for index := range g.environments {
		if g.environments[index].bound.FrameID == transition.destination {
			return transition, &g.environments[index], true
		}
	}
	return environmentTransition{}, nil, false
}

// refreshTransitionEnvironmentTiles acquires presentation geometry during the
// update phase. Drawing and prepared-frame construction never invoke a tile
// factory, which keeps rendering free of simulation/cache mutation and gives
// future cut scenes the same explicit preparation boundary.
func (g *Game) refreshTransitionEnvironmentTiles() {
	_, runtime, ok := g.viewTransitionEnvironment()
	if !ok {
		return
	}
	if runtime.tiles == nil {
		runtime.tiles = make(map[environment.TileCoordinate]environment.Tile)
	}
	for tileX := -transitionEnvironmentTileRadius; tileX <= transitionEnvironmentTileRadius; tileX++ {
		for tileZ := -transitionEnvironmentTileRadius; tileZ <= transitionEnvironmentTileRadius; tileZ++ {
			coordinate := environment.TileCoordinate{X: tileX, Z: tileZ}
			if _, exists := runtime.tiles[coordinate]; !exists {
				tile := runtime.bound.Definition.Tile(coordinate)
				if !tile.Bounds.Valid() {
					tile = environment.PrepareTile(tile)
				}
				runtime.tiles[coordinate] = tile
			}
		}
	}
}

func (g *Game) worldObjectByID(id scene.ObjectID) *scene.Object {
	if g.world == nil {
		return nil
	}
	for index := range g.world.Objects {
		if g.world.Objects[index].ID == id {
			return &g.world.Objects[index]
		}
	}
	return nil
}

// Snapshot returns a renderer-independent copy of the current world state.
func (g *Game) Snapshot() sim.Snapshot {
	if g.world == nil {
		return sim.Snapshot{}
	}
	g.world.Objects = g.objects
	return g.world.Snapshot()
}

// ApplySimulationCommands applies validated structural or motion commands at
// the simulation boundary. The adapter refreshes its object slice afterward.
func (g *Game) ApplySimulationCommands(commands ...sim.Command) error {
	if g.world == nil {
		return fmt.Errorf("simulation world is unavailable")
	}
	g.world.Objects = g.objects
	if err := g.world.Apply(commands...); err != nil {
		return err
	}
	g.objects = g.world.Objects
	return nil
}

func (g *Game) starfieldReferencePosition() *math3d.Vec3 {
	targetID := fighterID
	if g.viewCamera.Mode == camera.Chase || g.viewCamera.Mode == camera.Orbit {
		targetID = g.viewCamera.TargetID
	}
	object := g.objectByID(targetID)
	if object == nil {
		return nil
	}
	position := object.Pose.Position
	return &position
}

func (g *Game) switchToRandomSwarmFollowView() {
	ids := make([]scene.ObjectID, 0, len(g.controllers))
	for id := range g.controllers {
		if g.objectByID(id) != nil {
			ids = append(ids, id)
		}
	}
	if len(ids) == 0 {
		g.viewCamera.Mode = camera.Fixed
		return
	}
	sort.Slice(ids, func(first, second int) bool { return ids[first] < ids[second] })
	seed := uint64(g.respawnSequence) ^ uint64(math.Max(0, math.Floor(g.simulationTime*60)))
	index := int(deterministicSigned(&seed)*0.5*float64(len(ids)) + 0.5*float64(len(ids)))
	index = max(0, min(len(ids)-1, index))
	g.viewCamera.ClearFixedView()
	g.viewCamera.TargetID = ids[index]
	g.viewCamera.Mode = camera.Chase
}

// The death camera follows the fighter responsible for the kill when it is
// still alive. Otherwise it picks the closest hostile fighter in the same
// environment so the wreckage remains in the ensuing chase view.
func (g *Game) switchToEnemyDestructionFollowView(victim scene.Object, preferred scene.ObjectID) bool {
	eligible := func(object scene.Object) bool {
		if object.ID == 0 || object.Team == scene.TeamNeutral || object.Team == victim.Team ||
			!sameFrame(object, victim) {
			return false
		}
		if _, autonomous := g.controllers[object.ID]; !autonomous {
			return false
		}
		_, hasChase := object.Anchor("chase")
		return hasChase
	}
	selected := scene.Object{}
	if preferred != 0 {
		if attacker := g.objectByID(preferred); attacker != nil && eligible(*attacker) {
			selected = *attacker
		}
	}
	if selected.ID == 0 {
		bestDistance := math.Inf(1)
		for _, candidate := range g.objects {
			if !eligible(candidate) {
				continue
			}
			offset := candidate.Pose.Position.Sub(victim.Pose.Position)
			distance := offset.Dot(offset)
			if distance < bestDistance || (distance == bestDistance && candidate.ID < selected.ID) {
				selected, bestDistance = candidate, distance
			}
		}
	}
	if selected.ID == 0 {
		g.fixPlayerDestructionView(victim)
		return false
	}
	g.viewCamera.ClearFixedView()
	g.viewCamera.TargetID = selected.ID
	g.viewCamera.Mode = camera.Chase
	return true
}

func objectPositions(objects []scene.Object) map[scene.ObjectID]math3d.Vec3 {
	positions := make(map[scene.ObjectID]math3d.Vec3, len(objects))
	for _, object := range objects {
		positions[object.ID] = object.Pose.Position
	}
	return positions
}

func (g *Game) resolveLaserCollisions(previous map[scene.ObjectID]math3d.Vec3) {
	remove := make(map[scene.ObjectID]bool)
	destroyed := make(map[scene.ObjectID]scene.Object)
	var playerAttacker scene.ObjectID
	playerShieldHit := false
	for _, projectile := range g.objects {
		if projectile.CollisionRole != scene.CollisionProjectile || remove[projectile.ID] {
			continue
		}
		start, ok := previous[projectile.ID]
		if !ok {
			start = projectile.Pose.Position
		}
		// Resolve opposing projectile interception before object targeting. A
		// swept test is important here because laser bolts move many world units
		// per tick and may cross between rendered frames.
		owner := g.owners[projectile.ID]
		for _, other := range g.objects {
			if other.ID <= projectile.ID || other.CollisionRole != scene.CollisionProjectile ||
				remove[other.ID] || g.owners[other.ID] == owner || !sameFrame(projectile, other) {
				continue
			}
			if projectile.Team != scene.TeamNeutral && projectile.Team == other.Team {
				continue
			}
			otherStart, ok := previous[other.ID]
			if !ok {
				otherStart = other.Pose.Position
			}
			relativeStart := start.Sub(otherStart)
			relativeEnd := projectile.Pose.Position.Sub(other.Pose.Position)
			if _, hit := collision.SegmentSphere(
				relativeStart,
				relativeEnd,
				math3d.Vec3{},
				projectile.CollisionRadius+other.CollisionRadius,
			); hit {
				remove[projectile.ID] = true
				remove[other.ID] = true
				break
			}
		}
		if remove[projectile.ID] {
			continue
		}
		nearestTime := math.Inf(1)
		var nearest scene.Object
		for _, target := range g.objects {
			if target.ID == owner || target.ID == projectile.ID || destroyed[target.ID].ID != 0 || !target.Hittable {
				continue
			}
			if projectile.Team != scene.TeamNeutral && projectile.Team == target.Team {
				continue
			}
			if !sameFrame(projectile, target) {
				continue
			}
			targetStart, ok := previous[target.ID]
			if !ok {
				targetStart = target.Pose.Position
			}
			relativeStart := start.Sub(targetStart)
			relativeEnd := projectile.Pose.Position.Sub(target.Pose.Position)
			hitTime, hit := collision.SegmentSphere(
				relativeStart,
				relativeEnd,
				math3d.Vec3{},
				projectile.CollisionRadius+target.CollisionRadius,
			)
			if hit && hitTime < nearestTime {
				nearestTime = hitTime
				nearest = target
			}
		}
		if nearest.ID != 0 {
			remove[projectile.ID] = true
			if nearest.ID == fighterID {
				if !playerShieldHit {
					playerShieldHit = true
					// Damage is applied once per simulation tick for a paired volley.
					if g.applyShieldDamage(g.profile.Player.Shield.LaserDamage) {
						destroyed[nearest.ID] = nearest
						playerAttacker = owner
					}
				}
			} else if nearest.Destructible {
				destroyed[nearest.ID] = nearest
			}
			if owner == fighterID {
				if _, autonomous := g.controllers[nearest.ID]; autonomous {
					g.kills++
				}
			}
		}
	}
	if len(remove) == 0 && len(destroyed) == 0 {
		return
	}
	g.destroyAndDisintegrateWithAttacker(destroyed, remove, playerAttacker)
}

func (g *Game) resolveSolidCollisions(previous map[scene.ObjectID]math3d.Vec3) {
	destroyed := make(map[scene.ObjectID]scene.Object)
	var playerAttacker scene.ObjectID
	for firstIndex, first := range g.objects {
		if !first.Physical || destroyed[first.ID].ID != 0 {
			continue
		}
		for _, second := range g.objects[firstIndex+1:] {
			if !second.Physical || destroyed[second.ID].ID != 0 || !sameFrame(first, second) {
				continue
			}
			firstStart := previous[first.ID]
			secondStart := previous[second.ID]
			relativeStart := firstStart.Sub(secondStart)
			relativeEnd := first.Pose.Position.Sub(second.Pose.Position)
			_, hit := collision.SegmentSphere(
				relativeStart,
				relativeEnd,
				math3d.Vec3{},
				first.CollisionRadius+second.CollisionRadius,
			)
			if hit {
				g.collisions++
				if first.ID == fighterID {
					if g.applyShieldDamage(g.profile.Player.Shield.CollisionDamage) {
						destroyed[first.ID] = first
						playerAttacker = second.ID
					}
				} else if first.Destructible {
					destroyed[first.ID] = first
				}
				if second.ID == fighterID {
					if g.applyShieldDamage(g.profile.Player.Shield.CollisionDamage) {
						destroyed[second.ID] = second
						playerAttacker = first.ID
					}
				} else if second.Destructible {
					destroyed[second.ID] = second
				}
				break
			}
		}
	}
	if len(destroyed) > 0 {
		g.destroyAndDisintegrateWithAttacker(destroyed, nil, playerAttacker)
	}
}

func (g *Game) resolveEnvironmentCollisions(previous map[scene.ObjectID]math3d.Vec3) {
	remove := make(map[scene.ObjectID]bool)
	destroyed := make(map[scene.ObjectID]scene.Object)
	for _, object := range g.objects {
		if object.CollisionRole != scene.CollisionProjectile && !object.Physical {
			continue
		}
		if g.environmentContacts[object.ID] > 0 || g.transitionedOnCurrentTick(object.ID) {
			continue
		}
		var runtime *localEnvironment
		for index := range g.environments {
			if g.environments[index].bound.FrameID == normalizedObjectFrame(object) {
				runtime = &g.environments[index]
				break
			}
		}
		if runtime == nil {
			continue
		}
		start, ok := previous[object.ID]
		if !ok {
			start = object.Pose.Position
		}
		var nearest collision.Hit
		hitFound := false
		for _, tile := range runtime.tiles {
			for _, plane := range tile.Planes {
				if hit, ok := collision.SweepSpherePlane(start, object.Pose.Position, object.CollisionRadius, plane); ok && (!hitFound || hit.Time < nearest.Time) {
					nearest, hitFound = hit, true
				}
			}
			for _, box := range tile.Boxes {
				if runtime.featureStates[string(box.FeatureID)].Destroyed {
					continue
				}
				if hit, ok := collision.SweepSphereBox(start, object.Pose.Position, object.CollisionRadius, box); ok && (!hitFound || hit.Time < nearest.Time) {
					nearest, hitFound = hit, true
				}
			}
		}
		if !hitFound {
			continue
		}
		if object.CollisionRole == scene.CollisionProjectile {
			remove[object.ID] = true
			featureID := string(nearest.FeatureID)
			if feature, ok := environmentFeatureByID(runtime, featureID); ok {
				if feature.Kind == "exhaust-port" {
					g.resolveExhaustPortAttack(object, nearest)
				} else if feature.Hittable && (object.Team == scene.TeamNeutral || object.Team != feature.Team) {
					g.hitEnvironmentFeature(runtime, feature, object, nearest)
				} else {
					g.spawnSurfaceImpact(object.Frame, nearest.Point, nearest.Normal)
				}
			} else {
				g.spawnSurfaceImpact(object.Frame, nearest.Point, nearest.Normal)
			}
			continue
		}
		g.collisions++
		if object.ID == fighterID {
			if g.applyShieldDamage(g.profile.Player.Shield.CollisionDamage) {
				destroyed[object.ID] = object
				continue
			}
		} else if object.Destructible {
			destroyed[object.ID] = object
			continue
		}
		if live := g.objectByID(object.ID); live != nil {
			live.Pose.Position = nearest.Point.Add(nearest.Normal.Scale(live.CollisionRadius + 0.02))
			velocity := live.Pose.Forward().Scale(live.Motion.Speed).Add(live.Motion.Velocity)
			reflected := velocity.Sub(nearest.Normal.Scale(1.4 * velocity.Dot(nearest.Normal)))
			if reflected.Length() > 1e-9 {
				direction := reflected.Normalize()
				live.Pose.Orientation = math3d.QuaternionFromYawPitchRoll(
					math.Atan2(direction.X, direction.Z),
					-math.Asin(max(-1, min(1, direction.Y))),
					0,
				)
				live.Motion.Speed = reflected.Length() * 0.7
				live.Motion.Velocity = math3d.Vec3{}
			}
			g.environmentContacts[live.ID] = 0.35
		}
	}
	if len(remove) > 0 || len(destroyed) > 0 {
		g.destroyAndDisintegrate(destroyed, remove)
	}
}

const (
	exhaustAttackWrongWeapon = "attack-proton-torpedo-required"
	exhaustAttackWrongPilot  = "attack-alliance-pilot-required"
	exhaustAttackTooClose    = "attack-torpedo-not-armed"
	exhaustAttackTooFar      = "attack-torpedo-out-of-range"
	exhaustAttackOffCenter   = "attack-aim-off-center"
	exhaustAttackWrongCourse = "attack-wrong-approach"
)

// validateExhaustPortAttack contains no rendering or input assumptions. It
// validates the authoritative payload, owner, travel envelope and impact
// vector so the same result can be reproduced for a remote player.
func validateExhaustPortAttack(projectile scene.Object, owner scene.ObjectID, mission sim.MissionState, hit collision.Hit, config combat.TorpedoConfig) string {
	if projectile.ProjectileKind != scene.ProjectileProtonTorpedo {
		return exhaustAttackWrongWeapon
	}
	if owner == 0 || owner != mission.PlayerID || projectile.Team != scene.TeamAlliance {
		return exhaustAttackWrongPilot
	}
	if projectile.ProjectileTravel < config.MinimumTravel {
		return exhaustAttackTooClose
	}
	if projectile.ProjectileTravel > config.MaximumTravel {
		return exhaustAttackTooFar
	}
	port := environment.DeathStarExhaustPortPoint()
	dx, dz := hit.Point.X-port.X, hit.Point.Z-port.Z
	if dx*dx+dz*dz > config.MaximumAlignment*config.MaximumAlignment {
		return exhaustAttackOffCenter
	}
	forward := projectile.Pose.Forward()
	if forward.Z < config.MinimumForwardDot || -forward.Y < config.MinimumDownDot {
		return exhaustAttackWrongCourse
	}
	return ""
}

func (g *Game) resolveExhaustPortAttack(projectile scene.Object, hit collision.Hit) {
	g.spawnSurfaceImpact(projectile.Frame, hit.Point, hit.Normal)
	if g.world == nil || g.world.Mission.ID != yavinMissionID || g.world.Mission.Phase != sim.MissionExhaustPortAttack {
		return
	}
	reason := validateExhaustPortAttack(projectile, g.owners[projectile.ID], g.world.Mission, hit, g.profile.Combat.Torpedo)
	if reason != "" {
		_ = g.world.Apply(sim.ReportMissionFeedback{Reason: reason})
		return
	}
	_ = g.world.Apply(sim.AdvanceMission{To: sim.MissionEscape, Reason: "exhaust-port-hit"})
}

func (g *Game) spawnSurfaceImpact(frame scene.FrameID, point, normal math3d.Vec3) {
	if len(g.surfaceEffects) >= 24 {
		return
	}
	id := g.nextObjectID
	g.nextObjectID++
	if normal == (math3d.Vec3{}) {
		normal = math3d.Vec3{Y: 1}
	}
	impact := scene.Object{
		ID: id, Name: "surface impact", Definition: "builtin/surface-impact", Frame: frame,
		Pose:         kinematics.Pose{Position: point.Add(normal.Normalize().Scale(0.08)), Orientation: math3d.IdentityQuaternion()},
		Parts:        []scene.Part{{Name: "spark", Mesh: surfaceImpactMesh, Color: color.RGBA{R: 255, G: 208, B: 64, A: 255}, LineWidth: 1.5}},
		Anchors:      map[string]kinematics.Pose{"center": {Orientation: math3d.IdentityQuaternion()}},
		VisualRadius: 1.2,
	}
	g.objects = append(g.objects, impact)
	g.surfaceEffects[id] = surfaceEffect{remaining: 0.18}
}

func (g *Game) updateSurfaceEffects(seconds float64) {
	remove := make(map[scene.ObjectID]bool)
	for id, effect := range g.surfaceEffects {
		effect.remaining -= seconds
		if effect.remaining <= 0 {
			remove[id] = true
			continue
		}
		g.surfaceEffects[id] = effect
	}
	g.removeObjects(remove)
}

func environmentFeatureByID(runtime *localEnvironment, id string) (environment.Feature, bool) {
	if runtime == nil || id == "" {
		return environment.Feature{}, false
	}
	for _, tile := range runtime.tiles {
		for _, feature := range tile.Features {
			if feature.ID == id {
				return feature, true
			}
		}
	}
	return environment.Feature{}, false
}

func (g *Game) hitEnvironmentFeature(runtime *localEnvironment, feature environment.Feature, projectile scene.Object, hit collision.Hit) {
	if runtime.featureStates == nil {
		runtime.featureStates = make(map[string]featureDamageState)
	}
	state := runtime.featureStates[feature.ID]
	if state.Destroyed || !feature.Hittable {
		return
	}
	state.Hits++
	if feature.DisableAfter > 0 && state.Hits >= feature.DisableAfter {
		state.Disabled = true
	}
	hitPoints := feature.HitPoints
	if hitPoints <= 0 {
		hitPoints = 1
	}
	if state.Hits >= hitPoints {
		state.Destroyed = true
		state.Disabled = true
	}
	runtime.featureStates[feature.ID] = state
	event := sim.FeatureDamageEvent{
		HostID: runtime.bound.HostID, Frame: projectile.Frame, FeatureID: feature.ID,
		Point: hit.Point, Normal: hit.Normal, Hits: state.Hits,
		Disabled: state.Disabled, Destroyed: state.Destroyed,
	}
	if g.world != nil {
		event.Tick = g.world.Tick
		g.world.FeatureEvents = append(g.world.FeatureEvents, event)
	}
	g.presentFeatureDamage(event, projectile)
}

func (g *Game) presentFeatureDamage(event sim.FeatureDamageEvent, projectile scene.Object) {
	g.spawnSurfaceImpact(event.Frame, event.Point, event.Normal)
	if event.Destroyed {
		g.spawnInstallationFracture(event, projectile)
	}
}

func (g *Game) spawnInstallationFracture(event sim.FeatureDamageEvent, projectile scene.Object) {
	seed := uint64(1469598103934665603)
	for index := range len(event.FeatureID) {
		seed = (seed ^ uint64(event.FeatureID[index])) * 1099511628211
	}
	seed ^= event.Tick * 0x9e3779b97f4a7c15
	if seed == 0 {
		seed = 1
	}
	normal := event.Normal
	if normal.Length() < 1e-9 {
		normal = math3d.Vec3{Y: 1}
	}
	normal = normal.Normalize()
	forward := projectile.Pose.Forward().Scale(max(3, projectile.Motion.Speed*0.06))
	for index := 0; index < 3 && len(g.surfaceEffects) < 24; index++ {
		id := g.nextObjectID
		g.nextObjectID++
		noise := math3d.Vec3{
			X: deterministicSigned(&seed) * 3.5,
			Y: deterministicSigned(&seed) * 2.5,
			Z: deterministicSigned(&seed) * 3.5,
		}
		fragment := scene.Object{
			ID: id, Name: "installation fragment", Definition: "builtin/installation-fragment", Frame: event.Frame,
			Pose:   kinematics.Pose{Position: event.Point.Add(normal.Scale(0.2)), Orientation: math3d.IdentityQuaternion()},
			Motion: kinematics.Motion{Velocity: forward.Add(normal.Scale(4)).Add(noise)},
			Parts: []scene.Part{{Name: "fragment", Mesh: installationShardMesh,
				Color: color.RGBA{R: 255, G: 160, B: 56, A: 255}, LineWidth: 1.25,
				Surface: scene.SurfaceMaterial{Mode: scene.SurfaceFlatOpaque, Color: color.RGBA{R: 14, G: 10, B: 7, A: 255}}}},
			VisualRadius: 0.25,
		}
		g.objects = append(g.objects, fragment)
		g.surfaceEffects[id] = surfaceEffect{remaining: 0.6}
	}
}

func (g *Game) transitionedOnCurrentTick(id scene.ObjectID) bool {
	if g.world == nil {
		return false
	}
	for index := len(g.world.Transitions) - 1; index >= 0; index-- {
		event := g.world.Transitions[index]
		if event.Tick < g.world.Tick {
			return false
		}
		if event.ObjectID == id {
			return true
		}
	}
	return false
}

func (g *Game) destroyAndDisintegrate(destroyed map[scene.ObjectID]scene.Object, remove map[scene.ObjectID]bool) {
	g.destroyAndDisintegrateWithAttacker(destroyed, remove, 0)
}

func (g *Game) destroyAndDisintegrateWithAttacker(destroyed map[scene.ObjectID]scene.Object, remove map[scene.ObjectID]bool, playerAttacker scene.ObjectID) {
	if remove == nil {
		remove = make(map[scene.ObjectID]bool)
	}
	ids := make([]scene.ObjectID, 0, len(destroyed))
	transients := make(map[scene.ObjectID]destructionTransient, len(destroyed))
	for id := range destroyed {
		ids = append(ids, id)
		remove[id] = true
		transients[id] = g.debris[id]
	}
	sort.Slice(ids, func(first, second int) bool { return ids[first] < ids[second] })
	for _, id := range ids {
		if destroyed[id].DestructionStage != scene.DestructionIntact {
			continue
		}
		if id == fighterID {
			g.playerDestroyed = true
			if g.world != nil && g.world.Mission.ID == yavinMissionID {
				_ = g.world.Apply(sim.FailMission{Reason: "fighter-destroyed"})
			}
			g.destructionVictim = destroyed[id]
			g.controlsRemaining = g.profile.Display.ControlsDisplayDuration
			g.controlsPinned = false
			g.playerViewMode = g.viewCamera.Mode
			g.fixPlayerDestructionView(destroyed[id])
			if g.mouseFlight {
				g.mouseFlight = false
				ebiten.SetCursorMode(ebiten.CursorModeVisible)
			}
		}
		if _, autonomous := g.controllers[id]; autonomous {
			g.respawns = append(g.respawns, autonomousRespawn{
				readyAt:    g.simulationTime + g.profile.Swarm.RespawnDelay,
				definition: destroyed[id].Definition,
			})
			delete(g.controllers, id)
		}
	}
	g.removeObjects(remove)
	for _, id := range ids {
		object := destroyed[id]
		switch object.DestructionStage {
		case scene.DestructionIntact:
			cinematicTarget := g.nextObjectID
			g.spawnDisintegration(object)
			if id == fighterID {
				g.destructionViewRemaining = g.profile.Simulation.PlayerDestructionViewTime
				g.viewCamera.TargetID = cinematicTarget
			}
		case scene.DestructionComponent:
			g.spawnPolygonDisintegration(object, transients[id])
		}
	}
	if victim, ok := destroyed[fighterID]; ok && victim.DestructionStage == scene.DestructionIntact {
		g.switchToEnemyDestructionFollowView(victim, playerAttacker)
	}
}

func (g *Game) fixPlayerDestructionView(object scene.Object) {
	pose, ok := object.Anchor("chase")
	if !ok {
		pose = object.Pose
		pose.Position = pose.Position.
			Sub(object.Pose.Forward().Scale(max(3.5, object.VisualRadius*1.4))).
			Add(object.Pose.Orientation.Rotate(math3d.Vec3{Y: max(1, object.VisualRadius*0.3)}))
		pose.Orientation = pose.Orientation.Mul(math3d.QuaternionFromYawPitchRoll(math.Pi, 0, 0))
	}
	g.viewCamera.FixAt(pose)
}

func (g *Game) spawnDisintegration(object scene.Object) {
	inheritedVelocity := object.Pose.Forward().Scale(object.Motion.Speed).Add(object.Motion.Velocity)
	for index := range 3 {
		seed := uint64(object.ID)*0x9e3779b97f4a7c15 + uint64(index+1)*0x517cc1b727220a95
		fragment, err := g.catalogRegistry.CreateFragment(object.Definition, g.nextObjectID, index, object.Pose)
		if err != nil {
			continue
		}
		g.nextObjectID++
		sourceOrigin, _ := recenterObjectGeometry(&fragment, math3d.Vec3{})
		// Debris continues along the destroyed craft's actual world trajectory.
		// A small deterministic local perturbation separates the components
		// without turning the breakup into a symmetric radial explosion.
		noiseScale := max(0.16, inheritedVelocity.Length()*0.07)
		localNoise := math3d.Vec3{
			X: deterministicSigned(&seed),
			Y: deterministicSigned(&seed),
			Z: deterministicSigned(&seed) * 0.55,
		}.Scale(noiseScale)
		noise := object.Pose.Orientation.Rotate(localNoise)
		spinSign := 1.0
		if (uint64(object.ID)+uint64(index))%2 == 0 {
			spinSign = -1
		}
		fragment.Motion = kinematics.Motion{
			Velocity:  inheritedVelocity.Add(noise),
			YawRate:   spinSign * (1.25 + 0.30*float64(index)),
			PitchRate: -spinSign * (1.05 + 0.22*float64(index)),
			RollRate:  spinSign * (2.0 + 0.40*float64(index)),
		}
		fragment.Frame = object.Frame
		fragment.Team = object.Team
		g.objects = append(g.objects, fragment)
		lifetime := g.profile.Simulation.DisintegrationTime
		if object.ID == fighterID {
			lifetime = g.profile.Simulation.PlayerDestructionViewTime
		}
		g.debris[fragment.ID] = destructionTransient{
			remaining:      lifetime,
			rootObjectID:   object.ID,
			componentIndex: index,
			stage:          scene.DestructionComponent,
			sourceOrigin:   sourceOrigin,
		}
	}
}

func (g *Game) spawnPolygonDisintegration(component scene.Object, transient destructionTransient) {
	if transient.rootObjectID == 0 {
		transient.rootObjectID = component.ID
	}
	polygonCount, err := g.catalogRegistry.PolygonCount(component.Definition, transient.componentIndex)
	if err != nil {
		return
	}
	for polygonIndex := 0; polygonIndex < polygonCount; polygonIndex++ {
		shard, err := g.catalogRegistry.CreatePolygon(component.Definition,
			g.nextObjectID,
			transient.componentIndex,
			polygonIndex,
			component.Pose,
		)
		if err != nil {
			continue
		}
		g.nextObjectID++
		polygonOrigin, _ := recenterObjectGeometry(&shard, transient.sourceOrigin)
		localDirection := polygonOrigin.Sub(transient.sourceOrigin).Normalize()
		if localDirection == (math3d.Vec3{}) {
			angle := 2 * math.Pi * float64(polygonIndex+1) / float64(max(1, polygonCount))
			localDirection = math3d.Vec3{X: math.Cos(angle), Y: math.Sin(angle), Z: 0.35}.Normalize()
		}
		direction := component.Pose.Orientation.Rotate(localDirection).Normalize()
		spinSign := 1.0
		if (uint64(component.ID)+uint64(polygonIndex))%2 == 0 {
			spinSign = -1
		}
		shard.Motion = kinematics.Motion{
			Velocity:  component.Motion.Velocity.Add(direction.Scale(0.65 + 0.05*float64(polygonIndex%5))),
			YawRate:   spinSign * (1.4 + 0.11*float64(polygonIndex%7)),
			PitchRate: -spinSign * (1.1 + 0.09*float64(polygonIndex%5)),
			RollRate:  spinSign * (2.2 + 0.13*float64(polygonIndex%9)),
		}
		shard.Frame = component.Frame
		shard.Team = component.Team
		g.objects = append(g.objects, shard)
		g.debris[shard.ID] = destructionTransient{
			remaining:      g.profile.Simulation.DisintegrationTime,
			rootObjectID:   transient.rootObjectID,
			componentIndex: transient.componentIndex,
			stage:          scene.DestructionPolygon,
			sourceOrigin:   polygonOrigin,
		}
	}
}

// recenterObjectGeometry moves authored model vertices onto an object's own
// local origin and offsets its world pose by the inverse amount. The rendered
// geometry therefore remains stationary at the instant of breakup, but later
// angular integration rotates it around its own approximate centre of mass.
// sourceOrigin describes the current pose origin in the catalog model frame.
func recenterObjectGeometry(object *scene.Object, sourceOrigin math3d.Vec3) (math3d.Vec3, bool) {
	if object == nil {
		return math3d.Vec3{}, false
	}
	center, found := objectGeometryBoundsCenter(*object)
	if !found {
		return math3d.Vec3{}, false
	}
	recenter := math3d.Translation(-center.X, -center.Y, -center.Z)
	for index := range object.Parts {
		object.Parts[index].Mesh = modelpkg.Transform(object.Parts[index].Mesh, recenter)
	}
	localPoseOffset := center.Sub(sourceOrigin)
	object.Pose.Position = object.Pose.Position.Add(object.Pose.Orientation.Rotate(localPoseOffset))
	return center, true
}

func objectGeometryBoundsCenter(object scene.Object) (math3d.Vec3, bool) {
	var minimum, maximum math3d.Vec3
	found := false
	for _, part := range object.Parts {
		for _, vertex := range part.Mesh.Verts {
			if !found {
				minimum, maximum, found = vertex, vertex, true
				continue
			}
			minimum.X, minimum.Y, minimum.Z = min(minimum.X, vertex.X), min(minimum.Y, vertex.Y), min(minimum.Z, vertex.Z)
			maximum.X, maximum.Y, maximum.Z = max(maximum.X, vertex.X), max(maximum.Y, vertex.Y), max(maximum.Z, vertex.Z)
		}
	}
	if !found {
		return math3d.Vec3{}, false
	}
	return minimum.Add(maximum).Scale(0.5), true
}

func (g *Game) updateDebris(seconds float64) {
	remove := make(map[scene.ObjectID]bool)
	for id, transient := range g.debris {
		if transient.remaining <= seconds+1e-9 {
			remove[id] = true
			continue
		}
		transient.remaining -= seconds
		g.debris[id] = transient
	}
	g.removeObjects(remove)
}

func (g *Game) removeObjects(remove map[scene.ObjectID]bool) {
	if len(remove) == 0 {
		return
	}
	kept := g.objects[:0]
	for _, object := range g.objects {
		if remove[object.ID] {
			delete(g.projectiles, object.ID)
			delete(g.owners, object.ID)
			delete(g.debris, object.ID)
			delete(g.surfaceEffects, object.ID)
			delete(g.environmentContacts, object.ID)
			delete(g.controllerTargets, object.ID)
			continue
		}
		kept = append(kept, object)
	}
	g.objects = kept
	for controller, target := range g.controllerTargets {
		if remove[target] || remove[controller] {
			delete(g.controllerTargets, controller)
		}
	}
}

func autonomousFighterPoses(positions []math3d.Vec3) []kinematics.Pose {
	poses := make([]kinematics.Pose, 0, len(positions))
	for index, position := range positions {
		poses = append(poses, kinematics.Pose{
			Position: position,
			Orientation: math3d.QuaternionFromYawPitchRoll(
				-0.35+float64(index)*0.17,
				0.04*float64(index%3-1),
				0,
			),
		})
	}
	return poses
}

func (g *Game) updateRespawns() {
	// Respawns are wave-based: keep the current engagement finite and only
	// replenish it after every autonomous fighter has been destroyed.
	if len(g.controllers) > 0 {
		return
	}
	for _, request := range g.respawns {
		if request.readyAt > g.simulationTime {
			return
		}
	}
	for _, request := range g.respawns {
		g.spawnAutonomousFighter(request.definition)
	}
	g.respawns = g.respawns[:0]
}

func (g *Game) spawnAutonomousFighter(definition string) {
	center := g.initialPose.Position
	var headingTarget *scene.Object
	if hangarPose, ok := g.hangarSpawnPose(int(g.respawnSequence)); ok {
		center = hangarPose.Position
	} else if player := g.objectByID(fighterID); player != nil {
		center = player.Pose.Position
		if nearest := g.nearestAutonomousTo(center); nearest != nil {
			away := center.Sub(nearest.Pose.Position).Normalize()
			if away == (math3d.Vec3{}) {
				away = math3d.Vec3{Z: -1}
			}
			candidate := center.Add(away.Scale(g.profile.Swarm.RespawnDistance))
			if g.positionIsSafe(candidate, 6.0) {
				center = candidate
				headingTarget = nearest
			}
		}
	}
	pose := g.safeAutonomousSpawnPose(center)
	if hangarPose, ok := g.hangarSpawnPose(int(g.respawnSequence)); ok {
		pose = hangarPose
	}
	if headingTarget != nil {
		direction := headingTarget.Pose.Position.Sub(pose.Position).Normalize()
		if direction != (math3d.Vec3{}) {
			pose.Orientation = math3d.QuaternionFromYawPitchRoll(
				math.Atan2(direction.X, direction.Z),
				-math.Asin(max(-1, min(1, direction.Y))),
				0,
			)
		}
	}
	id := g.nextObjectID
	g.nextObjectID++
	if definition == "" {
		definition = g.profile.Swarm.Object
	}
	fighter, err := g.catalogRegistry.Create(definition, id, pose)
	if err != nil {
		return
	}
	fighter.Motion.Speed = g.profile.Swarm.InitialSpeed + g.profile.Swarm.SpeedStep*float64(g.respawnSequence%uint64(max(1, g.profile.Swarm.Count)))
	fighter.Team = g.profile.Swarm.Team
	controller, err := g.controllerRegistry.Create(g.profile.Swarm.Controller, uint64(id)*0x9e3779b97f4a7c15, g.profile.Swarm.Pursuit)
	if err != nil {
		return
	}
	g.objects = append(g.objects, fighter)
	g.controllers[id] = controller
	g.respawnSequence++
}

// hangarSpawnPose places a fighter just beyond the Death Star's surface in a
// deterministic launch formation. The formation is derived from the host
// object's pose and radius, so it remains valid if the Death Star is moved or
// replaced by another large static object later.
func (g *Game) hangarSpawnPose(index int) (kinematics.Pose, bool) {
	for _, host := range g.objects {
		if host.Definition != catalog.DeathStarName || host.CollisionRadius <= 0 {
			continue
		}
		outward := host.Pose.Orientation.Rotate(math3d.Vec3{Z: -1}).Normalize()
		if outward == (math3d.Vec3{}) {
			outward = math3d.Vec3{Z: -1}
		}
		right := host.Pose.Orientation.Rotate(math3d.Vec3{X: 1}).Normalize()
		up := host.Pose.Orientation.Rotate(math3d.Vec3{Y: 1}).Normalize()
		column := float64(index % 3)
		row := float64((index / 3) % 3)
		position := host.Pose.Position.Add(outward.Scale(host.CollisionRadius + 10))
		position = position.Add(right.Scale((column - 1) * 14))
		position = position.Add(up.Scale((row - 1) * 10))
		return kinematics.Pose{
			Position:    position,
			Orientation: orientationToward(outward),
		}, true
	}
	return kinematics.Pose{}, false
}

func orientationToward(direction math3d.Vec3) math3d.Quaternion {
	direction = direction.Normalize()
	if direction == (math3d.Vec3{}) {
		return math3d.IdentityQuaternion()
	}
	return math3d.QuaternionFromYawPitchRoll(
		math.Atan2(direction.X, direction.Z),
		-math.Asin(max(-1, min(1, direction.Y))),
		0,
	)
}

func (g *Game) nearestAutonomousTo(position math3d.Vec3) *scene.Object {
	var nearest *scene.Object
	nearestDistance := math.Inf(1)
	for id := range g.controllers {
		object := g.objectByID(id)
		if object == nil {
			continue
		}
		distance := object.Pose.Position.Sub(position).Length()
		if distance < nearestDistance {
			copy := *object
			nearest = &copy
			nearestDistance = distance
		}
	}
	return nearest
}

func (g *Game) safeAutonomousSpawnPose(center math3d.Vec3) kinematics.Pose {
	for attempt := range 12 {
		sequence := float64(g.respawnSequence + uint64(attempt))
		angle := sequence * math.Pi * (3 - math.Sqrt(5))
		position := center.Add(math3d.Vec3{
			X: math.Cos(angle) * g.profile.Swarm.SpawnRadius,
			Y: math.Sin(sequence*1.17) * 3.5,
			Z: math.Sin(angle) * g.profile.Swarm.SpawnRadius,
		})
		if g.positionIsSafe(position, 4.5) {
			direction := center.Sub(position).Normalize()
			yaw := math.Atan2(direction.X, direction.Z)
			pitch := -math.Asin(max(-1, min(1, direction.Y)))
			return kinematics.Pose{
				Position:    position,
				Orientation: math3d.QuaternionFromYawPitchRoll(yaw, pitch, 0),
			}
		}
	}
	return kinematics.Pose{
		Position:    center.Add(math3d.Vec3{Z: -g.profile.Swarm.SpawnRadius * 1.5}),
		Orientation: math3d.IdentityQuaternion(),
	}
}

func (g *Game) positionIsSafe(position math3d.Vec3, minimumDistance float64) bool {
	for _, object := range g.objects {
		if normalizedObjectFrame(object) == scene.ExteriorFrame && object.CollisionRole == scene.CollisionSolid && object.Pose.Position.Sub(position).Length() < minimumDistance+object.CollisionRadius {
			return false
		}
	}
	return true
}

func (g *Game) beginSurfaceEncounter(frame scene.FrameID, participant scene.ObjectID) {
	runtime := g.surfaceRuntime(frame)
	if runtime == nil {
		return
	}
	if runtime.encounter.started {
		if current := g.objectByID(runtime.encounter.participant); current == nil || normalizedObjectFrame(*current) != frame {
			runtime.encounter.participant = participant
		}
		return
	}
	runtime.encounter.started = true
	runtime.encounter.participant = participant
	runtime.encounter.nextWaveAt = g.simulationTime + g.profile.Surface.ReinforcementDelay
	g.moveAttackersToSurface(runtime, participant, g.profile.Surface.InitialAttackers)
}

// updateSurfaceEncounters advances deterministic host/frame-scoped combat.
// It consumes authoritative objects and feature state only; camera visibility
// and streamed horizon tiles cannot start, stop, or retime an encounter.
func (g *Game) updateSurfaceEncounters() {
	for runtimeIndex := range g.environments {
		runtime := &g.environments[runtimeIndex]
		if !runtime.encounter.started {
			continue
		}
		phase, active := g.yavinSurfaceMissionPhase(runtime)
		if !active {
			continue
		}
		participant := g.objectByID(runtime.encounter.participant)
		if participant == nil || normalizedObjectFrame(*participant) != runtime.bound.FrameID || participant.Team != g.profile.Player.Team {
			participant = g.nearestTeamObject(runtime.bound.FrameID, g.profile.Player.Team, math3d.Vec3{})
			if participant == nil {
				continue
			}
			runtime.encounter.participant = participant.ID
		}
		attackerLimit := g.profile.Surface.InitialAttackers
		if phase >= sim.MissionTrenchRun {
			attackerLimit = g.profile.Surface.MaxAttackers
			// Entering the trench turns a surface skirmish into a pursued attack
			// run. Promote one immediate deterministic wave, then retain the
			// existing capped reinforcement cadence.
			if runtime.encounter.missionPhase < sim.MissionTrenchRun {
				runtime.encounter.nextWaveAt = g.simulationTime
			}
		}
		runtime.encounter.missionPhase = phase
		activeAttackers := g.countControlledTeam(runtime.bound.FrameID, g.profile.Swarm.Team)
		if activeAttackers < attackerLimit && g.simulationTime >= runtime.encounter.nextWaveAt {
			waveSize := min(2, attackerLimit-activeAttackers)
			g.moveAttackersToSurface(runtime, participant.ID, waveSize)
			runtime.encounter.wave++
			runtime.encounter.nextWaveAt = g.simulationTime + g.profile.Surface.ReinforcementDelay
		}
		g.updateSurfaceCannons(runtime)
	}
}

// yavinSurfaceMissionPhase keeps surface pressure tied to the active
// authoritative mission rather than to camera state or streamed tile detail.
func (g *Game) yavinSurfaceMissionPhase(runtime *localEnvironment) (sim.MissionPhase, bool) {
	if runtime == nil || g.world == nil {
		return sim.MissionInactive, false
	}
	mission := g.world.Mission
	if mission.ID != yavinMissionID || mission.Phase < sim.MissionSurfaceAssault || mission.Phase >= sim.MissionEscape {
		return sim.MissionInactive, false
	}
	player := g.objectByID(mission.PlayerID)
	if player == nil || normalizedObjectFrame(*player) != runtime.bound.FrameID {
		return sim.MissionInactive, false
	}
	return mission.Phase, true
}

func (g *Game) countControlledTeam(frame scene.FrameID, team scene.TeamID) int {
	count := 0
	for id := range g.controllers {
		if object := g.objectByID(id); object != nil && normalizedObjectFrame(*object) == frame && object.Team == team {
			count++
		}
	}
	return count
}

func (g *Game) moveAttackersToSurface(runtime *localEnvironment, participantID scene.ObjectID, requested int) int {
	if runtime == nil || requested <= 0 || g.world == nil {
		return 0
	}
	participant := g.objectByID(participantID)
	if participant == nil {
		return 0
	}
	ids := make([]scene.ObjectID, 0, len(g.controllers))
	for id := range g.controllers {
		object := g.objectByID(id)
		if object == nil || object.Team == participant.Team || normalizedObjectFrame(*object) == runtime.bound.FrameID {
			continue
		}
		if _, transitioning := g.transitions[id]; !transitioning {
			ids = append(ids, id)
		}
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	if len(ids) > requested {
		ids = ids[:requested]
	}
	offsets := [...]math3d.Vec3{
		{Y: 7, Z: 52}, {X: -24, Y: 9, Z: 40}, {X: 24, Y: 8, Z: 44},
		{X: -15, Y: 12, Z: -36}, {X: 18, Y: 10, Z: -42},
	}
	g.world.Objects = g.objects
	moved := 0
	for index, id := range ids {
		if err := g.world.Apply(sim.Transfer{ObjectID: id, Destination: runtime.bound.FrameID, Anchor: "surface-encounter"}); err != nil {
			continue
		}
		object := g.worldObjectByID(id)
		if object == nil {
			continue
		}
		offset := offsets[(runtime.encounter.wave*2+index)%len(offsets)]
		object.Pose.Position = participant.Pose.Position.Add(participant.Pose.Orientation.Rotate(offset))
		direction := participant.Pose.Position.Sub(object.Pose.Position).Normalize()
		object.Pose.Orientation = orientationToward(direction)
		object.Motion = kinematics.Motion{Speed: g.profile.Surface.CruiseSpeed + 0.2*float64(index)}
		g.controllerTargets[id] = participant.ID
		if starter, ok := g.controllers[id].(control.EngagementStarter); ok {
			starter.EngageNow()
		}
		moved++
	}
	g.objects = g.world.Objects
	return moved
}

func (g *Game) nearestTeamObject(frame scene.FrameID, team scene.TeamID, position math3d.Vec3) *scene.Object {
	var nearest *scene.Object
	nearestDistance := math.Inf(1)
	for index := range g.objects {
		candidate := &g.objects[index]
		if candidate.Team != team || !candidate.Targetable || normalizedObjectFrame(*candidate) != frame {
			continue
		}
		distance := candidate.Pose.Position.Sub(position).Length()
		if distance < nearestDistance {
			nearest, nearestDistance = candidate, distance
		}
	}
	return nearest
}

type activeCannon struct {
	feature  environment.Feature
	target   scene.Object
	distance float64
}

func (g *Game) updateSurfaceCannons(runtime *localEnvironment) {
	if runtime == nil || g.profile.Surface.MaxActiveCannons == 0 {
		return
	}
	hostTeam := scene.TeamEmpire
	if host := g.objectByID(runtime.bound.HostID); host != nil && host.Team != scene.TeamNeutral {
		hostTeam = host.Team
	}
	cannons := make([]activeCannon, 0, g.profile.Surface.MaxActiveCannons)
	for _, tile := range runtime.tiles {
		for _, feature := range tile.Features {
			state := runtime.featureStates[feature.ID]
			if feature.Kind != "cannon" || state.Disabled || state.Destroyed {
				continue
			}
			target := g.nearestHostileForCannon(runtime.bound.FrameID, hostTeam, feature)
			if target.ID == 0 {
				continue
			}
			distance := target.Pose.Position.Sub(feature.Pose.Position).Length()
			if distance <= g.profile.Surface.CannonRange {
				cannons = append(cannons, activeCannon{feature: feature, target: target, distance: distance})
			}
		}
	}
	sort.Slice(cannons, func(i, j int) bool {
		if cannons[i].distance == cannons[j].distance {
			return cannons[i].feature.ID < cannons[j].feature.ID
		}
		return cannons[i].distance < cannons[j].distance
	})
	if len(cannons) > g.profile.Surface.MaxActiveCannons {
		cannons = cannons[:g.profile.Surface.MaxActiveCannons]
	}
	for _, cannon := range cannons {
		aim := runtime.encounter.cannonAim[cannon.feature.ID]
		muzzle := cannonMuzzle(cannon.feature, aim, 0)
		point := g.cannonAimPoint(cannon.feature, cannon.target, muzzle, aim.shots)
		desiredYaw, desiredPitch := cannonAngles(cannon.feature, point)
		desiredYaw = max(-g.profile.Surface.CannonYawLimit, min(g.profile.Surface.CannonYawLimit, desiredYaw))
		desiredPitch = max(-g.profile.Surface.CannonPitchLimit, min(g.profile.Surface.CannonPitchLimit, desiredPitch))
		step := g.profile.Surface.CannonTraverseSpeed * g.profile.Simulation.TickSeconds
		aim.yaw = moveAngleToward(aim.yaw, desiredYaw, step)
		aim.pitch = moveAngleToward(aim.pitch, desiredPitch, step)
		runtime.encounter.cannonAim[cannon.feature.ID] = aim
		readyAt, initialized := runtime.encounter.cannonReadyAt[cannon.feature.ID]
		if !initialized {
			seed := stableStringSeed(cannon.feature.ID)
			readyAt = g.simulationTime + 0.2 + (deterministicSigned(&seed)+1)*0.35
			runtime.encounter.cannonReadyAt[cannon.feature.ID] = readyAt
		}
		if g.simulationTime < readyAt || g.countProjectilesForTeam(hostTeam) >= 18 ||
			math.Abs(aim.yaw-desiredYaw) > g.profile.Surface.CannonFireTolerance ||
			math.Abs(aim.pitch-desiredPitch) > g.profile.Surface.CannonFireTolerance ||
			!g.cannonLineOfFireClear(runtime, cannon.feature, aim, cannon.target) {
			continue
		}
		if !g.fireSurfaceCannon(runtime, cannon.feature, hostTeam, aim) {
			continue
		}
		aim.shots++
		runtime.encounter.cannonAim[cannon.feature.ID] = aim
		seed := stableStringSeed(cannon.feature.ID) ^ uint64(runtime.encounter.wave+1)*0x9e3779b97f4a7c15 ^ uint64(g.world.Tick)
		amount := (deterministicSigned(&seed) + 1) * 0.5
		gap := g.profile.Surface.CannonFireMinGap + amount*(g.profile.Surface.CannonFireMaxGap-g.profile.Surface.CannonFireMinGap)
		runtime.encounter.cannonReadyAt[cannon.feature.ID] = g.simulationTime + gap
	}
}

func moveAngleToward(current, target, maxStep float64) float64 {
	return current + max(-maxStep, min(maxStep, target-current))
}

func cannonAngles(feature environment.Feature, point math3d.Vec3) (float64, float64) {
	local := feature.Pose.Orientation.Conjugate().Rotate(point.Sub(feature.Pose.Position))
	scale := feature.Scale
	if scale.X > 0 && scale.Y > 0 && scale.Z > 0 {
		local = math3d.Vec3{X: local.X / scale.X, Y: local.Y / scale.Y, Z: local.Z / scale.Z}
	}
	fromTurret := local.Sub(feature.TurretPivot)
	yaw := math.Atan2(fromTurret.X, fromTurret.Z)
	unyawed := math3d.QuaternionFromAxisAngle(math3d.Vec3{Y: 1}, -yaw).Rotate(fromTurret).Add(feature.TurretPivot)
	fromBarrel := unyawed.Sub(feature.BarrelPivot)
	return yaw, -math.Atan2(fromBarrel.Y, math.Hypot(fromBarrel.X, fromBarrel.Z))
}

func cannonMuzzle(feature environment.Feature, aim cannonAimState, index int) math3d.Vec3 {
	if len(feature.Muzzles) == 0 {
		return feature.Pose.Position
	}
	return feature.Matrix().Mul(feature.TurretMatrix(aim.yaw, aim.pitch)).TransformPoint(feature.Muzzles[index%len(feature.Muzzles)])
}

func cannonDirection(feature environment.Feature, aim cannonAimState) math3d.Vec3 {
	return feature.Matrix().Mul(feature.TurretMatrix(aim.yaw, aim.pitch)).TransformDirection(math3d.Vec3{Z: 1}).Normalize()
}

func (g *Game) cannonAimPoint(feature environment.Feature, target scene.Object, muzzle math3d.Vec3, shots uint64) math3d.Vec3 {
	velocity := target.Pose.Forward().Scale(target.Motion.Speed).Add(target.Motion.Velocity)
	lead := min(0.6, target.Pose.Position.Sub(muzzle).Length()/g.profile.Combat.Laser.Speed)
	point := target.Pose.Position.Add(velocity.Scale(lead * g.profile.Simulation.MotionScale))
	seed := stableStringSeed(feature.ID) ^ (shots+1)*0x517cc1b727220a95
	return point.Add(target.Pose.Orientation.Rotate(math3d.Vec3{
		X: deterministicSigned(&seed) * g.profile.Surface.CannonAimError,
		Y: deterministicSigned(&seed) * g.profile.Surface.CannonAimError * 0.7,
	}))
}

func (g *Game) cannonLineOfFireClear(runtime *localEnvironment, feature environment.Feature, aim cannonAimState, target scene.Object) bool {
	start := cannonMuzzle(feature, aim, int(aim.shots))
	end := target.Pose.Position
	for _, tile := range runtime.tiles {
		if tile.Bounds.Valid() {
			if _, nearTile := collision.SegmentSphere(start, end, tile.Bounds.Center, tile.Bounds.Radius+0.02); !nearTile {
				continue
			}
		}
		for _, plane := range tile.Planes {
			if hit, blocked := collision.SweepSpherePlane(start, end, 0.02, plane); blocked && hit.Time < 0.98 {
				return false
			}
			if hit, blocked := collision.SweepSpherePlane(end, start, 0.02, plane); blocked && hit.Time < 0.98 {
				return false
			}
		}
		for _, box := range tile.Boxes {
			if string(box.FeatureID) == feature.ID || runtime.featureStates[string(box.FeatureID)].Destroyed {
				continue
			}
			if hit, blocked := collision.SweepSphereBox(start, end, 0.02, box); blocked && hit.Time < 0.98 {
				return false
			}
		}
	}
	return true
}

func cannonTargetInArc(feature environment.Feature, target scene.Object) bool {
	if len(feature.Muzzles) == 0 {
		return true
	}
	forward := feature.Pose.Orientation.Rotate(math3d.Vec3{Z: 1})
	return target.Pose.Position.Sub(feature.Pose.Position).Dot(forward) > 0
}

func (g *Game) nearestHostileForCannon(frame scene.FrameID, team scene.TeamID, feature environment.Feature) scene.Object {
	nearestDistance := math.Inf(1)
	var nearest scene.Object
	for _, candidate := range g.objects {
		if candidate.Team == scene.TeamNeutral || candidate.Team == team || !candidate.Targetable || normalizedObjectFrame(candidate) != frame || !cannonTargetInArc(feature, candidate) {
			continue
		}
		yaw, pitch := cannonAngles(feature, candidate.Pose.Position)
		if math.Abs(yaw) > g.profile.Surface.CannonYawLimit || math.Abs(pitch) > g.profile.Surface.CannonPitchLimit {
			continue
		}
		distance := candidate.Pose.Position.Sub(feature.Pose.Position).Length()
		if distance < nearestDistance || (distance == nearestDistance && candidate.ID < nearest.ID) {
			nearest, nearestDistance = candidate, distance
		}
	}
	return nearest
}

func (g *Game) countProjectilesForTeam(team scene.TeamID) int {
	count := 0
	for _, object := range g.objects {
		if object.CollisionRole == scene.CollisionProjectile && object.Team == team {
			count++
		}
	}
	return count
}

func (g *Game) fireSurfaceCannon(runtime *localEnvironment, feature environment.Feature, team scene.TeamID, aim cannonAimState) bool {
	muzzle := cannonMuzzle(feature, aim, int(aim.shots))
	direction := cannonDirection(feature, aim)
	shooter := scene.Object{
		ID: runtime.bound.HostID, Definition: catalog.TIEFighterName, Team: team, Frame: runtime.bound.FrameID,
		Pose:    kinematics.Pose{Position: muzzle, Orientation: math3d.IdentityQuaternion()},
		Anchors: map[string]kinematics.Pose{"muzzle": {Orientation: math3d.IdentityQuaternion()}},
	}
	laser := g.profile.Combat.Laser
	laser.Lifetime = g.profile.Surface.CannonBoltLifetime
	spawn, err := combat.FireLaserTowardWithConfig(shooter, g.nextObjectID, "muzzle", muzzle.Add(direction.Scale(g.profile.Surface.CannonRange)), laser)
	if err != nil {
		return false
	}
	g.nextObjectID++
	g.objects = append(g.objects, spawn.Object)
	g.projectiles[spawn.Object.ID] = spawn.Lifetime
	g.owners[spawn.Object.ID] = spawn.OwnerID
	return true
}

func stableStringSeed(value string) uint64 {
	seed := uint64(1469598103934665603)
	for index := 0; index < len(value); index++ {
		seed ^= uint64(value[index])
		seed *= 1099511628211
	}
	return seed
}

func (g *Game) updateAutonomous(seconds float64) {
	solidSnapshots := make([]scene.Object, 0, len(g.objects))
	for _, object := range g.objects {
		if object.CollisionRole == scene.CollisionSolid {
			solidSnapshots = append(solidSnapshots, object)
		}
	}
	for id, controller := range g.controllers {
		if !g.swarmLaunched {
			continue
		}
		object := g.objectByID(id)
		if object == nil {
			continue
		}
		nearby := make([]scene.Object, 0, len(solidSnapshots)-1)
		for _, candidate := range solidSnapshots {
			if candidate.ID != id && sameFrame(*object, candidate) {
				nearby = append(nearby, candidate)
			}
		}
		controllerTarget := g.selectControllerTarget(*object)
		if controllerTarget.ID != 0 {
			g.controllerTargets[id] = controllerTarget.ID
		} else {
			delete(g.controllerTargets, id)
		}
		context := control.Context{
			Self:        *object,
			Target:      controllerTarget,
			Nearby:      nearby,
			Seconds:     seconds,
			MotionScale: g.profile.Simulation.MotionScale,
		}
		decision := controller.Decide(context)
		limits := g.profile.Swarm.Flight
		if surface := g.surfaceRuntime(normalizedObjectFrame(*object)); surface != nil {
			decision.Flight = g.applySurfaceGuidance(surface, *object, decision.Flight)
			limits.MaxForward = g.profile.Surface.MaxForward * 1.08
			limits.MaxReverse = limits.MaxForward
			limits.Acceleration = max(limits.Acceleration, g.profile.Surface.Acceleration)
			if object.Motion.Speed < g.profile.Surface.CruiseSpeed {
				decision.Flight.Throttle = 1
			}
		}
		object.Motion = control.ApplyWithLimits(object.Motion, decision.Flight, limits, seconds)
		if controllerTarget.ID != 0 {
			if decision.Fire {
				g.fireAutonomousLaser(*object, controllerTarget)
			}
		}
	}
}

// applySurfaceGuidance composes local terrain constraints around a strategy's
// objective. It does not replace pursuit: it only adds bounded climb and
// obstacle-avoidance intent when the predicted path becomes unsafe.
func (g *Game) applySurfaceGuidance(runtime *localEnvironment, object scene.Object, intent control.Intent) control.Intent {
	altitude, found := g.surfaceAltitude(runtime, object.Pose.Position)
	if found {
		forward := object.Pose.Forward().Normalize()
		up := runtime.bound.Definition.LevelUp.Normalize()
		lookAhead := g.surfaceGuidanceLookAhead(object)
		predicted := altitude + forward.Dot(up)*lookAhead
		if predicted < g.profile.Surface.MinimumAltitude {
			deficit := (g.profile.Surface.MinimumAltitude - predicted) / g.profile.Surface.MinimumAltitude
			// Once the projected flight path enters the protected deck envelope,
			// commit to a perceptible climb. A very small proportional correction
			// arrives too late at surface-flight speeds.
			climb := -min(1, 0.35+deficit*g.profile.Surface.GuidanceStrength)
			if intent.Pitch > climb {
				intent.Pitch = climb
			}
		}
	}
	if avoidance, avoid := g.surfaceObstacleAvoidance(runtime, object); avoid {
		intent.Yaw = avoidance.Yaw
		intent.Roll = -avoidance.Yaw * 0.8
		if intent.Pitch > avoidance.Pitch {
			intent.Pitch = avoidance.Pitch
		}
	}
	return intent
}

func (g *Game) surfaceGuidanceLookAhead(object scene.Object) float64 {
	motionScale := max(1, g.profile.Simulation.MotionScale)
	// Roughly three seconds of travel gives angular acceleration time to turn
	// a fighter rather than asking it to dodge at the collision boundary.
	return max(g.profile.Surface.TerrainLookAhead, object.Motion.Speed*motionScale*3)
}

func (g *Game) surfaceAltitude(runtime *localEnvironment, position math3d.Vec3) (float64, bool) {
	if runtime == nil {
		return 0, false
	}
	up := runtime.bound.Definition.LevelUp.Normalize()
	best := math.Inf(1)
	found := false
	for _, tile := range runtime.tiles {
		for _, plane := range tile.Planes {
			normal := plane.Normal.Normalize()
			if normal.Dot(up) < 0.9 {
				continue
			}
			offset := position.Sub(plane.Center)
			axisU := plane.AxisU.Normalize()
			axisV := normal.Cross(axisU).Normalize()
			if math.Abs(offset.Dot(axisU)) > plane.HalfU || math.Abs(offset.Dot(axisV)) > plane.HalfV {
				continue
			}
			altitude := offset.Dot(normal)
			if altitude >= 0 && altitude < best {
				best, found = altitude, true
			}
		}
	}
	return best, found
}

type surfaceAvoidanceIntent struct {
	Yaw   float64
	Pitch float64
}

// surfaceObstacleAvoidance performs the same swept-volume queries used by
// collision resolution, but against a forward prediction. It therefore
// understands installation volumes and trench/deck planes without maintaining
// a second approximate obstacle representation.
func (g *Game) surfaceObstacleAvoidance(runtime *localEnvironment, object scene.Object) (surfaceAvoidanceIntent, bool) {
	if runtime == nil {
		return surfaceAvoidanceIntent{}, false
	}
	forward := object.Pose.Forward().Normalize()
	right := object.Pose.Orientation.Rotate(math3d.Vec3{X: 1}).Normalize()
	up := runtime.bound.Definition.LevelUp.Normalize()
	lookAhead := g.surfaceGuidanceLookAhead(object)
	end := object.Pose.Position.Add(forward.Scale(lookAhead))
	safetyRadius := object.CollisionRadius + 2.5
	nearestTime := math.Inf(1)
	result := surfaceAvoidanceIntent{}
	found := false
	for _, tile := range runtime.tiles {
		for _, plane := range tile.Planes {
			hit, collisionRisk := collision.SweepSpherePlane(object.Pose.Position, end, safetyRadius, plane)
			if !collisionRisk || hit.Time >= nearestTime {
				continue
			}
			nearestTime, found = hit.Time, true
			normal := hit.Normal.Normalize()
			if normal.Dot(up) > 0.7 {
				result = surfaceAvoidanceIntent{Pitch: -1}
				continue
			}
			// Trench and terminal walls provide a safe inward normal. Convert it
			// to the fighter's local horizontal steering direction.
			yaw := sign(normal.Dot(right))
			if yaw == 0 {
				yaw = deterministicAvoidanceSide(object.ID, string(hit.FeatureID))
			}
			result = surfaceAvoidanceIntent{Yaw: yaw, Pitch: -0.25}
		}
		for _, box := range tile.Boxes {
			if runtime.featureStates[string(box.FeatureID)].Destroyed {
				continue
			}
			hit, collisionRisk := collision.SweepSphereBox(object.Pose.Position, end, safetyRadius, box)
			if !collisionRisk || hit.Time >= nearestTime {
				continue
			}
			nearestTime, found = hit.Time, true
			lateral := box.Center.Sub(object.Pose.Position).Dot(right)
			yaw := -sign(lateral)
			if math.Abs(lateral) < 0.25 {
				yaw = deterministicAvoidanceSide(object.ID, string(box.FeatureID))
			}
			// Surface installations are best passed with a bank plus a modest
			// climb; the climb also protects against tall cannon silhouettes.
			result = surfaceAvoidanceIntent{Yaw: yaw, Pitch: -0.55}
		}
	}
	return result, found
}

func deterministicAvoidanceSide(objectID scene.ObjectID, featureID string) float64 {
	if (uint64(objectID)^stableStringSeed(featureID))&1 == 0 {
		return -1
	}
	return 1
}

func sign(value float64) float64 {
	if value < 0 {
		return -1
	}
	if value > 0 {
		return 1
	}
	return 0
}

// selectControllerTarget preserves a valid target and otherwise selects the
// nearest targetable hostile in the same spatial frame. Team and frame are
// authoritative; model names, colors, and the local human-player ID are not.
func (g *Game) selectControllerTarget(self scene.Object) scene.Object {
	eligible := func(candidate scene.Object) bool {
		return candidate.ID != self.ID && candidate.Targetable && candidate.Team != scene.TeamNeutral &&
			self.Team != scene.TeamNeutral && candidate.Team != self.Team && sameFrame(self, candidate)
	}
	if current := g.objectByID(g.controllerTargets[self.ID]); current != nil && eligible(*current) {
		return *current
	}
	nearestDistance := math.Inf(1)
	var nearest scene.Object
	for _, candidate := range g.objects {
		if !eligible(candidate) {
			continue
		}
		distance := candidate.Pose.Position.Sub(self.Pose.Position).Length()
		if distance < nearestDistance || (distance == nearestDistance && candidate.ID < nearest.ID) {
			nearest, nearestDistance = candidate, distance
		}
	}
	return nearest
}

func (g *Game) fireAutonomousLaser(shooter, target scene.Object) bool {
	leadTime := shooter.Pose.Position.Sub(target.Pose.Position).Length() / g.profile.Combat.Laser.Speed
	leadTime = min(0.45, leadTime)
	targetVelocity := target.Pose.Forward().Scale(target.Motion.Speed).Add(target.Motion.Velocity)
	aimPoint := target.Pose.Position.Add(targetVelocity.Scale(leadTime * g.profile.Simulation.MotionScale))
	// Keep autonomous attacks deterministic but imperfect. Error is expressed
	// in the target's local lateral/vertical axes, so it remains a believable
	// miss as the player turns instead of becoming world-axis drift.
	seed := uint64(shooter.ID)*0x9e3779b97f4a7c15 ^ uint64(math.Max(0, math.Floor(g.simulationTime*60)))
	errorX := deterministicSigned(&seed) * g.profile.Swarm.AimError
	errorY := deterministicSigned(&seed) * g.profile.Swarm.AimError * 0.7
	aimPoint = aimPoint.Add(target.Pose.Orientation.Rotate(math3d.Vec3{X: errorX, Y: errorY}))
	spawned := false
	for _, muzzle := range []string{"muzzle-upper-left", "muzzle-upper-right"} {
		spawn, err := combat.FireLaserTowardWithConfig(shooter, g.nextObjectID, muzzle, aimPoint, g.profile.Combat.Laser)
		if err != nil {
			continue
		}
		spawn.Lifetime = g.laserConvergenceLifetime(spawn, aimPoint)
		g.nextObjectID++
		g.objects = append(g.objects, spawn.Object)
		g.objects[len(g.objects)-1].Frame = shooter.Frame
		g.projectiles[spawn.Object.ID] = spawn.Lifetime
		g.owners[spawn.Object.ID] = spawn.OwnerID
		spawned = true
	}
	return spawned
}

func deterministicSigned(state *uint64) float64 {
	value := *state
	value ^= value << 13
	value ^= value >> 7
	value ^= value << 17
	*state = value
	return 2*(float64(value>>11)/float64(uint64(1)<<53)) - 1
}

func (g *Game) fireLaser() bool {
	if g.fireCooldown > 0 || !g.withinFireRateLimit() {
		return false
	}
	fighter := g.objectByID(fighterID)
	if fighter == nil {
		return false
	}
	// Firing is an explicit player action, so return immediately to the
	// player's cockpit even if the camera was following a swarm fighter.
	g.viewCamera.TargetID = fighterID
	g.viewCamera.Mode = camera.Cockpit
	muzzlePairs := [...][2]string{
		{"muzzle-upper-left", "muzzle-upper-right"},
		{"muzzle-lower-left", "muzzle-lower-right"},
	}
	pair := g.nextMuzzlePair
	aimTarget, aimed := g.cockpitAimTarget()
	for _, muzzle := range muzzlePairs[pair] {
		var spawn combat.Spawn
		var err error
		if aimed {
			spawn, err = combat.FireLaserTowardWithConfig(*fighter, g.nextObjectID, muzzle, aimTarget, g.profile.Combat.Laser)
			if err == nil {
				spawn.Lifetime = g.laserConvergenceLifetime(spawn, aimTarget)
			}
		} else {
			spawn, err = combat.FireLaserWithConfig(*fighter, g.nextObjectID, muzzle, g.profile.Combat.Laser)
		}
		if err != nil {
			return false
		}
		g.nextObjectID++
		g.objects = append(g.objects, spawn.Object)
		g.objects[len(g.objects)-1].Frame = fighter.Frame
		g.projectiles[spawn.Object.ID] = spawn.Lifetime
		g.owners[spawn.Object.ID] = spawn.OwnerID
	}
	g.laserBeamPair = pair
	g.laserBeamTime = g.profile.Combat.BeamTime
	g.nextMuzzlePair = (pair + 1) % len(muzzlePairs)
	g.fireCooldown = g.profile.Combat.FireInterval
	g.fireHistory = append(g.fireHistory, g.simulationTime)
	return true
}

func (g *Game) fireProtonTorpedo() bool {
	if g.torpedoCooldown > 0 || g.torpedoesRemaining <= 0 {
		return false
	}
	fighter := g.objectByID(fighterID)
	if fighter == nil || fighter.Team != scene.TeamAlliance {
		return false
	}
	target := fighter.Pose.Position.Add(fighter.Pose.Forward().Scale(g.profile.Targeting.AimConvergence))
	if aimedTarget, aimed := g.cockpitAimTarget(); aimed {
		target = aimedTarget
	}
	launcher := "muzzle-lower-left"
	if g.torpedoesRemaining%2 == 0 {
		launcher = "muzzle-lower-right"
	}
	spawn, err := combat.FireProtonTorpedoToward(*fighter, g.nextObjectID, launcher, target, g.profile.Combat.Torpedo)
	if err != nil {
		return false
	}
	g.nextObjectID++
	g.objects = append(g.objects, spawn.Object)
	g.projectiles[spawn.Object.ID] = spawn.Lifetime
	g.owners[spawn.Object.ID] = spawn.OwnerID
	g.torpedoesRemaining--
	g.torpedoCooldown = g.profile.Combat.Torpedo.Cooldown
	g.viewCamera.TargetID = fighterID
	g.viewCamera.Mode = camera.Cockpit
	return true
}

// laserConvergenceLifetime stops an aimed bolt at the point its cannon pair
// was instructed to meet. World motion is advanced using MotionScale, so the
// effective travel speed must include that multiplier.
func (g *Game) laserConvergenceLifetime(spawn combat.Spawn, target math3d.Vec3) float64 {
	effectiveSpeed := spawn.Object.Motion.Speed * g.profile.Simulation.MotionScale
	if effectiveSpeed <= 0 {
		return spawn.Lifetime
	}
	distance := target.Sub(spawn.Object.Pose.Position).Length()
	if distance <= 0 {
		return 0.01
	}
	return distance / effectiveSpeed
}

func (g *Game) withinFireRateLimit() bool {
	cutoff := g.simulationTime - g.profile.Combat.FireWindow
	firstActive := 0
	for firstActive < len(g.fireHistory) && g.fireHistory[firstActive] <= cutoff {
		firstActive++
	}
	if firstActive > 0 {
		g.fireHistory = append(g.fireHistory[:0], g.fireHistory[firstActive:]...)
	}
	return len(g.fireHistory) < g.profile.Combat.MaxFireEvents
}

func (g *Game) updateProjectiles(seconds float64) {
	kept := g.objects[:0]
	for _, object := range g.objects {
		remaining, projectile := g.projectiles[object.ID]
		if projectile {
			remaining -= seconds
			if remaining <= 0 {
				delete(g.projectiles, object.ID)
				delete(g.owners, object.ID)
				continue
			}
			g.projectiles[object.ID] = remaining
		}
		kept = append(kept, object)
	}
	g.objects = kept
}

func (g *Game) resetFighter() {
	fighter := g.objectByID(fighterID)
	if fighter == nil {
		g.respawnPlayer()
		return
	}
	g.resetYavinMission()
	// Restarting always returns the player to the shared orbital frame. A
	// surface-frame restart must not leave the initial pose interpreted in the
	// Death Star's local coordinates.
	fighter.Frame = scene.ExteriorFrame
	fighter.Pose = g.initialPose
	g.controlsRemaining = 0
	g.controlsPinned = false
	g.destructionViewRemaining = 0
	delete(g.transitions, fighterID)
	delete(g.transitionCommitments, fighterID)
	g.shieldStrength = g.profile.Player.Shield.Maximum
	g.shieldQuietTime = 0
	g.fireCooldown = 0
	g.torpedoCooldown = 0
	g.torpedoesRemaining = g.profile.Combat.Torpedo.Ammunition
	g.fireHistory = g.fireHistory[:0]
	g.starField.Wrap(g.initialPose.Position)
	if g.mode == modeAutopilot {
		fighter.Motion = g.autoMotion
	} else {
		fighter.Motion = kinematics.Motion{}
	}
	g.world.Objects = g.objects
	_ = g.initializeYavinOrbitalProgress()
	if g.flow == flowPlaying {
		g.beginHyperspaceArrival(fighter.Pose)
	}
}

// beginHyperspaceArrival starts the short orbital entry presentation. It is
// intentionally gated by the player's frame so surface-flight resets and
// future local environments retain their own presentation rules.
func (g *Game) beginHyperspaceArrival(target kinematics.Pose) bool {
	if g.profile.Simulation.HyperspaceArrivalTime <= 0 {
		return false
	}
	fighter := g.objectByID(fighterID)
	if fighter == nil || normalizedObjectFrame(*fighter) != scene.ExteriorFrame {
		return false
	}
	forward := target.Forward()
	if forward.Length() <= 1e-9 {
		forward = kinematics.LocalForward
	}
	entry := target
	entry.Position = target.Position.Sub(forward.Scale(72))
	g.hyperspaceArrival = &hyperspaceArrival{
		duration:     g.profile.Simulation.HyperspaceArrivalTime,
		from:         entry,
		to:           target,
		motion:       fighter.Motion,
		previousMode: g.viewCamera.Mode,
	}
	fighter.Pose = entry
	fighter.Motion = kinematics.Motion{}
	g.viewCamera.TargetID = fighterID
	// A chase view makes the arrival readable as a physical re-entry while the
	// streaks in Draw provide the hyperspace cue. The prior view is restored at
	// the end, so a restart does not permanently change cockpit/follow mode.
	g.viewCamera.Mode = camera.Chase
	return true
}

func (g *Game) advanceHyperspaceArrival(seconds float64) {
	arrival := g.hyperspaceArrival
	if arrival == nil {
		return
	}
	arrival.elapsed += max(0, seconds)
	amount := min(1, arrival.elapsed/arrival.duration)
	eased := amount * amount * (3 - 2*amount)
	if fighter := g.objectByID(fighterID); fighter != nil {
		fighter.Pose.Position = arrival.from.Position.Add(arrival.to.Position.Sub(arrival.from.Position).Scale(eased))
		fighter.Pose.Orientation = math3d.Slerp(arrival.from.Orientation, arrival.to.Orientation, eased)
	}
	if amount < 1 {
		return
	}
	if fighter := g.objectByID(fighterID); fighter != nil {
		fighter.Pose = arrival.to
		fighter.Motion = arrival.motion
	}
	g.viewCamera.Mode = arrival.previousMode
	g.hyperspaceArrival = nil
	if g.flow == flowLaunching {
		g.flow = flowPlaying
	}
}

func (g *Game) applyShieldDamage(amount int) bool {
	if amount <= 0 || g.shieldStrength < 0 {
		return g.shieldStrength < 0
	}
	g.shieldStrength -= amount
	g.shieldQuietTime = 0
	return g.shieldStrength < 0
}

func (g *Game) updateShield(seconds float64) {
	maximum := g.profile.Player.Shield.Maximum
	interval := g.profile.Player.Shield.RechargeInterval
	if g.objectByID(fighterID) == nil || g.shieldStrength >= maximum || seconds <= 0 {
		return
	}
	g.shieldQuietTime += seconds
	for g.shieldQuietTime >= interval && g.shieldStrength < maximum {
		g.shieldStrength++
		g.shieldQuietTime -= interval
	}
}

func (g *Game) respawnPlayer() {
	pose := g.safePlayerRespawnPose()
	fighter, err := g.catalogRegistry.Create(g.profile.Player.Object, fighterID, pose)
	if err != nil {
		return
	}
	if g.mode == modeAutopilot {
		fighter.Motion = g.autoMotion
	} else {
		// A respawn always launches at full forward speed; manual control can
		// immediately brake or reverse from this known, energetic starting state.
		fighter.Motion.Speed = g.profile.Player.Flight.MaxForward
	}
	g.objects = append(g.objects, fighter)
	g.world.Objects = g.objects
	g.resetYavinMission()
	g.playerDestroyed = false
	g.destructionVictim = scene.Object{}
	g.controlsRemaining = 0
	g.controlsPinned = false
	g.destructionViewRemaining = 0
	g.shieldStrength = g.profile.Player.Shield.Maximum
	g.shieldQuietTime = 0
	g.fireCooldown = 0
	g.torpedoCooldown = 0
	g.torpedoesRemaining = g.profile.Combat.Torpedo.Ammunition
	g.fireHistory = g.fireHistory[:0]
	g.starField.Wrap(pose.Position)
	g.viewCamera.ClearFixedView()
	g.viewCamera.Mode = g.playerViewMode
	g.viewCamera.TargetID = fighterID
	_ = g.initializeYavinOrbitalProgress()
	if g.flow == flowPlaying {
		g.beginHyperspaceArrival(pose)
	}
}

func (g *Game) resetYavinMission() {
	if g.world == nil || g.world.Mission.ID != yavinMissionID {
		return
	}
	_ = g.world.Apply(sim.ResetMission{})
}

func (g *Game) safePlayerRespawnPose() kinematics.Pose {
	origin := g.initialPose.Position
	candidates := []math3d.Vec3{
		origin,
		origin.Add(math3d.Vec3{Z: -18}),
		origin.Add(math3d.Vec3{X: 18}),
		origin.Add(math3d.Vec3{X: -18}),
		origin.Add(math3d.Vec3{Y: 18}),
		origin.Add(math3d.Vec3{Y: -18}),
		origin.Add(math3d.Vec3{Z: -36}),
	}
	for _, position := range candidates {
		if g.positionIsClear(position, 10.0) {
			pose := g.initialPose
			pose.Position = position
			return pose
		}
	}
	pose := g.initialPose
	pose.Position = origin.Add(math3d.Vec3{Z: -48})
	return pose
}

func (g *Game) positionIsClear(position math3d.Vec3, minimumDistance float64) bool {
	for _, object := range g.objects {
		if normalizedObjectFrame(object) == scene.ExteriorFrame && object.Pose.Position.Sub(position).Length() < minimumDistance+object.CollisionRadius {
			return false
		}
	}
	return true
}

func (g *Game) objectByID(id scene.ObjectID) *scene.Object {
	for index := range g.objects {
		if g.objects[index].ID == id {
			return &g.objects[index]
		}
	}
	return nil
}

func (g *Game) readIntent() control.Intent {
	intent := control.Intent{
		Throttle: keyAxis(ebiten.KeyS, ebiten.KeyW),
		Yaw:      keyAxis(ebiten.KeyArrowLeft, ebiten.KeyArrowRight),
		Pitch:    keyAxis(ebiten.KeyArrowUp, ebiten.KeyArrowDown),
		Roll:     keyAxis(ebiten.KeyQ, ebiten.KeyE),
		Stop:     false,
	}
	if g.viewCamera.Mode == camera.Cockpit && ebiten.IsMouseButtonPressed(ebiten.MouseButtonRight) {
		mouseX, mouseY := ebiten.CursorPosition()
		intent.Yaw, intent.Pitch = cockpitSteeringAxes(mouseX, mouseY, g.profile.Input)
	} else if g.mouseFlight {
		mouseX, mouseY := ebiten.CursorPosition()
		mouseYaw, mousePitch := mouseFlightAxes(mouseX, mouseY, g.mouseNeutralX, g.mouseNeutralY, g.profile.Input)
		intent.Yaw += mouseYaw
		intent.Pitch += mousePitch
	}
	return intent
}

func (g *Game) surfaceRuntime(frame scene.FrameID) *localEnvironment {
	for index := range g.environments {
		runtime := &g.environments[index]
		if runtime.bound.FrameID == frame && runtime.bound.Definition.LevelUp != (math3d.Vec3{}) {
			return runtime
		}
	}
	return nil
}

func (g *Game) playerFlightConfig(fighter scene.Object) control.ManualConfig {
	config := g.profile.Player.Flight
	if g.surfaceRuntime(normalizedObjectFrame(fighter)) != nil {
		config.MaxForward = g.profile.Surface.MaxForward
		config.Acceleration = g.profile.Surface.Acceleration
	}
	return config
}

// applySurfaceAutoLevel adds only a local roll rate. It is intentionally
// downstream of manual intent mapping so explicit roll or turn input always
// wins, and it activates only in an environment that declares a level axis.
func (g *Game) applySurfaceAutoLevel(fighter scene.Object, intent control.Intent, motion kinematics.Motion) kinematics.Motion {
	config := g.profile.Player.AutoLevel
	config.Enabled = g.surfaceAutoLevel
	if !config.Enabled || math.Abs(intent.Yaw) > config.TurnDeadzone || math.Abs(intent.Roll) > config.TurnDeadzone {
		return motion
	}
	frame := normalizedObjectFrame(fighter)
	for _, runtime := range g.environments {
		if runtime.bound.FrameID != frame || runtime.bound.Definition.LevelUp == (math3d.Vec3{}) {
			continue
		}
		motion.RollRate = control.AutoLevelRollRate(fighter.Pose.Orientation, runtime.bound.Definition.LevelUp, config)
		return motion
	}
	return motion
}

func navigationInputPressed() bool {
	return ebiten.IsKeyPressed(ebiten.KeyW) || ebiten.IsKeyPressed(ebiten.KeyS) ||
		ebiten.IsKeyPressed(ebiten.KeyArrowLeft) || ebiten.IsKeyPressed(ebiten.KeyArrowRight) ||
		ebiten.IsKeyPressed(ebiten.KeyArrowUp) || ebiten.IsKeyPressed(ebiten.KeyArrowDown) ||
		ebiten.IsKeyPressed(ebiten.KeyQ) || ebiten.IsKeyPressed(ebiten.KeyE)
}

func (g *Game) drawCockpitOverlay(screen *ebiten.Image) {
	cyan := color.RGBA{R: 40, G: 255, B: 224, A: 255}
	amber := color.RGBA{R: 255, G: 176, B: 32, A: 255}
	cx, cy, targetInRange := g.cockpitTarget()
	g.drawShieldIndicator(screen)
	g.drawSpeedIndicator(screen)
	g.drawThreatIndicator(screen)
	g.drawTargetableIndicator(screen)
	g.drawMissionTargetingComputer(screen)
	targetColor := color.Color(cyan)
	if !targetInRange {
		targetColor = amber
	}

	// Four small arrows point inward while leaving the exact aim point clear.
	drawCockpitArrow(screen, cx-31, cy-23, cx-10, cy-7, targetColor)
	drawCockpitArrow(screen, cx+31, cy-23, cx+10, cy-7, targetColor)
	drawCockpitArrow(screen, cx-31, cy+23, cx-10, cy+7, targetColor)
	drawCockpitArrow(screen, cx+31, cy+23, cx+10, cy+7, targetColor)

	centerX, centerY := float32(ScreenWidth/2), float32(ScreenHeight/2)
	vector.StrokeLine(screen, centerX-4, centerY, centerX+4, centerY, 1, cyan, true)
	vector.StrokeLine(screen, centerX, centerY-4, centerX, centerY+4, 1, cyan, true)
	if ebiten.IsMouseButtonPressed(ebiten.MouseButtonRight) {
		vector.StrokeLine(screen, centerX, centerY, cx, cy, 1, color.RGBA{R: 24, G: 112, B: 112, A: 180}, true)
	}

	// Perspective wireframe cannons: red recessed housings surround three blue
	// barrel rails, echoing the layered vector assemblies of the arcade cockpit.
	layout, exists := g.cockpitRegistry.ForDefinition("builtin/tie-fighter")
	if target := g.objectByID(g.viewCamera.TargetID); target != nil {
		if candidate, ok := g.cockpitRegistry.ForDefinition(target.Definition); ok {
			layout, exists = candidate, true
		}
	}
	if !exists {
		layout = cockpit.Fallback()
	}
	cannons := layout.Cannons
	muzzleTops := make([][2]float32, len(cannons))
	for index, cannon := range cannons {
		drawCockpitCannon(screen, cannon.X, cannon.Y, cx, cy, cannon.Housing, cannon.Barrel)
		muzzleTops[index] = cockpitCannonMuzzleTop(cannon.X, cannon.Y, cx, cy)
	}

	if g.laserBeamTime <= 0 {
		return
	}
	beamColor := color.RGBA{R: 80, G: 255, B: 240, A: uint8(255 * g.laserBeamTime / g.profile.Combat.BeamTime)}
	start := 0
	if g.laserBeamPair == 1 {
		start = 2
	}
	vector.StrokeLine(screen, muzzleTops[start][0], muzzleTops[start][1], cx-5, cy, 3, beamColor, true)
	vector.StrokeLine(screen, muzzleTops[start+1][0], muzzleTops[start+1][1], cx+5, cy, 3, beamColor, true)
}

func (g *Game) drawMissionTargetingComputer(screen *ebiten.Image) {
	if g.viewCamera.TargetID != fighterID || g.world == nil || g.world.Mission.ID != yavinMissionID || g.world.Mission.Phase == sim.MissionInactive {
		return
	}
	const left, top, width, height = float32(305), float32(474), float32(350), float32(42)
	computerColor := color.RGBA{R: 64, G: 180, B: 176, A: 190}
	mission := g.world.Mission
	var computerPath vector.Path
	for _, line := range [][4]float32{
		{left, top, left + width, top}, {left + width, top, left + width, top + height},
		{left + width, top + height, left, top + height}, {left, top + height, left, top},
	} {
		computerPath.MoveTo(line[0], line[1])
		computerPath.LineTo(line[2], line[3])
	}
	status := fmt.Sprintf("%s  T%d", mission.Phase.String(), g.torpedoesRemaining)
	if feedback := missionFeedbackText(mission.Reason); feedback != "" {
		status = feedback
	}
	appendVectorTextPath(&computerPath, left+width/2-18, top+15, status)
	computerDraw := &vector.DrawPathOptions{AntiAlias: true}
	computerDraw.ColorScale.ScaleWithColor(computerColor)
	vector.StrokePath(screen, &computerPath, &vector.StrokeOptions{Width: 1.5}, computerDraw)
	target, ok := g.missionObjectiveTarget()
	if !ok {
		return
	}
	directionX, directionY := objectiveArrowDirection(g.pipeline.View.TransformPoint(target))
	arrowX, arrowY := left+width-24, top+height/2
	red := color.RGBA{R: 224, G: 58, B: 48, A: 175}
	drawCockpitArrow(screen,
		arrowX-float32(directionX)*7, arrowY-float32(directionY)*7,
		arrowX+float32(directionX)*11, arrowY+float32(directionY)*11, red)
}

func missionFeedbackText(reason string) string {
	switch reason {
	case exhaustAttackWrongWeapon:
		return "USE PROTON TORPEDO"
	case exhaustAttackWrongPilot:
		return "INVALID LAUNCHER"
	case exhaustAttackTooClose:
		return "TORPEDO NOT ARMED"
	case exhaustAttackTooFar:
		return "TARGET OUT OF RANGE"
	case exhaustAttackOffCenter:
		return "AIM OFF CENTER"
	case exhaustAttackWrongCourse:
		return "INVALID APPROACH"
	default:
		return ""
	}
}

// missionObjectiveTarget returns a point in the player's current simulation
// frame. The targeting computer remains independent of rendered tiles and LOD.
func (g *Game) missionObjectiveTarget() (math3d.Vec3, bool) {
	if g.world == nil || g.world.Mission.ID != yavinMissionID {
		return math3d.Vec3{}, false
	}
	player := g.objectByID(g.world.Mission.PlayerID)
	if player == nil {
		return math3d.Vec3{}, false
	}
	switch g.world.Mission.Phase {
	case sim.MissionOrbitalBattle, sim.MissionApproach:
		if g.world.Mission.HostID == 0 {
			return math3d.Vec3{}, false
		}
		pose, err := g.world.PoseInFrame(g.world.Mission.HostID, normalizedObjectFrame(*player))
		if err != nil {
			return math3d.Vec3{}, false
		}
		return pose.Position, true
	case sim.MissionSurfaceAssault:
		if g.deathStarSurfaceRuntime(normalizedObjectFrame(*player)) == nil {
			return math3d.Vec3{}, false
		}
		return environment.DeathStarTrenchGuidePoint(player.Pose.Position), true
	case sim.MissionTrenchRun:
		if g.deathStarSurfaceRuntime(normalizedObjectFrame(*player)) == nil {
			return math3d.Vec3{}, false
		}
		checkpoints := environment.DeathStarTrenchCheckpoints()
		if index := g.world.Mission.Progress.TrenchCheckpoint; index >= 0 && index < len(checkpoints) {
			return math3d.Vec3{Y: -2, Z: checkpoints[index]}, true
		}
		return environment.DeathStarExhaustPortPoint(), true
	case sim.MissionExhaustPortAttack:
		if g.deathStarSurfaceRuntime(normalizedObjectFrame(*player)) == nil {
			return math3d.Vec3{}, false
		}
		return environment.DeathStarExhaustPortPoint(), true
	default:
		return math3d.Vec3{}, false
	}
}

func objectiveArrowDirection(cameraPoint math3d.Vec3) (float64, float64) {
	forward := -cameraPoint.Z
	var x, y float64
	if forward > 1e-6 {
		x, y = cameraPoint.X/forward, -cameraPoint.Y/forward
	} else {
		x, y = cameraPoint.X, -cameraPoint.Y
		if math.Hypot(x, y) < 1e-6 {
			y = 1
		}
	}
	length := math.Hypot(x, y)
	if length < 0.02 {
		return 0, -1
	}
	return x / length, y / length
}

func (g *Game) drawSpeedIndicator(screen *ebiten.Image) {
	fighter := g.objectByID(fighterID)
	if fighter == nil || g.profile.Player.Flight.MaxForward <= 0 {
		return
	}
	mlgt := int(math.Round(fighter.Motion.Speed / g.profile.Player.Flight.MaxForward * 100))
	color := color.RGBA{R: 64, G: 180, B: 160, A: 220}
	drawVectorText(screen, 850, 505, fmt.Sprintf("SPD %03d MLGT", max(-99, min(999, mlgt))), color)
}

func (g *Game) drawTargetableIndicator(screen *ebiten.Image) {
	target, ok := g.aimedTarget()
	if !ok {
		return
	}
	center := target.Pose.Position
	if anchor, exists := target.Anchor("target"); exists {
		center = anchor.Position
	}
	projected, visible := g.pipeline.ProjectPoint(center)
	if !visible {
		return
	}
	radius := float32(14)
	if target.VisualRadius > 0 {
		cameraPoint := g.pipeline.View.TransformPoint(target.Pose.Position)
		if depth := -cameraPoint.Z; depth > g.pipeline.Near {
			radius = float32(min(42, max(14, target.VisualRadius/depth*g.pipeline.Projection[1][1]*float64(g.pipeline.Height)*0.5)))
		}
	}
	x, y := float32(projected.X), float32(projected.Y)
	c := color.RGBA{R: 255, G: 224, B: 32, A: 255}
	const arm = float32(7)
	for _, segment := range [][4]float32{{x - radius, y - radius, x - radius + arm, y - radius}, {x - radius, y - radius, x - radius, y - radius + arm}, {x + radius, y - radius, x + radius - arm, y - radius}, {x + radius, y - radius, x + radius, y - radius + arm}, {x - radius, y + radius, x - radius + arm, y + radius}, {x - radius, y + radius, x - radius, y + radius - arm}, {x + radius, y + radius, x + radius - arm, y + radius}, {x + radius, y + radius, x + radius, y + radius - arm}} {
		vector.StrokeLine(screen, segment[0], segment[1], segment[2], segment[3], 2, c, true)
	}
}

func (g *Game) aimedTarget() (scene.Object, bool) {
	x, y, _ := g.cockpitTarget()
	ray, ok := g.pipeline.ScreenRay(float64(x), float64(y))
	if !ok {
		return scene.Object{}, false
	}
	nearest := math.Inf(1)
	var selected scene.Object
	viewFrame := g.activeViewFrame()
	for _, object := range g.objects {
		if object.ID == fighterID || !object.Targetable || normalizedObjectFrame(object) != viewFrame {
			continue
		}
		radius := object.CollisionRadius
		if radius <= 0 {
			radius = object.VisualRadius
		}
		toCenter := object.Pose.Position.Sub(ray.Origin)
		along := toCenter.Dot(ray.Direction)
		if along <= 0 || along >= nearest {
			continue
		}
		closest := ray.Origin.Add(ray.Direction.Scale(along))
		if object.Pose.Position.Sub(closest).Length() > radius {
			continue
		}
		nearest, selected = along, object
	}
	for _, runtime := range g.environments {
		if runtime.bound.FrameID != viewFrame {
			continue
		}
		for _, tile := range runtime.tiles {
			for _, feature := range tile.Features {
				if !feature.Targetable || runtime.featureStates[feature.ID].Destroyed {
					continue
				}
				radius := 3.0
				for _, box := range feature.Boxes {
					radius = max(radius, box.HalfExtents.Length())
				}
				toCenter := feature.Pose.Position.Sub(ray.Origin)
				along := toCenter.Dot(ray.Direction)
				if along <= 0 || along >= nearest {
					continue
				}
				closest := ray.Origin.Add(ray.Direction.Scale(along))
				if feature.Pose.Position.Sub(closest).Length() > radius {
					continue
				}
				nearest = along
				selected = scene.Object{
					ID:              environmentFeatureObjectID(runtime.bound.HostID, feature.ID),
					Name:            feature.Kind,
					Definition:      runtime.bound.Definition.Name + "/" + feature.Kind,
					Frame:           runtime.bound.FrameID,
					Pose:            feature.Pose,
					CollisionRadius: radius,
					VisualRadius:    radius,
					Targetable:      true,
				}
			}
		}
	}
	return selected, selected.ID != 0
}

func environmentFeatureObjectID(hostID scene.ObjectID, featureID string) scene.ObjectID {
	const offset64 = uint64(14695981039346656037)
	const prime64 = uint64(1099511628211)
	hash := offset64 ^ uint64(hostID)
	for index := 0; index < len(featureID); index++ {
		hash ^= uint64(featureID[index])
		hash *= prime64
	}
	return scene.ObjectID(hash | 1<<63)
}

func (g *Game) drawShieldIndicator(screen *ebiten.Image) {
	const (
		topY       = float32(8)
		halfWidth  = float32(68)
		canopyDrop = float32(34)
	)
	cx := float32(ScreenWidth / 2)
	yellow := color.RGBA{R: 255, G: 224, B: 32, A: 255}
	segmentCount := g.profile.Player.Shield.Maximum
	activeSegments := max(0, min(segmentCount, g.shieldStrength))
	for side := 0; side < 2; side++ {
		for index := 0; index < segmentCount; index++ {
			if index < segmentCount-activeSegments {
				continue
			}
			// Square-root spacing fans the divisions outward: broad segments at
			// each outside edge taper into narrow segments near the center.
			fractionA := float32(math.Sqrt(float64(index) / float64(segmentCount)))
			fractionB := float32(math.Sqrt(float64(index+1) / float64(segmentCount)))
			var xA, xB float32
			if side == 0 {
				xA = cx - halfWidth + halfWidth*fractionA
				xB = cx - halfWidth + halfWidth*fractionB
			} else {
				xA = cx + halfWidth - halfWidth*fractionB
				xB = cx + halfWidth - halfWidth*fractionA
			}
			yA := topY + canopyDrop*(float32(math.Abs(float64(xA-cx)))/halfWidth)
			yB := topY + canopyDrop*(float32(math.Abs(float64(xB-cx)))/halfWidth)
			vector.StrokeLine(screen, xA, topY, xB, topY, 2, yellow, true)
			vector.StrokeLine(screen, xA, topY, xA, yA, 2, yellow, true)
			vector.StrokeLine(screen, xB, topY, xB, yB, 2, yellow, true)
			vector.StrokeLine(screen, xA, yA, xB, yB, 2, yellow, true)
		}
	}
	drawVectorShieldNumber(screen, cx, topY+canopyDrop-20, g.shieldStrength, yellow)
	drawVectorShieldWord(screen, cx, 52, yellow)
}

func drawVectorShieldWord(screen *ebiten.Image, centerX, topY float32, lineColor color.Color) {
	drawVectorText(screen, centerX, topY, "SHIELD", lineColor)
}

func drawVectorText(screen *ebiten.Image, centerX, topY float32, text string, lineColor color.Color) {
	var path vector.Path
	appendVectorTextPath(&path, centerX, topY, text)
	draw := &vector.DrawPathOptions{AntiAlias: true}
	draw.ColorScale.ScaleWithColor(lineColor)
	vector.StrokePath(screen, &path, &vector.StrokeOptions{Width: 2}, draw)
}

func appendVectorTextPath(path *vector.Path, centerX, topY float32, text string) {
	const glyphWidth, glyphGap, spaceWidth = float32(8), float32(3), float32(6)
	total := float32(0)
	for _, letter := range text {
		if letter == ' ' {
			total += spaceWidth + glyphGap
		} else {
			total += glyphWidth + glyphGap
		}
	}
	left := centerX - (total-glyphGap)/2
	for _, letter := range text {
		if letter == ' ' {
			left += spaceWidth + glyphGap
			continue
		}
		appendVectorGlyphPath(path, left, topY, letter)
		left += glyphWidth + glyphGap
	}
}

func drawVectorGlyph(screen *ebiten.Image, left, top float32, letter rune, lineColor color.Color) {
	var path vector.Path
	appendVectorGlyphPath(&path, left, top, letter)
	draw := &vector.DrawPathOptions{AntiAlias: true}
	draw.ColorScale.ScaleWithColor(lineColor)
	vector.StrokePath(screen, &path, &vector.StrokeOptions{Width: 2}, draw)
}

// Glyph topology is immutable. Keeping it in glyph-local coordinates avoids
// rebuilding a complete map and all of its slices for every character drawn.
var vectorGlyphSegments = map[rune][][4]float32{
	'S': {{0, 0, 8, 0}, {0, 0, 0, 5}, {0, 5, 8, 5}, {8, 5, 8, 10}, {0, 10, 8, 10}},
	'H': {{0, 0, 0, 10}, {8, 0, 8, 10}, {0, 5, 8, 5}},
	'I': {{0, 0, 8, 0}, {4, 0, 4, 10}, {0, 10, 8, 10}},
	'E': {{0, 0, 8, 0}, {0, 0, 0, 10}, {0, 5, 8, 5}, {0, 10, 8, 10}},
	'F': {{0, 0, 8, 0}, {0, 0, 0, 10}, {0, 5, 8, 5}},
	'L': {{0, 0, 0, 10}, {0, 10, 8, 10}},
	'D': {{0, 0, 6, 0}, {0, 0, 0, 10}, {6, 0, 8, 5}, {6, 5, 8, 10}, {0, 10, 6, 10}},
	'P': {{0, 0, 6, 0}, {0, 0, 0, 10}, {6, 0, 8, 5}, {0, 5, 6, 5}},
	'R': {{0, 0, 6, 0}, {0, 0, 0, 10}, {6, 0, 8, 5}, {0, 5, 6, 5}, {1, 5, 8, 10}},
	'T': {{0, 0, 8, 0}, {4, 0, 4, 10}},
	'O': {{0, 0, 8, 0}, {0, 0, 0, 10}, {8, 0, 8, 10}, {0, 10, 8, 10}},
	'Q': {{0, 0, 8, 0}, {0, 0, 0, 10}, {8, 0, 8, 10}, {0, 10, 8, 10}, {6, 8, 8, 10}},
	'A': {{0, 10, 0, 2}, {0, 2, 4, 0}, {4, 0, 8, 2}, {8, 2, 8, 10}, {0, 5, 8, 5}},
	'U': {{0, 0, 0, 10}, {8, 0, 8, 10}, {0, 10, 8, 10}},
	'Y': {{0, 0, 4, 5}, {8, 0, 4, 5}, {4, 5, 4, 10}},
	'N': {{0, 10, 0, 0}, {0, 0, 8, 10}, {8, 10, 8, 0}},
	'G': {{8, 0, 0, 0}, {0, 0, 0, 10}, {0, 10, 8, 10}, {8, 10, 8, 5}, {8, 5, 4, 5}},
	'M': {{0, 10, 0, 0}, {0, 0, 4, 5}, {4, 5, 8, 0}, {8, 0, 8, 10}},
	'V': {{0, 0, 4, 10}, {4, 10, 8, 0}},
	'W': {{0, 0, 2, 10}, {2, 10, 4, 5}, {4, 5, 6, 10}, {6, 10, 8, 0}},
	'K': {{0, 0, 0, 10}, {0, 5, 8, 0}, {0, 5, 8, 10}},
	'C': {{8, 0, 0, 0}, {0, 0, 0, 10}, {0, 10, 8, 10}},
	'B': {{0, 0, 0, 10}, {0, 0, 6, 0}, {6, 0, 6, 5}, {0, 5, 6, 5}, {6, 5, 6, 10}, {0, 10, 6, 10}},
	'X': {{0, 0, 8, 10}, {8, 0, 0, 10}},
	'.': {{4, 9, 4, 10}},
	'/': {{8, 0, 0, 10}},
	'+': {{4, 2, 4, 8}, {1, 5, 7, 5}},
	':': {{4, 2, 4, 3}, {4, 8, 4, 9}},
	'!': {{4, 0, 4, 7}, {4, 10, 4, 10}},
}

func appendVectorGlyphPath(path *vector.Path, left, top float32, letter rune) {
	if letter >= '0' && letter <= '9' {
		appendVectorShieldDigitPath(path, left+4, top-2, int(letter-'0'))
		return
	}
	for _, segment := range vectorGlyphSegments[letter] {
		path.MoveTo(left+segment[0], top+segment[1])
		path.LineTo(left+segment[2], top+segment[3])
	}
}

func appendVectorShieldDigitPath(path *vector.Path, centerX, topY float32, value int) {
	negative := value < 0
	if negative {
		value = -value
	}
	if value > 9 {
		value = 9
	}
	const width, height = float32(9), float32(14)
	left, right := centerX-width/2, centerX+width/2
	middle, bottom := topY+height/2, topY+height
	segments := [...][4]float32{
		{left + 1.5, topY, right - 1.5, topY},
		{right, topY + 1.5, right, middle - 1.5},
		{right, middle + 1.5, right, bottom - 1.5},
		{left + 1.5, bottom, right - 1.5, bottom},
		{left, middle + 1.5, left, bottom - 1.5},
		{left, topY + 1.5, left, middle - 1.5},
		{left + 1.5, middle, right - 1.5, middle},
	}
	digitSegments := [...]uint8{0x3f, 0x06, 0x5b, 0x4f, 0x66, 0x6d, 0x7d, 0x07, 0x7f, 0x6f}
	mask := digitSegments[value]
	if negative {
		mask = 0x40
	}
	for index, segment := range segments {
		if mask&(1<<index) == 0 {
			continue
		}
		path.MoveTo(segment[0], segment[1])
		path.LineTo(segment[2], segment[3])
	}
}

func drawVectorShieldNumber(screen *ebiten.Image, centerX, topY float32, value int, lineColor color.Color) {
	if value >= 10 {
		drawVectorShieldDigit(screen, centerX-6, topY, value/10, lineColor)
		drawVectorShieldDigit(screen, centerX+6, topY, value%10, lineColor)
		return
	}
	drawVectorShieldDigit(screen, centerX, topY, value, lineColor)
}

func drawVectorShieldDigit(screen *ebiten.Image, centerX, topY float32, value int, lineColor color.Color) {
	negative := value < 0
	if negative {
		value = -value
	}
	if value > 9 {
		value = 9
	}
	const (
		width  = float32(9)
		height = float32(14)
	)
	left := centerX - width/2
	right := centerX + width/2
	middle := topY + height/2
	bottom := topY + height
	segments := [...][4]float32{
		{left + 1.5, topY, right - 1.5, topY},
		{right, topY + 1.5, right, middle - 1.5},
		{right, middle + 1.5, right, bottom - 1.5},
		{left + 1.5, bottom, right - 1.5, bottom},
		{left, middle + 1.5, left, bottom - 1.5},
		{left, topY + 1.5, left, middle - 1.5},
		{left + 1.5, middle, right - 1.5, middle},
	}
	digitSegments := [...]uint8{0x3f, 0x06, 0x5b, 0x4f, 0x66, 0x6d, 0x7d, 0x07, 0x7f, 0x6f}
	mask := digitSegments[value]
	if negative {
		mask = 0x40
	}
	for index, segment := range segments {
		if mask&(1<<index) == 0 {
			continue
		}
		vector.StrokeLine(screen, segment[0], segment[1], segment[2], segment[3], 1.5, lineColor, true)
	}
}

type threatUrgency uint8

const (
	threatNone threatUrgency = iota
	threatBlue
	threatOrange
	threatRed
	threatFlashingRed
)

func (g *Game) drawThreatIndicator(screen *ebiten.Image) {
	for _, threat := range g.nearestCockpitThreats(8) {
		g.drawThreatMarker(screen, threat.object, threat.distance)
	}
}

type cockpitThreat struct {
	object   scene.Object
	distance float64
}

func (g *Game) drawThreatMarker(screen *ebiten.Image, threat scene.Object, distance float64) {
	urgency := cockpitThreatUrgency(distance)
	if urgency == threatFlashingRed && int(g.simulationTime*8)%2 == 0 {
		return
	}
	colors := [...]color.RGBA{
		{},
		{R: 48, G: 128, B: 255, A: 255},
		{R: 255, G: 160, B: 32, A: 255},
		{R: 255, G: 48, B: 32, A: 255},
		{R: 255, G: 24, B: 24, A: 255},
	}
	world := threat.Pose.Position
	cameraPoint := g.pipeline.View.TransformPoint(world)
	if cameraPoint.Z > 0 {
		cameraPoint.X = -cameraPoint.X
		cameraPoint.Y = -cameraPoint.Y
		cameraPoint.Z = -cameraPoint.Z
	}
	depth := max(0.1, -cameraPoint.Z)
	normalizedX := cameraPoint.X / depth * g.pipeline.Projection[0][0]
	normalizedY := cameraPoint.Y / depth * g.pipeline.Projection[1][1]
	centerX, centerY := float32(ScreenWidth/2), float32(ScreenHeight/2)
	dx, dy := float32(normalizedX), float32(-normalizedY)
	length := float32(math.Hypot(float64(dx), float64(dy)))
	if length < 0.001 {
		dy = -1
		length = 1
	}
	dx, dy = dx/length, dy/length
	const border = float32(18)
	maxX, maxY := float32(ScreenWidth/2)-border, float32(ScreenHeight/2)-border
	scale := min(maxX/max(float32(math.Abs(float64(dx))), 0.001), maxY/max(float32(math.Abs(float64(dy))), 0.001))
	edgeX, edgeY := centerX+dx*scale, centerY+dy*scale
	fromX, fromY := edgeX-dx*15, edgeY-dy*15
	drawCockpitArrow(screen, fromX, fromY, edgeX, edgeY, colors[urgency])
	vector.StrokeLine(screen, edgeX-dy*5, edgeY+dx*5, edgeX+dy*5, edgeY-dx*5, 2, colors[urgency], true)
}

func (g *Game) nearestCockpitThreats(limit int) []cockpitThreat {
	player := g.objectByID(fighterID)
	if player == nil || limit <= 0 {
		return nil
	}
	threats := make([]cockpitThreat, 0, limit)
	for _, object := range g.objects {
		if object.ID == fighterID || !sameFrame(*player, object) {
			continue
		}
		isThreat := object.Physical ||
			(object.CollisionRole == scene.CollisionProjectile && g.owners[object.ID] != fighterID)
		if !isThreat {
			continue
		}
		distance := max(0, object.Pose.Position.Sub(player.Pose.Position).Length()-object.CollisionRadius)
		threats = append(threats, cockpitThreat{object: object, distance: distance})
	}
	sort.Slice(threats, func(first, second int) bool {
		return threats[first].distance < threats[second].distance
	})
	if len(threats) > limit {
		threats = threats[:limit]
	}
	return threats
}

func cockpitThreatUrgency(distance float64) threatUrgency {
	switch {
	case distance <= 5:
		return threatFlashingRed
	case distance <= 10:
		return threatRed
	case distance <= 20:
		return threatOrange
	default:
		return threatBlue
	}
}

func (g *Game) cockpitTarget() (float32, float32, bool) {
	if g.mouseFlight {
		return ScreenWidth / 2, ScreenHeight / 2, true
	}
	x, y := ebiten.CursorPosition()
	return clampCockpitTarget(float32(x), float32(y), float32(g.profile.Targeting.AimRadius))
}

func clampCockpitTarget(x, y, aimRadius float32) (float32, float32, bool) {
	cx, cy := float32(ScreenWidth/2), float32(ScreenHeight/2)
	dx, dy := x-cx, y-cy
	distance := float32(math.Hypot(float64(dx), float64(dy)))
	if distance <= aimRadius || distance == 0 {
		return x, y, true
	}
	scale := float32(aimRadius) / distance
	return cx + dx*scale, cy + dy*scale, false
}

func (g *Game) cockpitAimTarget() (math3d.Vec3, bool) {
	if g.viewCamera.Mode != camera.Cockpit {
		return math3d.Vec3{}, false
	}
	x, y, _ := g.cockpitTarget()
	ray, ok := g.pipeline.ScreenRay(float64(x), float64(y))
	if !ok {
		return math3d.Vec3{}, false
	}
	return ray.Origin.Add(ray.Direction.Scale(g.profile.Targeting.AimConvergence)), true
}

func cockpitCannonMuzzleTop(x, y, targetX, targetY float32) [2]float32 {
	dx, dy := targetX-x, targetY-y
	length := float32(math.Hypot(float64(dx), float64(dy)))
	if length == 0 {
		return [2]float32{x, y}
	}
	forwardX, forwardY := dx/length, dy/length
	sideX, sideY := -forwardY, forwardX
	side := float32(3)
	if sideY > 0 {
		side = -side
	}
	return [2]float32{
		x + forwardX*28 + sideX*side,
		y + forwardY*28 + sideY*side,
	}
}

func drawCockpitCannon(screen *ebiten.Image, x, y, targetX, targetY float32, housingColor, barrelColor color.Color) {
	dx, dy := targetX-x, targetY-y
	length := float32(math.Hypot(float64(dx), float64(dy)))
	if length == 0 {
		return
	}
	forwardX, forwardY := dx/length, dy/length
	sideX, sideY := -forwardY, forwardX
	point := func(forward, side float32) [2]float32 {
		return [2]float32{
			x + forwardX*forward + sideX*side,
			y + forwardY*forward + sideY*side,
		}
	}

	// The near and far housing profiles form an open, concave red shroud.
	near := [...][2]float32{
		point(-18, -15),
		point(15, -11),
		point(7, -4),
		point(7, 4),
		point(15, 11),
		point(-18, 15),
		point(-8, 5),
		point(-8, -5),
	}
	localProfile := [...][2]float32{
		{-18, -15}, {15, -11}, {7, -4}, {7, 4},
		{15, 11}, {-18, 15}, {-8, 5}, {-8, -5},
	}
	var far [len(localProfile)][2]float32
	for index, local := range localProfile {
		far[index] = point(local[0]-7, local[1]+3)
	}
	drawClosedWireShape(screen, far[:], 2, color.RGBA{R: 128, G: 18, B: 28, A: 255})
	drawClosedWireShape(screen, near[:], 3, housingColor)
	for _, index := range []int{0, 1, 4, 5} {
		vector.StrokeLine(
			screen,
			near[index][0], near[index][1], far[index][0], far[index][1],
			2, housingColor, true,
		)
	}

	// Three converging rails make the emitter read as a barrel with depth.
	for _, offset := range []float32{-4, 0, 4} {
		outer := point(-25, offset*1.35)
		inner := point(28, offset*0.65)
		vector.StrokeLine(
			screen,
			outer[0], outer[1], inner[0], inner[1],
			3, barrelColor, true,
		)
	}
	emitterA := point(28, -3)
	emitterB := point(28, 3)
	vector.StrokeLine(screen, emitterA[0], emitterA[1], emitterB[0], emitterB[1], 2, barrelColor, true)
}

func drawClosedWireShape(screen *ebiten.Image, points [][2]float32, width float32, shapeColor color.Color) {
	for index := range points {
		next := (index + 1) % len(points)
		vector.StrokeLine(
			screen,
			points[index][0], points[index][1],
			points[next][0], points[next][1],
			width, shapeColor, true,
		)
	}
}

func drawCockpitArrow(screen *ebiten.Image, fromX, fromY, tipX, tipY float32, arrowColor color.Color) {
	var path vector.Path
	appendCockpitArrowPath(&path, fromX, fromY, tipX, tipY)
	draw := &vector.DrawPathOptions{AntiAlias: true}
	draw.ColorScale.ScaleWithColor(arrowColor)
	vector.StrokePath(screen, &path, &vector.StrokeOptions{Width: 2}, draw)
}

func appendCockpitArrowPath(path *vector.Path, fromX, fromY, tipX, tipY float32) {
	dx, dy := tipX-fromX, tipY-fromY
	length := float32(math.Hypot(float64(dx), float64(dy)))
	if length == 0 {
		return
	}
	unitX, unitY := dx/length, dy/length
	perpX, perpY := -unitY, unitX
	const (
		dartHalfWidth = float32(6)
		notchDepth    = float32(8)
	)
	points := [...][2]float32{
		{tipX, tipY},
		{fromX + perpX*dartHalfWidth, fromY + perpY*dartHalfWidth},
		{fromX + unitX*notchDepth, fromY + unitY*notchDepth},
		{fromX - perpX*dartHalfWidth, fromY - perpY*dartHalfWidth},
	}
	for index := range points {
		next := (index + 1) % len(points)
		path.MoveTo(points[index][0], points[index][1])
		path.LineTo(points[next][0], points[next][1])
	}
}

func (g *Game) toggleMouseFlight() {
	g.mouseFlight = !g.mouseFlight
	if g.mouseFlight {
		g.mode = modeManual
		g.mouseNeutralX, g.mouseNeutralY = ebiten.CursorPosition()
		ebiten.SetCursorMode(ebiten.CursorModeCaptured)
	} else {
		ebiten.SetCursorMode(ebiten.CursorModeVisible)
	}
}

func mouseFlightAxes(x, y, neutralX, neutralY int, config profile.InputConfig) (yaw, pitch float64) {
	yaw = applyMouseDeadzone(float64(x-neutralX)/(ScreenWidth/2), config)
	pitch = applyMouseDeadzone(float64(y-neutralY)/(ScreenHeight/2), config)
	return yaw, pitch
}

// cockpitSteeringAxes accounts for the cockpit camera's 180-degree yaw: its
// screen-right direction is the fighter's local -X direction. Vertical camera
// and fighter pitch directions already agree.
func cockpitSteeringAxes(x, y int, config profile.InputConfig) (yaw, pitch float64) {
	screenYaw, pitch := mouseFlightAxes(x, y, ScreenWidth/2, ScreenHeight/2, config)
	return -screenYaw, pitch
}

func applyMouseDeadzone(value float64, config profile.InputConfig) float64 {
	magnitude := math.Abs(value)
	deadzone := config.MouseDeadzone
	if magnitude <= deadzone {
		return 0
	}
	scaled := (magnitude - deadzone) / (1 - deadzone) * config.MouseSensitivity
	return math.Copysign(min(scaled, 1), value)
}

func keyAxis(negative, positive ebiten.Key) float64 {
	value := 0.0
	if ebiten.IsKeyPressed(negative) {
		value--
	}
	if ebiten.IsKeyPressed(positive) {
		value++
	}
	return value
}

func (g *Game) updateZoom() {
	zoomInput := keyAxis(ebiten.KeyMinus, ebiten.KeyEqual)
	_, wheelY := ebiten.Wheel()
	g.viewCamera.AdjustZoom(zoomInput*g.profile.Display.ZoomSpeed*g.profile.Simulation.TickSeconds + wheelY*0.2)
}

// rasterizePreparedDomain builds one isolated CPU occlusion surface from the
// already classified candidates in a single depth domain.
func (g *Game) rasterizePreparedDomain(prepared *preparedFrame, domain *preparedDepthDomain, depth *render.DepthBuffer) {
	for _, candidateIndex := range domain.writers {
		candidate := prepared.candidates[candidateIndex]
		g.pipeline.RasterizePreparedDepthOwned(candidate.geometry, depth, candidate.owner)
	}
}

func (g *Game) depthBufferForPrepared(prepared *preparedFrame) *render.DepthBuffer {
	if len(prepared.domains) == 0 {
		return nil
	}
	if g.depthBuffer == nil || g.depthBuffer.ViewWidth != g.pipeline.Width || g.depthBuffer.ViewHeight != g.pipeline.Height {
		// Hidden-line decisions do not need one depth sample per output pixel.
		// Half resolution quarters raster work while final surfaces and vectors
		// remain full resolution.
		g.depthBuffer = render.NewScaledDepthBuffer(g.pipeline.Width, g.pipeline.Height, 0.5)
	}
	return g.depthBuffer
}

func jobsForPreparedCandidates(prepared *preparedFrame, indexes []int, depth *render.DepthBuffer, jobs []worldRenderJob) []worldRenderJob {
	for _, candidateIndex := range indexes {
		candidate := prepared.candidates[candidateIndex]
		if candidate.isBillboard || candidate.analyticSphere {
			continue
		}
		jobDepth := (*render.DepthBuffer)(nil)
		if depth != nil && candidate.testsDepth {
			jobDepth = depth
		}
		jobs = append(jobs, worldRenderJob{geometry: candidate.geometry, depth: jobDepth, owner: candidate.owner, selfOcclusion: candidate.selfOcclusion, objectID: candidate.objectID, color: candidate.color, lineWidth: candidate.lineWidth, portalClip: candidate.portalClip})
	}
	return jobs
}

func (g *Game) Draw(screen *ebiten.Image) {
	screen.Fill(background)
	g.renderStats = render.Stats{}
	g.pipeline.FineStats = g.showHUD
	if g.showHUD {
		g.pipeline.Stats = &g.renderStats
	} else {
		g.pipeline.Stats = nil
	}
	g.refreshViewContext()
	if g.showcaseActive {
		prepared := g.prepareShowcaseFrame()
		g.showcaseStarPoints = g.drawBackground(screen, prepared, g.showcaseStarField, g.showcaseStarPoints)
		g.drawShowcase(screen, prepared)
		return
	}
	if g.drawApplicationShell(screen) {
		return
	}
	visibleObjects := 0
	prepared := g.prepareGameplayFrame()
	g.drawStarfield(screen, prepared)
	g.drawHyperspaceArrival(screen)
	g.beginWorldBatch()
	worldJobs := g.worldJobs[:0]
	if g.visibleObjectIDs == nil {
		g.visibleObjectIDs = make(map[scene.ObjectID]bool)
	}
	for objectID := range g.visibleObjectIDs {
		delete(g.visibleObjectIDs, objectID)
	}
	g.drawPreparedSurfaces(screen, prepared)
	geometryStart := time.Now()
	depth := g.depthBufferForPrepared(prepared)
	for _, candidate := range prepared.candidates {
		if candidate.isBillboard && g.drawBillboard(screen, candidate.object, candidate.billboard) {
			visibleObjects++
			if g.pipeline.Stats != nil {
				g.pipeline.Stats.ObjectsVisible++
			}
		}
	}
	worldJobs = jobsForPreparedCandidates(prepared, prepared.unscoped, nil, worldJobs[:0])
	g.renderStats.RenderJobs += len(worldJobs)
	for _, result := range g.renderWorldJobs(worldJobs) {
		addRenderStats(&g.renderStats, result.stats)
		g.queueWorldLines(result.lines, result.color, result.lineWidth)
		if result.objectID != 0 && len(result.lines) > 0 {
			g.visibleObjectIDs[result.objectID] = true
		}
	}
	for domainIndex := range prepared.domains {
		domain := &prepared.domains[domainIndex]
		worldJobs = worldJobs[:0]
		depthStart := time.Now()
		depth.Clear()
		g.rasterizePreparedDomain(prepared, domain, depth)
		if g.pipeline.Stats != nil {
			g.renderStats.DepthRasterMS += time.Since(depthStart).Seconds() * 1000
		}
		worldJobs = jobsForPreparedCandidates(prepared, domain.members, depth, worldJobs)
		g.renderStats.RenderJobs += len(worldJobs)
		for _, result := range g.renderWorldJobs(worldJobs) {
			addRenderStats(&g.renderStats, result.stats)
			g.queueWorldLines(result.lines, result.color, result.lineWidth)
			if result.objectID != 0 && len(result.lines) > 0 {
				g.visibleObjectIDs[result.objectID] = true
			}
		}
	}
	g.worldJobs = worldJobs
	for range g.visibleObjectIDs {
		visibleObjects++
		if g.pipeline.Stats != nil {
			g.pipeline.Stats.ObjectsVisible++
		}
	}
	g.renderStats.GeometryMS = time.Since(geometryStart).Seconds() * 1000
	vectorSubmitStart := time.Now()
	g.renderStats.WorldBatches = g.flushWorldBatch(screen)
	g.renderStats.VectorSubmitMS = time.Since(vectorSubmitStart).Seconds() * 1000
	g.visibleObjects = visibleObjects
	if g.viewCamera.Mode == camera.Cockpit {
		g.drawCockpitOverlay(screen)
	}
	if g.mouseFlight {
		g.drawMouseReticle(screen)
	}
	if g.playerDestroyed {
		drawVectorText(screen, float32(ScreenWidth/2), float32(ScreenHeight/2-120), "YOU FAILED!", color.RGBA{R: 255, G: 64, B: 64, A: 255})
		drawVectorText(screen, float32(ScreenWidth/2), float32(ScreenHeight/2-104), "PRESS R TO RESTART", color.RGBA{R: 255, G: 224, B: 32, A: 255})
	}
	if g.quitPrompt {
		drawVectorText(screen, float32(ScreenWidth/2), float32(ScreenHeight/2-24), "PAUSED", color.RGBA{R: 96, G: 220, B: 255, A: 255})
		drawVectorText(screen, float32(ScreenWidth/2), float32(ScreenHeight/2-8), "QUIT GAME? Y/N", color.RGBA{R: 255, G: 224, B: 32, A: 255})
	}
	if g.controlsVisible() {
		ebitenutil.DebugPrintAt(screen, controlsText(!g.playerDestroyed), 16, 16)
	}
	if g.showHUD {
		ebitenutil.DebugPrint(screen, g.hudText())
		g.drawRealismSlider(screen)
	}
}

// drawHyperspaceArrival adds a sparse vector streak treatment behind the
// fighter during orbital entry. It consumes the already projected/occluded
// star points, so it neither paints a background nor submits hidden stars.
func (g *Game) drawHyperspaceArrival(screen *ebiten.Image) {
	arrival := g.hyperspaceArrival
	if arrival == nil || len(g.starPoints) == 0 || arrival.duration <= 0 {
		return
	}
	progress := max(0, min(1, arrival.elapsed/arrival.duration))
	intensity := math.Sin(progress * math.Pi)
	if intensity <= 0 {
		return
	}
	centerX, centerY := float64(ScreenWidth)/2, float64(ScreenHeight)/2
	for _, point := range g.starPoints {
		dx, dy := point.X-centerX, point.Y-centerY
		distance := math.Hypot(dx, dy)
		if distance < 1 {
			continue
		}
		length := (8 + 54*intensity) * min(1, distance/220)
		startX := point.X - dx/distance*length
		startY := point.Y - dy/distance*length
		alpha := uint8(max(24, min(220, int(float64(point.Brightness)*intensity))))
		vector.StrokeLine(screen, float32(startX), float32(startY), float32(point.X), float32(point.Y), 1, color.RGBA{R: 160, G: 224, B: 255, A: alpha}, true)
	}
}

func (g *Game) drawShowcase(screen *ebiten.Image, prepared *preparedFrame) {
	g.drawPreparedSurfaces(screen, prepared)
	depth := g.depthBufferForPrepared(prepared)
	drawCandidates := func(indexes []int, domainDepth *render.DepthBuffer) {
		for _, candidateIndex := range indexes {
			candidate := prepared.candidates[candidateIndex]
			if candidate.isBillboard || candidate.analyticSphere {
				continue
			}
			var lines []render.Line
			candidateDepth := (*render.DepthBuffer)(nil)
			if domainDepth != nil && candidate.testsDepth {
				candidateDepth = domainDepth
			}
			lines = g.pipeline.RenderPrepared(candidate.geometry, candidateDepth, candidate.owner, candidate.selfOcclusion)
			for _, line := range lines {
				drawLine(screen, line, candidate.color, candidate.lineWidth)
			}
		}
	}
	drawCandidates(prepared.unscoped, nil)
	for domainIndex := range prepared.domains {
		domain := &prepared.domains[domainIndex]
		depth.Clear()
		g.rasterizePreparedDomain(prepared, domain, depth)
		drawCandidates(domain.members, depth)
	}
	title := "FIGHTER SHOWCASE"
	if len(g.showcaseObjects) > 0 {
		if spec, ok := catalog.SpecificationFor(g.showcaseObjects[g.showcaseSelected%len(g.showcaseObjects)].Definition); ok {
			title = spec.Title
		}
	}
	titleColor := color.RGBA{R: 80, G: 180, B: 255, A: 255}
	drawVectorText(screen, ScreenWidth/2, 18, title, titleColor)
	rotationStatus := "ROTATING"
	if !g.showcaseRotating {
		rotationStatus = "ROTATION PAUSED"
	}
	viewStatus := "NORMAL VIEW"
	if g.showcaseTopDown {
		viewStatus = "TOP VIEW"
	}
	drawVectorText(screen, ScreenWidth/2, 34, "LEFT RIGHT SELECT   SPACE "+rotationStatus+"   T "+viewStatus+"   WHEEL ZOOM", titleColor)
	g.drawShowcaseSpecs(screen)
	g.drawRealismSlider(screen)
}

func (g *Game) drawShowcaseSpecs(screen *ebiten.Image) {
	if len(g.showcaseObjects) == 0 {
		return
	}
	selected := g.showcaseObjects[g.showcaseSelected%len(g.showcaseObjects)]
	spec, ok := catalog.SpecificationFor(selected.Definition)
	if !ok {
		return
	}
	left, top, right, bottom := float32(ScreenWidth*0.32), float32(ScreenHeight-195), float32(ScreenWidth-18), float32(ScreenHeight-18)
	lineColor := color.RGBA{R: 96, G: 255, B: 128, A: 255}
	vector.StrokeLine(screen, left, top, right, top, 2, lineColor, true)
	vector.StrokeLine(screen, right, top, right, bottom, 2, lineColor, true)
	vector.StrokeLine(screen, right, bottom, left, bottom, 2, lineColor, true)
	vector.StrokeLine(screen, left, bottom, left, top, 2, lineColor, true)
	weapons := "WEAPONS " + spec.Weapons
	if spec.Ordnance != "NONE" {
		weapons += " " + spec.Ordnance
	}
	lines := []string{spec.Description, spec.Description2, "LENGTH: " + spec.Length, "CREW: " + spec.Crew, "PASSENGERS: " + spec.Passengers, "MAX SPEED: " + spec.MaxSpeed, "HYPERDRIVE: " + spec.Hyperdrive, "WEAPONS: " + weapons[len("WEAPONS "):], "SHIELDS: " + spec.Shields}
	for index, line := range lines {
		y := top + 20 + float32(index*15)
		if index < 2 {
			drawVectorText(screen, (left+right)/2, y, line, color.RGBA{R: 80, G: 180, B: 255, A: 255})
		} else {
			drawVectorTextWithNumericHighlight(screen, (left+right)/2, y, line)
		}
	}
}

func drawVectorTextWithNumericHighlight(screen *ebiten.Image, centerX, topY float32, text string) {
	const glyphWidth, glyphGap, spaceWidth = float32(8), float32(3), float32(6)
	total := float32(0)
	for _, character := range text {
		if character == ' ' {
			total += spaceWidth + glyphGap
		} else {
			total += glyphWidth + glyphGap
		}
	}
	left := centerX - (total-glyphGap)/2
	for _, character := range text {
		if character == ' ' {
			left += spaceWidth + glyphGap
			continue
		}
		textColor := color.RGBA{R: 190, G: 235, B: 255, A: 255}
		if character >= '0' && character <= '9' {
			textColor = color.RGBA{R: 64, G: 255, B: 128, A: 255}
		}
		if character == ':' {
			textColor = color.RGBA{R: 190, G: 235, B: 255, A: 255}
		}
		drawVectorGlyph(screen, left, topY, character, textColor)
		left += glyphWidth + glyphGap
	}
}

// objectInView rejects whole objects before any part geometry is transformed.
// The conservative projected sphere intentionally keeps intersecting objects
// alive when their center lies just outside the viewport.
func (g *Game) objectInView(object scene.Object) bool {
	if object.VisualRadius <= 0 {
		return true
	}
	cameraPoint := g.pipeline.View.TransformPoint(object.Pose.Position)
	depth := -cameraPoint.Z
	radius := object.VisualRadius
	if depth+radius < g.pipeline.Near {
		return false
	}
	if g.pipeline.Far > g.pipeline.Near && depth-radius > g.pipeline.Far {
		return false
	}
	if depth <= 0 {
		return true
	}
	projectedRadius := radius / depth * g.pipeline.Projection[1][1]
	return cameraPoint.X/depth*g.pipeline.Projection[0][0] >= -1-projectedRadius &&
		cameraPoint.X/depth*g.pipeline.Projection[0][0] <= 1+projectedRadius &&
		cameraPoint.Y/depth*g.pipeline.Projection[1][1] >= -1-projectedRadius &&
		cameraPoint.Y/depth*g.pipeline.Projection[1][1] <= 1+projectedRadius
}

// renderOwner gives each physical part of an object its own depth identity.
// This preserves same-part structural edges while allowing one part (such as
// a TIE foil) to occlude another part (such as its cockpit) within one object.
func renderOwner(objectID scene.ObjectID, partIndex int) uint64 {
	return (uint64(objectID) << 32) | uint64(partIndex+1)
}

// environmentPartOwner gives streamed surface geometry a stable depth owner.
// Without distinct owners, a trench's own coplanar floor and wall surfaces
// compete with their vector edges and can shimmer as the camera moves.
func environmentPartOwner(hostID scene.ObjectID, coordinate environment.TileCoordinate, featureID string, partIndex int) uint64 {
	const prime = uint64(1099511628211)
	hash := uint64(14695981039346656037) ^ uint64(hostID)
	mix := func(value uint64) {
		hash ^= value
		hash *= prime
	}
	mix(uint64(int64(coordinate.X)))
	mix(uint64(int64(coordinate.Z)))
	for index := 0; index < len(featureID); index++ {
		mix(uint64(featureID[index]))
	}
	mix(uint64(partIndex + 1))
	// Keep environment owners outside the ordinary object-owner range and
	// avoid zero, which deliberately means "include every depth sample".
	return hash | (uint64(1) << 63)
}

// meshInView applies the same conservative projected-sphere test to streamed
// environment modules. Their compiled model bounds provide a cheap hierarchy
// level between a whole surface frame and individual faces.
func (g *Game) meshInView(mesh modelpkg.Model, world math3d.Mat4) bool {
	prepared := modelpkg.Prepare(mesh)
	if prepared.Topology == nil || prepared.Topology.BoundsRadius <= 0 {
		return true
	}
	visible, _ := g.renderBoundsInView(environment.RenderBounds{
		Center: prepared.Topology.BoundsCenter,
		Radius: prepared.Topology.BoundsRadius,
	}, world)
	return visible
}

// renderBoundsInView performs the common aggregate sphere rejection and also
// reports projected pixel radius for LOD selection. The maximum transformed
// basis length keeps non-uniformly scaled feature prototypes conservative.
func (g *Game) renderBoundsInView(bounds environment.RenderBounds, world math3d.Mat4) (bool, float64) {
	if !bounds.Valid() {
		return true, 0
	}
	center := world.TransformPoint(bounds.Center)
	scale := max(
		world.TransformDirection(math3d.Vec3{X: 1}).Length(),
		world.TransformDirection(math3d.Vec3{Y: 1}).Length(),
		world.TransformDirection(math3d.Vec3{Z: 1}).Length(),
	)
	if scale <= 0 {
		scale = 1
	}
	radius := bounds.Radius * scale
	cameraPoint := g.pipeline.View.TransformPoint(center)
	depth := -cameraPoint.Z
	if depth+radius < g.pipeline.Near {
		return false, 0
	}
	if g.pipeline.Far > g.pipeline.Near && depth-radius > g.pipeline.Far {
		return false, 0
	}
	if depth <= 0 {
		return true, math.Hypot(float64(g.pipeline.Width), float64(g.pipeline.Height))
	}
	projectedRadius := radius / depth * g.pipeline.Projection[1][1]
	visible := cameraPoint.X/depth*g.pipeline.Projection[0][0] >= -1-projectedRadius &&
		cameraPoint.X/depth*g.pipeline.Projection[0][0] <= 1+projectedRadius &&
		cameraPoint.Y/depth*g.pipeline.Projection[1][1] >= -1-projectedRadius &&
		cameraPoint.Y/depth*g.pipeline.Projection[1][1] <= 1+projectedRadius
	return visible, projectedRadius * float64(g.pipeline.Height) * 0.5
}

func (g *Game) drawBillboard(screen *ebiten.Image, object scene.Object, billboard appearance.Billboard) bool {
	// The object's centre may leave the viewport while its projected silhouette
	// is still visible at an edge. Keep the unclipped centre here; individual
	// artwork lines are clipped by drawBillboardLines below.
	center, visible := g.pipeline.ProjectPointUnclipped(object.Pose.Position)
	if !visible || object.VisualRadius <= 0 || center.Depth <= g.pipeline.Near {
		return false
	}
	projectedRadius := object.VisualRadius / center.Depth * g.pipeline.Projection[1][1] * float64(g.pipeline.Height) * 0.5
	if projectedRadius <= 0 {
		return false
	}
	// The same normalized artwork is used at every distance. Detail reveal is
	// monotonic with projected size and therefore stable while approaching.
	reveal := (projectedRadius - 55) / 260
	lines := g.billboardLines(billboard, reveal, object.Definition)
	if g.pipeline.Stats != nil {
		g.pipeline.Stats.BillboardObjects++
		g.pipeline.Stats.BillboardLines += len(lines)
	}
	batchCount := g.drawBillboardLines(screen, lines, center.X, center.Y, projectedRadius)
	if g.pipeline.Stats != nil {
		g.pipeline.Stats.BillboardBatches += batchCount
	}
	return true
}

// billboardLines returns a cached immutable line set for the current detail
// tier. Projected size changes continuously, but the revealed artwork changes
// only when a detail threshold is crossed.
func (g *Game) billboardLines(billboard appearance.Billboard, reveal float64, fallbackKey string) []appearance.Line {
	level := 0
	for _, detail := range billboard.Details {
		if detail.Threshold <= reveal {
			level++
		}
	}
	key := billboard.Name
	if key == "" {
		key = fallbackKey
	}
	levels := g.billboardLineCache[key]
	if levels == nil {
		levels = make(map[int][]appearance.Line)
		g.billboardLineCache[key] = levels
	}
	if lines, ok := levels[level]; ok {
		return lines
	}
	lines := make([]appearance.Line, 0, len(billboard.Base)+level)
	lines = append(lines, billboard.Base...)
	for _, detail := range billboard.Details {
		if detail.Threshold <= reveal {
			lines = append(lines, detail.Line)
		}
	}
	levels[level] = lines
	return lines
}

type vectorLineBatch struct {
	color color.RGBA
	width float32
	path  vector.Path
}

// drawBillboardLines batches independent lines by colour and width, reducing
// the number of vector draw submissions without joining unrelated strokes.
func (g *Game) drawBillboardLines(screen *ebiten.Image, lines []appearance.Line, centerX, centerY, radius float64) int {
	for index := range g.billboardBatches {
		g.billboardBatches[index].path.Reset()
	}
	batches := g.billboardBatches[:0]
	for _, line := range lines {
		candidate := render.Line{
			X1: centerX + line.A.X*radius, Y1: centerY - line.A.Y*radius,
			X2: centerX + line.B.X*radius, Y2: centerY - line.B.Y*radius,
		}
		clipped, visible := render.ClipLineToViewport(candidate, float64(screen.Bounds().Dx()), float64(screen.Bounds().Dy()))
		if !visible {
			continue
		}
		style := vectorLineBatch{color: line.Color, width: line.Width}
		batchIndex := -1
		for index := range batches {
			if batches[index].color == style.color && batches[index].width == style.width {
				batchIndex = index
				break
			}
		}
		if batchIndex < 0 {
			batches = append(batches, style)
			batchIndex = len(batches) - 1
		}
		batch := &batches[batchIndex]
		batch.path.MoveTo(float32(clipped.X1), float32(clipped.Y1))
		batch.path.LineTo(float32(clipped.X2), float32(clipped.Y2))
	}
	g.billboardBatches = batches
	return drawVectorLineBatches(screen, batches)
}

func drawVectorLineBatches(screen *ebiten.Image, batches []vectorLineBatch) int {
	for index := range batches {
		batch := &batches[index]
		stroke := &vector.StrokeOptions{Width: batch.width}
		draw := &vector.DrawPathOptions{AntiAlias: true}
		draw.ColorScale.ScaleWithColor(batch.color)
		vector.StrokePath(screen, &batch.path, stroke, draw)
	}
	return len(batches)
}

func (g *Game) beginWorldBatch() {
	for index := range g.worldBatches {
		g.worldBatches[index].path.Reset()
	}
	g.worldBatches = g.worldBatches[:0]
}

func appendVectorBatchLine(batches *[]vectorLineBatch, line render.Line, lineColor color.Color, lineWidth float32) {
	rgba, ok := lineColor.(color.RGBA)
	if !ok {
		red, green, blue, alpha := lineColor.RGBA()
		rgba = color.RGBA{R: uint8(red >> 8), G: uint8(green >> 8), B: uint8(blue >> 8), A: uint8(alpha >> 8)}
	}
	batchIndex := -1
	for index := range *batches {
		batch := &(*batches)[index]
		if batch.color == rgba && batch.width == lineWidth {
			batchIndex = index
			break
		}
	}
	if batchIndex < 0 {
		*batches = append(*batches, vectorLineBatch{color: rgba, width: lineWidth})
		batchIndex = len(*batches) - 1
	}
	batch := &(*batches)[batchIndex]
	batch.path.MoveTo(float32(line.X1), float32(line.Y1))
	batch.path.LineTo(float32(line.X2), float32(line.Y2))
}

func (g *Game) queueWorldLines(lines []render.Line, lineColor color.Color, lineWidth float32) {
	for _, line := range lines {
		appendVectorBatchLine(&g.worldBatches, line, lineColor, lineWidth)
	}
}

func (g *Game) flushWorldBatch(screen *ebiten.Image) int {
	return drawVectorLineBatches(screen, g.worldBatches)
}

// drawPreparedSurfaces submits only explicitly opted-in fills. The opaque-only
// path uses the same bounded batches as before; translucent fills join the
// stable far-to-near painter order only when present. Prepared triangles have
// already been back-face-classified and near/far clipped.
func (g *Game) drawPreparedSurfaces(screen *ebiten.Image, prepared *preparedFrame) {
	g.surfaceTriangles = g.surfaceTriangles[:0]
	for index := range prepared.candidates {
		candidate := &prepared.candidates[index]
		if !candidate.surface.Filled() || candidate.geometry == nil {
			continue
		}
		before := len(g.surfaceTriangles)
		if candidate.surface.Translucent() {
			g.surfaceTriangles = render.AppendTranslucentTriangles(g.surfaceTriangles, candidate.geometry, candidate.surface.Color, candidate.surface.TextureID)
		} else {
			g.surfaceTriangles = render.AppendOpaqueTriangles(g.surfaceTriangles, candidate.geometry, candidate.surface.Color, candidate.surface.TextureID)
		}
		if len(g.surfaceTriangles) > before && g.pipeline.Stats != nil {
			if candidate.surface.Translucent() {
				g.pipeline.Stats.TranslucentSurfaceCandidates++
			} else {
				g.pipeline.Stats.OpaqueSurfaceCandidates++
			}
		}
	}
	if len(g.surfaceTriangles) == 0 {
		return
	}
	render.SortFlatTriangles(g.surfaceTriangles)
	g.ensureWhitePixel()

	const maxTrianglesPerBatch = (1<<16 - 1) / 3
	for first := 0; first < len(g.surfaceTriangles); {
		textureID := g.surfaceTriangles[first].TextureID
		translucent := g.surfaceTriangles[first].Translucent
		end := first + 1
		for end < len(g.surfaceTriangles) && end-first < maxTrianglesPerBatch && g.surfaceTriangles[end].TextureID == textureID && g.surfaceTriangles[end].Translucent == translucent {
			end++
		}
		var batchStart time.Time
		if g.pipeline.Stats != nil {
			batchStart = time.Now()
		}
		source, textureWidth, textureHeight := g.surfaceSource(textureID)
		count := end - first
		g.surfaceVertices = resizeEbitenVertices(g.surfaceVertices, count*3)
		g.surfaceIndices = resizeUint16(g.surfaceIndices, count*3)
		for local, triangle := range g.surfaceTriangles[first:end] {
			vertex := local * 3
			red := float32(triangle.Color.R) / 255
			green := float32(triangle.Color.G) / 255
			blue := float32(triangle.Color.B) / 255
			alpha := float32(triangle.Color.A) / 255
			points := [...]render.Point{triangle.A, triangle.B, triangle.C}
			for corner, point := range points {
				sourceX, sourceY := float32(0.5), float32(0.5)
				if textureID != "" && source != g.whitePixel {
					sourceX = float32(triangle.UVs[corner].U) * textureWidth
					sourceY = float32(triangle.UVs[corner].V) * textureHeight
				}
				g.surfaceVertices[vertex+corner] = ebiten.Vertex{
					DstX: float32(point.X), DstY: float32(point.Y), SrcX: sourceX, SrcY: sourceY,
					ColorR: red, ColorG: green, ColorB: blue, ColorA: alpha,
				}
				g.surfaceIndices[vertex+corner] = uint16(vertex + corner)
			}
		}
		options := &ebiten.DrawTrianglesOptions{Filter: ebiten.FilterNearest}
		if textureID != "" && source != g.whitePixel {
			options.Address = ebiten.AddressRepeat
		}
		screen.DrawTriangles(g.surfaceVertices, g.surfaceIndices, source, options)
		if g.pipeline.Stats != nil {
			if translucent {
				g.pipeline.Stats.TranslucentBatches++
				g.pipeline.Stats.TranslucentTriangles += count
				g.pipeline.Stats.TranslucentSubmitMS += time.Since(batchStart).Seconds() * 1000
			} else {
				g.pipeline.Stats.OpaqueBatches++
				g.pipeline.Stats.OpaqueTriangles += count
				g.pipeline.Stats.OpaqueSubmitMS += time.Since(batchStart).Seconds() * 1000
			}
			if textureID != "" && !translucent {
				g.pipeline.Stats.TexturedBatches++
				g.pipeline.Stats.TexturedTriangles += count
			}
		}
		first = end
	}
}

// RegisterSurfaceTexture admits contributor-defined surface textures without
// exposing Ebitengine images to model, scene, or environment packages.
func (g *Game) RegisterSurfaceTexture(id string, texture render.Texture) error {
	if g.textureRegistry == nil {
		g.textureRegistry = render.NewTextureRegistry()
	}
	return g.textureRegistry.Register(id, texture)
}

func (g *Game) surfaceSource(id string) (*ebiten.Image, float32, float32) {
	if id == "" {
		return g.whitePixel, 1, 1
	}
	if image := g.textureImages[id]; image != nil {
		width, height := image.Size()
		return image, float32(width), float32(height)
	}
	texture, ok := g.textureRegistry.Lookup(id)
	if !ok {
		// Invalid external materials remain visible as their flat tint instead
		// of silently disappearing from the scene.
		return g.whitePixel, 1, 1
	}
	image := ebiten.NewImage(texture.Width, texture.Height)
	image.WritePixels(texture.Pixels)
	g.textureImages[id] = image
	return image, float32(texture.Width), float32(texture.Height)
}

func resizeEbitenVertices(vertices []ebiten.Vertex, length int) []ebiten.Vertex {
	if cap(vertices) < length {
		return make([]ebiten.Vertex, length)
	}
	return vertices[:length]
}

func resizeUint16(indices []uint16, length int) []uint16 {
	if cap(indices) < length {
		return make([]uint16, length)
	}
	return indices[:length]
}

func (g *Game) ensureWhitePixel() {
	if g.whitePixel != nil {
		return
	}
	g.whitePixel = ebiten.NewImage(1, 1)
	g.whitePixel.WritePixels([]byte{255, 255, 255, 255})
}

// renderWorldJobs parallelizes pure CPU line generation after depth has been
// rasterized. Workers only read the shared depth buffer; all Ebiten submission
// remains ordered on the calling/render thread.
func (g *Game) renderWorldJobs(jobs []worldRenderJob) []worldRenderResult {
	results := make([]worldRenderResult, len(jobs))
	renderOne := func(index int) {
		job := jobs[index]
		pipeline := g.pipeline
		if g.pipeline.Stats != nil {
			pipeline.Stats = &results[index].stats
		}
		results[index].lines = pipeline.RenderPrepared(job.geometry, job.depth, job.owner, job.selfOcclusion)
		if len(job.portalClip) > 0 {
			kept := results[index].lines[:0]
			for _, line := range results[index].lines {
				if clipped, ok := render.ClipLineToConvexPolygon(line, job.portalClip); ok {
					kept = append(kept, clipped)
				}
			}
			results[index].lines = kept
		}
		results[index].objectID = job.objectID
		results[index].color = job.color
		results[index].lineWidth = job.lineWidth
	}
	if len(jobs) < 8 || runtime.GOMAXPROCS(0) < 2 {
		for index := range jobs {
			renderOne(index)
		}
		return results
	}
	workers := min(runtime.GOMAXPROCS(0), len(jobs))
	var next int64
	var wait sync.WaitGroup
	wait.Add(workers)
	for range workers {
		go func() {
			defer wait.Done()
			for {
				index := int(atomic.AddInt64(&next, 1) - 1)
				if index >= len(jobs) {
					return
				}
				renderOne(index)
			}
		}()
	}
	wait.Wait()
	return results
}

func addRenderStats(total *render.Stats, part render.Stats) {
	total.GeometryPreparations += part.GeometryPreparations
	total.FacesClassified += part.FacesClassified
	total.PreparedTriangles += part.PreparedTriangles
	total.InputVertices += part.InputVertices
	total.TransformedVertices += part.TransformedVertices
	total.InputEdges += part.InputEdges
	total.OutputEdges += part.OutputEdges
	total.TinyEdges += part.TinyEdges
	total.InputFaces += part.InputFaces
	total.BackfaceRejected += part.BackfaceRejected
	total.PolicyRejected += part.PolicyRejected
	total.DepthRejected += part.DepthRejected
	total.LineDepthSamples += part.LineDepthSamples
	total.ClippedEdges += part.ClippedEdges
}

func (g *Game) activeViewFrame() scene.FrameID {
	if g.playerDestroyed && g.viewCamera.Mode == camera.Fixed {
		return normalizedObjectFrame(g.destructionVictim)
	}
	if target := g.objectByID(g.viewCamera.TargetID); target != nil &&
		(g.viewCamera.Mode != camera.Fixed || g.destructionViewRemaining > 0) {
		return normalizedObjectFrame(*target)
	}
	return scene.ExteriorFrame
}

func (g *Game) defaultBackground() view.Background {
	if g.profile.Starfield.Mode == profile.StarfieldModeWorld {
		return view.Background{Kind: view.BackgroundWorldStars}
	}
	return view.Background{Kind: view.BackgroundSkyfield}
}

// refreshViewContext is the single boundary between camera/environment state
// and frame preparation. Showcase uses its presentation camera; gameplay uses
// the controlled camera and lets a registered room override the default space
// background for its frame.
func (g *Game) refreshViewContext() {
	context := view.Context{
		FrameID:    g.activeViewFrame(),
		ViewMatrix: g.viewCamera.View(g.objects),
		Background: g.defaultBackground(),
	}
	if g.showcaseActive {
		context.FrameID = scene.ExteriorFrame
		context.ViewMatrix = math3d.Identity()
	} else if roomContext, ok := g.environmentRegistry.ViewContext(context.FrameID, context.ViewMatrix); ok {
		context = roomContext
	}
	g.viewContext = context
	g.pipeline.View = context.ViewMatrix
}

func (g *Game) objectDetailTier(object scene.Object) scene.DetailTier {
	thresholds := object.DetailThresholds
	if object.VisualRadius <= 0 || thresholds.MediumPixels <= 0 || thresholds.NearPixels <= 0 {
		return scene.DetailNear
	}
	cameraPoint := g.pipeline.View.TransformPoint(object.Pose.Position)
	depth := -cameraPoint.Z
	if depth <= g.pipeline.Near {
		return scene.DetailNear
	}
	projectedRadius := object.VisualRadius / depth * g.pipeline.Projection[1][1] * float64(g.pipeline.Height) * 0.5
	current := g.detailLevels[object.ID]
	current = detailTierForProjectedRadius(current, thresholds, projectedRadius)
	g.detailLevels[object.ID] = current
	return current
}

func (g *Game) environmentFeatureDetailTier(runtime *localEnvironment, featureID string, projectedRadius float64) scene.DetailTier {
	thresholds := runtime.bound.Definition.DetailThresholds
	if thresholds.MediumPixels <= 0 || thresholds.NearPixels <= 0 {
		return scene.DetailNear
	}
	if runtime.detailLevels == nil {
		runtime.detailLevels = make(map[string]scene.DetailTier)
	}
	current := detailTierForProjectedRadius(runtime.detailLevels[featureID], thresholds, projectedRadius)
	runtime.detailLevels[featureID] = current
	return current
}

func (g *Game) environmentTileDetailTier(runtime *localEnvironment, coordinate environment.TileCoordinate, projectedRadius float64) scene.DetailTier {
	thresholds := runtime.bound.Definition.DetailThresholds
	if thresholds.MediumPixels <= 0 || thresholds.NearPixels <= 0 {
		return scene.DetailNear
	}
	if runtime.tileDetailLevels == nil {
		runtime.tileDetailLevels = make(map[environment.TileCoordinate]scene.DetailTier)
	}
	current := detailTierForProjectedRadius(runtime.tileDetailLevels[coordinate], thresholds, projectedRadius)
	runtime.tileDetailLevels[coordinate] = current
	return current
}

func detailTierForProjectedRadius(current scene.DetailTier, thresholds scene.DetailThresholds, projectedRadius float64) scene.DetailTier {
	const leaveRatio = 0.88
	switch current {
	case scene.DetailNear:
		if projectedRadius >= thresholds.NearPixels*leaveRatio {
			return scene.DetailNear
		}
		current = scene.DetailMedium
	case scene.DetailMedium:
		if projectedRadius >= thresholds.NearPixels {
			current = scene.DetailNear
		} else if projectedRadius < thresholds.MediumPixels*leaveRatio {
			current = scene.DetailPrimary
		}
	default:
		if projectedRadius >= thresholds.MediumPixels {
			current = scene.DetailMedium
		}
	}
	return current
}

func (g *Game) setRealismLevel(level int) {
	if level < 0 {
		level = 0
	}
	if level >= len(renderingProfiles) {
		level = len(renderingProfiles) - 1
	}
	g.realismLevel = level
	g.pipeline.Stages = render.StagesForProfile(renderingProfiles[level])
	g.pipeline.MinLinePixels = realismLineThreshold(level)
}

func realismLineThreshold(level int) float64 {
	switch level {
	case 0:
		return 0.55
	case 1:
		return 0.35
	case 2:
		return 0.20
	case 3:
		return 0.10
	default:
		return 0.05
	}
}

func (g *Game) drawRealismSlider(screen *ebiten.Image) {
	x, y, width := float32(24), float32(ScreenHeight-28), float32(220)
	lineColor := color.RGBA{R: 64, G: 224, B: 255, A: 255}
	vector.StrokeLine(screen, x, y, x+width, y, 2, lineColor, true)
	for i := range renderingProfiles {
		px := x + width*float32(i)/float32(len(renderingProfiles)-1)
		vector.StrokeLine(screen, px, y-6, px, y+6, 2, lineColor, true)
	}
	px := x + width*float32(g.realismLevel)/float32(len(renderingProfiles)-1)
	vector.DrawFilledCircle(screen, px, y, 6, lineColor, true)
	drawVectorText(screen, x+width/2, y-14, fmt.Sprintf("REALISM: %s", realismProfileLabel(g.realismLevel)), lineColor)
}

func realismProfileLabel(level int) string {
	labels := []string{"ARCADE", "CULLED", "ENHANCED", "HIDDEN LINE", "MAXIMUM"}
	if level < 0 || level >= len(labels) {
		return "ARCADE"
	}
	return labels[level]
}

func (g *Game) handleRealismSliderClick() bool {
	if !inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
		return false
	}
	mouseX, mouseY := ebiten.CursorPosition()
	const sliderY = ScreenHeight - 28
	if mouseY < sliderY-14 || mouseY > sliderY+14 || mouseX < 24 || mouseX > 244 {
		return false
	}
	level := int(math.Round(float64(mouseX-24) / 220 * float64(len(renderingProfiles)-1)))
	g.setRealismLevel(level)
	return true
}

func controlsText(playerActive bool) string {
	text := "CONTROLS\n" +
		"W/S  throttle    Arrows  steer\n" +
		"Q/E  roll        Mouse  aim\n" +
		"Right mouse button  steer fighter\n" +
		"F / left mouse  fire    G  mouse flight\n" +
		"T  proton torpedo\n" +
		"V  view   Shift  follow fighter   Space  HUD\n" +
		"L  toggle surface auto-level\n" +
		"C  fighter showcase\n" +
		"[ / ]  rendering realism\n" +
		"?  show / hide controls\n\n"
	if playerActive {
		return text + "R  RESTART MISSION"
	}
	return text + "PRESS R TO RESPAWN"
}

func questionKeyJustPressed() bool {
	shift := ebiten.IsKeyPressed(ebiten.KeyShiftLeft) || ebiten.IsKeyPressed(ebiten.KeyShiftRight)
	return shift && inpututil.IsKeyJustPressed(ebiten.KeySlash)
}

func (g *Game) controlsVisible() bool {
	return g.controlsPinned || g.controlsRemaining > 0
}

func (g *Game) updateControls(seconds float64) {
	if seconds > 0 && g.controlsRemaining > 0 && !g.controlsPinned {
		g.controlsRemaining = max(0, g.controlsRemaining-seconds)
	}
}

func (g *Game) toggleControls() {
	if g.controlsVisible() {
		g.controlsPinned = false
		g.controlsRemaining = 0
		return
	}
	g.controlsPinned = true
}

func (g *Game) drawStarfield(screen *ebiten.Image, prepared *preparedFrame) {
	g.starPoints = g.drawBackground(screen, prepared, g.starField, g.starPoints)
}

// drawBackground consumes only the prepared frame's generic background policy.
// Rooms, Death Stars, and other environment types never enter this path.
func (g *Game) drawBackground(screen *ebiten.Image, prepared *preparedFrame, field *starfield.Field, points []starfield.Point) []starfield.Point {
	if field == nil || prepared.view.Background.Kind == view.BackgroundNone {
		return points[:0]
	}
	switch prepared.view.Background.Kind {
	case view.BackgroundWorldStars:
		field.SetMode(starfield.ModeWorld, 0)
	case view.BackgroundSkyfield:
		field.SetMode(starfield.ModeSkyfield, g.profile.Display.FarPlane*0.85)
	default:
		return points[:0]
	}
	g.buildStarOccluders(prepared)
	points = field.ProjectInto(g.pipeline, points, &g.starOccluders)
	g.drawStarPoints(screen, points)
	return points
}

// buildStarOccluders prepares only sparse screen-space tests. Analytic
// occluders describe large line-art bodies such as the Death Star; ordinary
// surface models contribute their already-authored, camera-facing triangles.
// Neither path paints or clears the framebuffer.
func (g *Game) buildStarOccluders(prepared *preparedFrame) {
	g.starOccluders.Reset()
	g.starOccluders.Stats = g.pipeline.Stats
	for _, candidate := range prepared.candidates {
		if candidate.analyticSphere {
			g.addSphereStarOccluder(candidate.object)
			continue
		}
		if !candidate.pointOccluder {
			continue
		}
		g.starOccluders.AddPreparedGeometry(candidate.geometry)
	}
	if g.pipeline.Stats != nil {
		for _, occluder := range g.starOccluders.Items {
			switch occluder.OccluderKind() {
			case render.OccluderAnalytic:
				g.pipeline.Stats.ActiveAnalyticOccluders++
			case render.OccluderGeometry:
				g.pipeline.Stats.ActiveGeometryOccluders++
			}
		}
	}
}

func (g *Game) addSphereStarOccluder(object scene.Object) {
	if object.VisualRadius <= 0 {
		return
	}
	cameraPoint := g.pipeline.View.TransformPoint(object.Pose.Position)
	depth := -cameraPoint.Z
	radius := object.VisualRadius
	if depth+radius <= g.pipeline.Near || (g.pipeline.Far > g.pipeline.Near && depth-radius > g.pipeline.Far) {
		return
	}
	center, visible := g.pipeline.ProjectPointUnclipped(object.Pose.Position)
	if !visible {
		// If the camera is inside the sphere, use a conservative screen-wide
		// circle rather than attempting a perspective projection behind the
		// near plane.
		if depth+radius <= g.pipeline.Near || depth > g.pipeline.Near {
			return
		}
		center = render.Point{X: float64(g.pipeline.Width) / 2, Y: float64(g.pipeline.Height) / 2, Depth: g.pipeline.Near}
		depth = g.pipeline.Near
	}
	projectedRadius := radius / math.Max(depth, g.pipeline.Near) * g.pipeline.Projection[1][1] * float64(g.pipeline.Height) * 0.5
	if depth <= g.pipeline.Near {
		projectedRadius = math.Hypot(float64(g.pipeline.Width), float64(g.pipeline.Height)) * 2
	}
	g.starOccluders.Add(render.CircleOccluder{CenterX: center.X, CenterY: center.Y, Radius: projectedRadius, Depth: depth, SphereRadius: radius})
}

func (g *Game) drawStarPoints(screen *ebiten.Image, points []starfield.Point) {
	if len(points) == 0 {
		return
	}
	g.ensureWhitePixel()
	g.starVertices = g.starVertices[:0]
	g.starIndices = g.starIndices[:0]
	if cap(g.starVertices) < len(points)*4 {
		g.starVertices = make([]ebiten.Vertex, 0, len(points)*4)
	}
	if cap(g.starIndices) < len(points)*6 {
		g.starIndices = make([]uint16, 0, len(points)*6)
	}
	for _, point := range points {
		half := point.Size
		x, y := float32(point.X), float32(point.Y)
		brightness := float32(point.Brightness) / 255
		base := uint16(len(g.starVertices))
		g.starVertices = append(g.starVertices,
			ebiten.Vertex{DstX: x - half, DstY: y - half, SrcX: 0.5, SrcY: 0.5, ColorR: brightness, ColorG: brightness, ColorB: brightness, ColorA: 1},
			ebiten.Vertex{DstX: x + half, DstY: y - half, SrcX: 0.5, SrcY: 0.5, ColorR: brightness, ColorG: brightness, ColorB: brightness, ColorA: 1},
			ebiten.Vertex{DstX: x + half, DstY: y + half, SrcX: 0.5, SrcY: 0.5, ColorR: brightness, ColorG: brightness, ColorB: brightness, ColorA: 1},
			ebiten.Vertex{DstX: x - half, DstY: y + half, SrcX: 0.5, SrcY: 0.5, ColorR: brightness, ColorG: brightness, ColorB: brightness, ColorA: 1},
		)
		g.starIndices = append(g.starIndices, base, base+1, base+2, base, base+2, base+3)
	}
	op := &ebiten.DrawTrianglesOptions{Filter: ebiten.FilterNearest}
	screen.DrawTriangles(g.starVertices, g.starIndices, g.whitePixel, op)
}

func (g *Game) drawMouseReticle(screen *ebiten.Image) {
	x, y := ebiten.CursorPosition()
	x = min(max(x, 8), ScreenWidth-8)
	y = min(max(y, 8), ScreenHeight-8)
	reticleColor := color.RGBA{R: 64, G: 224, B: 255, A: 255}
	vector.StrokeLine(screen, float32(x-8), float32(y), float32(x+8), float32(y), 1, reticleColor, true)
	vector.StrokeLine(screen, float32(x), float32(y-8), float32(x), float32(y+8), 1, reticleColor, true)
}

func (g *Game) hudText() string {
	motion := kinematics.Motion{}
	if fighter := g.objectByID(fighterID); fighter != nil {
		motion = fighter.Motion
	}
	status := "Running"
	if g.paused {
		status = "Paused"
	}
	if g.playerDestroyed {
		status = "DESTROYED - press R to respawn"
	}
	mouseStatus := "Off"
	if g.mouseFlight {
		mouseStatus = "On"
	}
	pointerMode := "Target"
	if g.viewCamera.Mode == camera.Cockpit && ebiten.IsMouseButtonPressed(ebiten.MouseButtonRight) {
		pointerMode = "Steer"
	}
	autoLevelStatus := "Off"
	if g.surfaceAutoLevel {
		autoLevelStatus = "On"
	}
	objective := sim.MissionInactive.String()
	if g.world != nil {
		objective = g.world.Mission.Phase.String()
		if g.world.Mission.Phase == sim.MissionFailed && g.world.Mission.Reason != "" {
			objective += " (" + g.world.Mission.Reason + ")"
		}
	}
	return fmt.Sprintf(
		"Objective: %s | Torpedoes: %d\nProfile: %s | Mode: %s | %s | View: %s | Pointer: %s | Captured: %s | Auto-level: %s | Tempo: %.1fx | Bolts: %d\nSpeed: %+0.2f  Yaw: %+0.2f  Pitch: %+0.2f  Roll: %+0.2f\n"+
			"Swarm: %d active, %d returning | Objects: %d total, %d visible | Shield: %d/%d | Kills: %d | Collisions: %d\n"+
			"Render objects: %d in, %d visible, %d culled | Vertices: %d input, %d transformed\n"+
			"Faces: %d input | Edges: %d input, %d output | Rejected: backface %d, policy %d, depth %d, tiny %d\n"+
			"Clipped edges: %d | Vectors: %d jobs, %d world batches\n"+
			"Prepared: %d candidates, %d geometries, %d faces classified, %d surface triangles | Depth: %d candidate objects, %d candidate parts, %d writing, %d testing, %d domains | enabled profile %t self %t\n"+
			"Depth work: %d faces, %d triangles, %d pixels tested, %d written, %d line samples\n"+
			"Opaque surfaces: %d candidates, %d triangles, %d batches | textured %d triangles, %d batches | glass %d candidates, %d triangles, %d batches\n"+
			"Environment: %d/%d tiles active/input, %d tile bounds rejected | features %d in, %d prepared, rejected %d bounds, %d LOD | parts rejected %d bounds, %d LOD\n"+
			"Billboards: %d objects, %d lines, %d batches | Star occlusion: %d analytic, %d geometry | Stars: %d considered, %d rejected, %d submitted\n"+
			"Timing ms: depth %.2f | geometry %.2f | opaque submit %.2f | glass submit %.2f | vector submit %.2f\n"+
			"Profile: %s\n"+
			"W/S throttle  Mouse/arrows yaw/pitch  Q/E roll  Space stop\nF/left-click fire  G mouse  M mode  V view  P pause  R reset  +/- or wheel zoom",
		objective,
		g.torpedoesRemaining,
		g.profile.Name,
		g.mode,
		status,
		g.viewCamera.Mode,
		pointerMode,
		mouseStatus,
		autoLevelStatus,
		g.profile.Simulation.MotionScale,
		len(g.projectiles),
		motion.Speed,
		motion.YawRate,
		motion.PitchRate,
		motion.RollRate,
		len(g.controllers),
		len(g.respawns),
		len(g.objects),
		g.visibleObjects,
		g.shieldStrength,
		g.profile.Player.Shield.Maximum,
		g.kills,
		g.collisions,
		g.renderStats.ObjectsInput,
		g.renderStats.ObjectsVisible,
		g.renderStats.ObjectsCulled,
		g.renderStats.InputVertices,
		g.renderStats.TransformedVertices,
		g.renderStats.InputFaces,
		g.renderStats.InputEdges,
		g.renderStats.OutputEdges,
		g.renderStats.BackfaceRejected,
		g.renderStats.PolicyRejected,
		g.renderStats.DepthRejected,
		g.renderStats.TinyEdges,
		g.renderStats.ClippedEdges,
		g.renderStats.RenderJobs,
		g.renderStats.WorldBatches,
		g.renderStats.CandidatesPrepared,
		g.renderStats.GeometryPreparations,
		g.renderStats.FacesClassified,
		g.renderStats.PreparedTriangles,
		g.renderStats.DepthCandidateObjects,
		g.renderStats.DepthCandidateParts,
		g.renderStats.DepthWritingCandidates,
		g.renderStats.DepthTestingCandidates,
		g.renderStats.ActiveDepthDomains,
		g.renderStats.DepthEnabledByProfile,
		g.renderStats.DepthEnabledBySelfOcclusion,
		g.renderStats.DepthFacesSubmitted,
		g.renderStats.DepthTrianglesRasterized,
		g.renderStats.DepthPixelsTested,
		g.renderStats.DepthPixelsWritten,
		g.renderStats.LineDepthSamples,
		g.renderStats.OpaqueSurfaceCandidates,
		g.renderStats.OpaqueTriangles,
		g.renderStats.OpaqueBatches,
		g.renderStats.TexturedTriangles,
		g.renderStats.TexturedBatches,
		g.renderStats.TranslucentSurfaceCandidates,
		g.renderStats.TranslucentTriangles,
		g.renderStats.TranslucentBatches,
		g.renderStats.ActiveEnvironmentTiles,
		g.renderStats.EnvironmentTilesInput,
		g.renderStats.EnvironmentTilesBoundsRejected,
		g.renderStats.EnvironmentFeaturesInput,
		g.renderStats.EnvironmentInstancesPrepared,
		g.renderStats.EnvironmentFeaturesBoundsRejected,
		g.renderStats.EnvironmentFeaturesLODRejected,
		g.renderStats.EnvironmentPartsBoundsRejected,
		g.renderStats.EnvironmentPartsLODRejected,
		g.renderStats.BillboardObjects,
		g.renderStats.BillboardLines,
		g.renderStats.BillboardBatches,
		g.renderStats.ActiveAnalyticOccluders,
		g.renderStats.ActiveGeometryOccluders,
		g.renderStats.StarsConsidered,
		g.renderStats.StarsAnalyticRejected+g.renderStats.StarsGeometryRejected,
		g.renderStats.StarsSubmitted,
		g.renderStats.DepthRasterMS,
		g.renderStats.GeometryMS,
		g.renderStats.OpaqueSubmitMS,
		g.renderStats.TranslucentSubmitMS,
		g.renderStats.VectorSubmitMS,
		g.profile.Name,
	)
}

func drawLine(screen *ebiten.Image, line render.Line, lineColor color.Color, lineWidth float32) {
	vector.StrokeLine(
		screen,
		float32(line.X1), float32(line.Y1),
		float32(line.X2), float32(line.Y2),
		lineWidth, lineColor, true,
	)
}

func (g *Game) Layout(_, _ int) (int, int) {
	return ScreenWidth, ScreenHeight
}
