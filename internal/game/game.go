package game

import (
	"fmt"
	"image/color"
	"math"
	"sort"

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

type autonomousRespawn struct {
	readyAt float64
}

type destructionTransient struct {
	remaining      float64
	rootObjectID   scene.ObjectID
	componentIndex int
	stage          scene.DestructionStage
}

type localEnvironment struct {
	bound     environment.Bound
	tiles     map[environment.TileCoordinate]environment.Tile
	destroyed map[string]bool
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
}

const (
	modeAutopilot flightMode = iota
	modeManual
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
	environments             []localEnvironment
	transitions              map[scene.ObjectID]environmentTransition
	transitionCommitments    map[scene.ObjectID]bool
	objects                  []scene.Object
	pipeline                 render.Pipeline
	initialPose              kinematics.Pose
	autoMotion               kinematics.Motion
	mode                     flightMode
	paused                   bool
	quitPrompt               bool
	started                  bool
	swarmLaunched            bool
	showHUD                  bool
	showcaseActive            bool
	showcaseTime              float64
	showcaseObjects           []scene.Object
	showcaseStarField         *starfield.Field
	showcaseDistance          float64
	showcaseSelected          int
	showcaseSlide             float64
	showcasePreviousMode      camera.Mode
	viewCamera               *camera.Camera
	nextObjectID             scene.ObjectID
	projectiles              map[scene.ObjectID]float64
	owners                   map[scene.ObjectID]scene.ObjectID
	fireCooldown             float64
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
	debris                   map[scene.ObjectID]destructionTransient
	environmentContacts      map[scene.ObjectID]float64
	respawns                 []autonomousRespawn
	respawnSequence          uint64
	playerDestroyed          bool
	playerViewMode           camera.Mode
	kills                    int
	collisions               int
	visibleObjects           int
	world                    *sim.World
	detailLevels             map[scene.ObjectID]scene.DetailTier
	shieldStrength           int
	shieldQuietTime          float64
	destructionViewRemaining float64
	controlsRemaining        float64
	controlsPinned           bool
	realismLevel             int
}

var renderingProfiles = []string{"builtin/arcade", "builtin/culled", "builtin/hidden-line", "builtin/depth-cue"}

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
	objects := []scene.Object{fighter}
	controllers := make(map[scene.ObjectID]control.Strategy, gameProfile.Swarm.Count)
	for index, pose := range autonomousFighterPoses(gameProfile.Swarm.InitialPositions) {
		id := scene.ObjectID(index + 2)
		autonomous, err := catalogRegistry.Create(gameProfile.Swarm.Object, id, pose)
		if err != nil {
			return nil, fmt.Errorf("create swarm object: %w", err)
		}
		autonomous.Motion.Speed = gameProfile.Swarm.InitialSpeed + float64(index)*gameProfile.Swarm.SpeedStep
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
	game := &Game{
		profile:             gameProfile,
		controllerRegistry:  registry,
		catalogRegistry:     catalogRegistry,
		appearanceRegistry:  appearanceRegistry,
		cockpitRegistry:     cockpit.DefaultRegistry(),
		environmentRegistry: environmentRegistry,
		pipeline:            render.NewPipeline(ScreenWidth, ScreenHeight, gameProfile.Display.VerticalFOV, gameProfile.Display.NearPlane, gameProfile.Display.FarPlane),
		objects:             objects,
		initialPose:         initialPose,
		autoMotion:          autoMotion,
		viewCamera:          viewCamera,
		nextObjectID:        nextObjectID,
		projectiles:         make(map[scene.ObjectID]float64),
		owners:              make(map[scene.ObjectID]scene.ObjectID),
		starField:           starfield.New(gameProfile.Starfield.Count, gameProfile.Starfield.Seed, gameProfile.Starfield.Radius, initialPose.Position),
		controllers:         controllers,
		debris:              make(map[scene.ObjectID]destructionTransient),
		environmentContacts: make(map[scene.ObjectID]float64),
		transitions:         make(map[scene.ObjectID]environmentTransition),
		transitionCommitments: make(map[scene.ObjectID]bool),
		respawnSequence:     uint64(gameProfile.Swarm.Count),
		shieldStrength:      gameProfile.Player.Shield.Maximum,
		started:             false,
		swarmLaunched:       true,
		showHUD:             false,
		controlsRemaining:   gameProfile.Display.ControlsDisplayDuration,
		detailLevels:        make(map[scene.ObjectID]scene.DetailTier),
	}
	game.pipeline.Stages = render.StagesForProfile(gameProfile.Display.RenderingProfile)
	game.showcaseObjects = game.createShowcaseObjects()
	game.showcaseStarField = starfield.New(500, gameProfile.Starfield.Seed+101, 90, math3d.Vec3{})
	game.showcaseDistance = 16
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
	game.pipeline.View = game.viewCamera.View(game.objects)
	return game, nil
}

func (g *Game) createShowcaseObjects() []scene.Object {
	objects := make([]scene.Object, 0, 2)
	for index, definition := range []string{catalog.XWingName, catalog.TIEFighterName} {
		object, err := g.catalogRegistry.Create(definition, scene.ObjectID(900000+index), kinematics.Pose{
			Position: math3d.Vec3{X: float64(index*2-1) * 5.5, Z: -40},
		})
		if err == nil {
			scale := 1.65
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
			bound:     bound,
			tiles:     make(map[environment.TileCoordinate]environment.Tile),
			destroyed: make(map[string]bool),
		}
		g.environments = append(g.environments, runtime)
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
		g.pipeline.View = g.viewCamera.View(g.objects)
		return nil
	}
	if !g.showcaseActive && inpututil.IsKeyJustPressed(ebiten.KeyEscape) {
		g.paused = true
		g.quitPrompt = true
		g.pipeline.View = g.viewCamera.View(g.objects)
		return nil
	}
	if g.showcaseActive {
		g.handleRealismSliderClick()
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
		if !g.paused {
			g.showcaseTime += seconds
			_, wheelY := ebiten.Wheel()
			g.showcaseDistance = max(16, min(90, g.showcaseDistance-wheelY*2.5))
			mouseX, mouseY := ebiten.CursorPosition()
			yaw := (float64(mouseX)-ScreenWidth/2) / (ScreenWidth/2) * 0.65
			pitch := (float64(mouseY)-ScreenHeight/2) / (ScreenHeight/2) * 0.35
			angle := g.showcaseTime * 0.7
			for index := range g.showcaseObjects {
				offset := float64(index-g.showcaseSelected) + g.showcaseSlide
				if len(g.showcaseObjects) == 2 {
					if offset > 1 { offset -= float64(len(g.showcaseObjects)) }
					if offset < -1 { offset += float64(len(g.showcaseObjects)) }
				}
				g.showcaseObjects[index].Pose.Position = math3d.Vec3{X: offset * 28, Z: -g.showcaseDistance - math.Abs(offset)*18}
				g.showcaseObjects[index].Pose.Orientation = math3d.QuaternionFromYawPitchRoll(angle+offset*0.25+yaw, 0.12+pitch, 0)
			}
		}
		g.pipeline.View = math3d.Identity()
		return nil
	}
	g.updateControls(seconds)
	if !g.started {
		if inpututil.IsKeyJustPressed(ebiten.KeyS) ||
			inpututil.IsKeyJustPressed(ebiten.KeyF) ||
			inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
			g.started = true
			g.controlsRemaining = 0
			g.controlsPinned = false
		}
		g.pipeline.View = g.viewCamera.View(g.objects)
		return nil
	}
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
		}
		g.pipeline.View = g.viewCamera.View(g.objects)
		return nil
	}

	g.updateZoom()
	g.laserBeamTime = max(0, g.laserBeamTime-seconds)
	if g.mode == modeManual {
		if fighter := g.objectByID(fighterID); fighter != nil {
			fighter.Motion = control.Apply(fighter.Motion, g.readIntent(), g.profile.Player.Flight, seconds)
		}
	}
	if g.paused {
		g.pipeline.View = g.viewCamera.View(g.objects)
		return nil
	}
	g.simulationTime += seconds
	g.updateShield(seconds)
	g.updateEnvironmentContacts(seconds)
	if g.destructionViewRemaining > 0 {
		g.destructionViewRemaining -= seconds
		g.viewCamera.PullBack(3.0 * seconds)
		if g.destructionViewRemaining <= 0 {
			g.destructionViewRemaining = 0
			g.switchToRandomSwarmFollowView()
		}
	}
	g.updateRespawns()
	g.updateAutonomous(seconds)
	g.fireCooldown = max(0, g.fireCooldown-seconds)
	if ebiten.IsKeyPressed(ebiten.KeyF) || ebiten.IsMouseButtonPressed(ebiten.MouseButtonLeft) {
		if g.showHUD && g.handleRealismSliderClick() {
			g.pipeline.View = g.viewCamera.View(g.objects)
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
	if err := g.updateEnvironmentTransitions(); err != nil {
		return err
	}
	g.refreshEnvironmentTiles()
	g.updateDebris(seconds)
	g.resolveLaserCollisions(previousPositions)
	g.resolveSolidCollisions(previousPositions)
	g.resolveEnvironmentCollisions(previousPositions)
	g.updateProjectiles(seconds)
	if reference := g.starfieldReferencePosition(); reference != nil {
		g.starField.Wrap(*reference)
	}
	g.viewCamera.Update(seconds)
	g.pipeline.View = g.viewCamera.View(g.objects)
	return nil
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
				inApproach := object.ID == fighterID && transition.Trigger.Contains(pose.Position)
				// Pursuers inherit the target's transition. This lets a swarm
				// fighter follow the player into a local surface frame even when
				// it is still outside the ordinary approach volume.
				if !inApproach && object.ID != fighterID && g.pursuesTarget(object.ID) {
					target := g.objectByID(fighterID)
					if target != nil && normalizedObjectFrame(*target) == source {
						if targetTransition, exists := g.transitions[fighterID]; exists {
							inApproach = targetTransition.destination == destination
						} else {
							targetPose, targetErr := g.world.PoseInFrame(fighterID, runtime.bound.FrameID)
							inApproach = targetErr == nil && transition.Trigger.Contains(targetPose.Position)
						}
					}
				}
				if !inApproach && g.transitionCommitments[object.ID] {
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
					if err := g.completeEnvironmentTransition(environmentTransition{objectID: object.ID, destination: destination, anchor: transition.Name, motion: object.Motion}, transition.EntryPose); err != nil {
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
	object.Pose = finalLocalPose
	object.Motion = transition.motion
	if framePose, err := g.world.FramePose(transition.destination); err == nil {
		object.Motion.Velocity = framePose.Orientation.Conjugate().Rotate(transition.worldVelocity)
	}
	if transition.objectID == g.viewCamera.TargetID {
		g.viewCamera.Mode = transition.previousMode
	}
	g.objects = g.world.Objects
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
			center := environment.TileCoordinate{
				X: int(math.Floor(object.Pose.Position.X/size + 0.5)),
				Z: int(math.Floor(object.Pose.Position.Z/size + 0.5)),
			}
			for offsetX := -radius; offsetX <= radius; offsetX++ {
				for offsetZ := -radius; offsetZ <= radius; offsetZ++ {
					desired[environment.TileCoordinate{X: center.X + offsetX, Z: center.Z + offsetZ}] = true
				}
			}
		}
		tiles := make(map[environment.TileCoordinate]environment.Tile, len(desired))
		for coordinate := range desired {
			if tile, exists := runtime.tiles[coordinate]; exists {
				tiles[coordinate] = tile
			} else {
				tiles[coordinate] = runtime.bound.Definition.Tile(coordinate)
			}
		}
		runtime.tiles = tiles
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
	g.viewCamera.TargetID = ids[index]
	g.viewCamera.Mode = camera.Chase
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
	g.destroyAndDisintegrate(destroyed, remove)
}

func (g *Game) resolveSolidCollisions(previous map[scene.ObjectID]math3d.Vec3) {
	destroyed := make(map[scene.ObjectID]scene.Object)
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
					}
				} else if first.Destructible {
					destroyed[first.ID] = first
				}
				if second.ID == fighterID {
					if g.applyShieldDamage(g.profile.Player.Shield.CollisionDamage) {
						destroyed[second.ID] = second
					}
				} else if second.Destructible {
					destroyed[second.ID] = second
				}
				break
			}
		}
	}
	if len(destroyed) > 0 {
		g.destroyAndDisintegrate(destroyed, nil)
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
				if runtime.destroyed[string(box.FeatureID)] {
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
			if g.environmentFeatureIsHittable(runtime, string(nearest.FeatureID)) {
				runtime.destroyed[string(nearest.FeatureID)] = true
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

func (g *Game) environmentFeatureIsHittable(runtime *localEnvironment, id string) bool {
	if runtime == nil || id == "" {
		return false
	}
	for _, tile := range runtime.tiles {
		for _, feature := range tile.Features {
			if feature.ID == id {
				return feature.Hittable
			}
		}
	}
	return false
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
			g.controlsRemaining = g.profile.Display.ControlsDisplayDuration
			g.controlsPinned = false
			g.playerViewMode = g.viewCamera.Mode
			g.viewCamera.Mode = camera.Fixed
			if g.mouseFlight {
				g.mouseFlight = false
				ebiten.SetCursorMode(ebiten.CursorModeVisible)
			}
		}
		if _, autonomous := g.controllers[id]; autonomous {
			g.respawns = append(g.respawns, autonomousRespawn{readyAt: g.simulationTime + g.profile.Swarm.RespawnDelay})
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
				g.viewCamera.Mode = camera.Orbit
				g.viewCamera.AdjustZoom(-2)
			}
		case scene.DestructionComponent:
			g.spawnPolygonDisintegration(object, transients[id])
		}
	}
}

func (g *Game) spawnDisintegration(object scene.Object) {
	inheritedVelocity := object.Pose.Forward().Scale(object.Motion.Speed).Add(object.Motion.Velocity)
	localDirections := [...]math3d.Vec3{
		{X: -1, Y: 0.35, Z: -0.2},
		{X: 0.05, Y: 1, Z: 0.3},
		{X: 1, Y: -0.3, Z: 0.15},
	}
	for index, localDirection := range localDirections {
		// Preserve the parent trajectory while adding a deterministic blast
		// perturbation. The original motion remains dominant; the random spread
		// prevents every destruction from producing the same symmetric fan.
		seed := uint64(object.ID)*0x9e3779b97f4a7c15 + uint64(index+1)*0x517cc1b727220a95
		localDirection = localDirection.Add(math3d.Vec3{
			X: deterministicSigned(&seed) * 0.45,
			Y: deterministicSigned(&seed) * 0.35,
			Z: deterministicSigned(&seed) * 0.45,
		}).Normalize()
		direction := object.Pose.Orientation.Rotate(localDirection).Normalize()
		pose := object.Pose
		pose.Position = pose.Position.Add(direction.Scale(0.08))
		fragment, err := g.catalogRegistry.CreateFragment(object.Definition, g.nextObjectID, index, pose)
		if err != nil {
			continue
		}
		g.nextObjectID++
		spinSign := 1.0
		if (uint64(object.ID)+uint64(index))%2 == 0 {
			spinSign = -1
		}
		fragment.Motion = kinematics.Motion{
			Velocity:  inheritedVelocity.Scale(0.82).Add(direction.Scale(1.35 + 0.28*float64(index))),
			YawRate:   spinSign * (1.25 + 0.30*float64(index)),
			PitchRate: -spinSign * (1.05 + 0.22*float64(index)),
			RollRate:  spinSign * (2.0 + 0.40*float64(index)),
		}
		fragment.Frame = object.Frame
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
		centroid := modelCentroid(shard.Parts[0].Mesh.Verts)
		localDirection := centroid.Normalize()
		if localDirection == (math3d.Vec3{}) {
			angle := 2 * math.Pi * float64(polygonIndex+1) / float64(max(1, polygonCount))
			localDirection = math3d.Vec3{X: math.Cos(angle), Y: math.Sin(angle), Z: 0.35}.Normalize()
		}
		direction := component.Pose.Orientation.Rotate(localDirection).Normalize()
		shard.Pose.Position = shard.Pose.Position.Add(direction.Scale(0.04))
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
		g.objects = append(g.objects, shard)
		g.debris[shard.ID] = destructionTransient{
			remaining:      g.profile.Simulation.DisintegrationTime,
			rootObjectID:   transient.rootObjectID,
			componentIndex: transient.componentIndex,
			stage:          scene.DestructionPolygon,
		}
	}
}

func modelCentroid(vertices []math3d.Vec3) math3d.Vec3 {
	if len(vertices) == 0 {
		return math3d.Vec3{}
	}
	centroid := math3d.Vec3{}
	for _, vertex := range vertices {
		centroid = centroid.Add(vertex)
	}
	return centroid.Scale(1 / float64(len(vertices)))
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
			delete(g.environmentContacts, object.ID)
			continue
		}
		kept = append(kept, object)
	}
	g.objects = kept
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
	for range g.respawns {
		g.spawnAutonomousFighter()
	}
	g.respawns = g.respawns[:0]
}

func (g *Game) spawnAutonomousFighter() {
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
	fighter, err := g.catalogRegistry.Create(g.profile.Swarm.Object, id, pose)
	if err != nil {
		return
	}
	fighter.Motion.Speed = g.profile.Swarm.InitialSpeed + g.profile.Swarm.SpeedStep*float64(g.respawnSequence%uint64(max(1, g.profile.Swarm.Count)))
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

func (g *Game) updateAutonomous(seconds float64) {
	target := g.objectByID(fighterID)
	var targetSnapshot scene.Object
	if target != nil {
		targetSnapshot = *target
	}
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
		controllerTarget := targetSnapshot
		if target == nil || !sameFrame(*object, targetSnapshot) {
			controllerTarget = scene.Object{}
		}
		context := control.Context{
			Self:        *object,
			Target:      controllerTarget,
			Nearby:      nearby,
			Seconds:     seconds,
			MotionScale: g.profile.Simulation.MotionScale,
		}
		decision := controller.Decide(context)
		object.Motion = control.ApplyWithLimits(object.Motion, decision.Flight, g.profile.Swarm.Flight, seconds)
		if controllerTarget.ID != 0 {
			if decision.Fire {
				g.fireAutonomousLaser(*object, targetSnapshot)
			}
		}
	}
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
	fighter.Pose = g.initialPose
	g.controlsRemaining = 0
	g.controlsPinned = false
	g.destructionViewRemaining = 0
	g.shieldStrength = g.profile.Player.Shield.Maximum
	g.shieldQuietTime = 0
	g.fireCooldown = 0
	g.fireHistory = g.fireHistory[:0]
	g.starField.Wrap(g.initialPose.Position)
	if g.mode == modeAutopilot {
		fighter.Motion = g.autoMotion
	} else {
		fighter.Motion = kinematics.Motion{}
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
	g.playerDestroyed = false
	g.controlsRemaining = 0
	g.controlsPinned = false
	g.destructionViewRemaining = 0
	g.shieldStrength = g.profile.Player.Shield.Maximum
	g.shieldQuietTime = 0
	g.fireCooldown = 0
	g.fireHistory = g.fireHistory[:0]
	g.starField.Wrap(pose.Position)
	g.viewCamera.Mode = g.playerViewMode
	g.viewCamera.TargetID = fighterID
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
		if candidate, ok := g.cockpitRegistry.ForDefinition(target.Definition); ok { layout, exists = candidate, true }
	}
	if !exists { layout = cockpit.Fallback() }
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
				if !feature.Targetable || runtime.destroyed[feature.ID] {
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
		drawVectorGlyph(screen, left, topY, letter, lineColor)
		left += glyphWidth + glyphGap
	}
}

func drawVectorGlyph(screen *ebiten.Image, left, top float32, letter rune, lineColor color.Color) {
	if letter >= '0' && letter <= '9' {
		drawVectorShieldDigit(screen, left+4, top-2, int(letter-'0'), lineColor)
		return
	}
	right, middle, bottom := left+8, top+5, top+10
	segments := map[rune][][4]float32{
		'S': {{left, top, right, top}, {left, top, left, middle}, {left, middle, right, middle}, {right, middle, right, bottom}, {left, bottom, right, bottom}},
		'H': {{left, top, left, bottom}, {right, top, right, bottom}, {left, middle, right, middle}},
		'I': {{left, top, right, top}, {left + 4, top, left + 4, bottom}, {left, bottom, right, bottom}},
		'E': {{left, top, right, top}, {left, top, left, bottom}, {left, middle, right, middle}, {left, bottom, right, bottom}},
		'F': {{left, top, right, top}, {left, top, left, bottom}, {left, middle, right, middle}},
		'L': {{left, top, left, bottom}, {left, bottom, right, bottom}},
		'D': {{left, top, right - 2, top}, {left, top, left, bottom}, {right - 2, top, right, middle}, {right - 2, middle, right, bottom}, {left, bottom, right - 2, bottom}},
		'P': {{left, top, right - 2, top}, {left, top, left, bottom}, {right - 2, top, right, middle}, {left, middle, right - 2, middle}},
		'R': {{left, top, right - 2, top}, {left, top, left, bottom}, {right - 2, top, right, middle}, {left, middle, right - 2, middle}, {left + 1, middle, right, bottom}},
		'T': {{left, top, right, top}, {left + 4, top, left + 4, bottom}},
		'O': {{left, top, right, top}, {left, top, left, bottom}, {right, top, right, bottom}, {left, bottom, right, bottom}},
		'Q': {{left, top, right, top}, {left, top, left, bottom}, {right, top, right, bottom}, {left, bottom, right, bottom}, {right - 2, bottom - 2, right, bottom}},
		'A': {{left, bottom, left, top + 2}, {left, top + 2, left + 4, top}, {left + 4, top, right, top + 2}, {right, top + 2, right, bottom}, {left, middle, right, middle}},
		'U': {{left, top, left, bottom}, {right, top, right, bottom}, {left, bottom, right, bottom}},
		'Y': {{left, top, left + 4, middle}, {right, top, left + 4, middle}, {left + 4, middle, left + 4, bottom}},
		'N': {{left, bottom, left, top}, {left, top, right, bottom}, {right, bottom, right, top}},
		'G': {{right, top, left, top}, {left, top, left, bottom}, {left, bottom, right, bottom}, {right, bottom, right, middle}, {right, middle, left + 4, middle}},
		'M': {{left, bottom, left, top}, {left, top, left + 4, middle}, {left + 4, middle, right, top}, {right, top, right, bottom}},
		'V': {{left, top, left + 4, bottom}, {left + 4, bottom, right, top}},
		'W': {{left, top, left + 2, bottom}, {left + 2, bottom, left + 4, middle}, {left + 4, middle, left + 6, bottom}, {left + 6, bottom, right, top}},
		'K': {{left, top, left, bottom}, {left, middle, right, top}, {left, middle, right, bottom}},
		'C': {{right, top, left, top}, {left, top, left, bottom}, {left, bottom, right, bottom}},
		'B': {{left, top, left, bottom}, {left, top, right - 2, top}, {right - 2, top, right - 2, middle}, {left, middle, right - 2, middle}, {right - 2, middle, right - 2, bottom}, {left, bottom, right - 2, bottom}},
		'X': {{left, top, right, bottom}, {right, top, left, bottom}},
		'.': {{left + 4, bottom - 1, left + 4, bottom}},
		'/': {{right, top, left, bottom}},
		'+': {{left + 4, top + 2, left + 4, bottom - 2}, {left + 1, middle, right - 1, middle}},
		':': {{left + 4, top + 2, left + 4, top + 3}, {left + 4, bottom - 2, left + 4, bottom - 1}},
		'!': {{left + 4, top, left + 4, bottom - 3}, {left + 4, bottom, left + 4, bottom}},
	}
	for _, segment := range segments[letter] {
		vector.StrokeLine(screen, segment[0], segment[1], segment[2], segment[3], 2, lineColor, true)
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
		vector.StrokeLine(
			screen,
			points[index][0], points[index][1],
			points[next][0], points[next][1],
			2, arrowColor, true,
		)
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

func (g *Game) Draw(screen *ebiten.Image) {
	screen.Fill(background)
	if g.showcaseActive {
		if g.showcaseStarField != nil {
			for _, star := range g.showcaseStarField.Project(g.pipeline) {
				starColor := color.RGBA{R: star.Brightness, G: star.Brightness, B: star.Brightness, A: 255}
				vector.DrawFilledCircle(screen, float32(star.X), float32(star.Y), star.Size, starColor, false)
			}
		}
		g.drawShowcase(screen)
		return
	}
	g.drawStarfield(screen)
	visibleObjects := 0
	viewFrame := g.activeViewFrame()
	// Occlusion masks are drawn before any world geometry so a distant solid
	// billboard hides only the starfield, while nearby fighters remain visible.
	for _, object := range g.objects {
		if !g.swarmLaunched {
			if _, autonomous := g.controllers[object.ID]; autonomous {
				continue
			}
		}
		if normalizedObjectFrame(object) != viewFrame {
			continue
		}
		if definition, ok := g.appearanceRegistry.ForObject(object.Definition, object.Appearance); ok && definition.Kind == "vector-billboard" && definition.Billboard.Occludes {
			g.drawBillboardOcclusion(screen, object)
		}
	}
	for _, object := range g.objects {
		if !g.swarmLaunched {
			if _, autonomous := g.controllers[object.ID]; autonomous {
				continue
			}
		}
		if normalizedObjectFrame(object) != viewFrame {
			continue
		}
		if definition, ok := g.appearanceRegistry.ForObject(object.Definition, object.Appearance); ok && definition.Kind == "vector-billboard" {
			if g.drawBillboard(screen, object, definition.Billboard) {
				visibleObjects++
			}
			continue
		}
		objectVisible := false
		detail := g.objectDetailTier(object)
		for _, part := range object.Parts {
			if part.Detail > detail {
				continue
			}
			insideTargetCockpit := g.viewCamera.Mode == camera.Cockpit && object.ID == g.viewCamera.TargetID
			if insideTargetCockpit && !part.VisibleInCockpit {
				continue
			}
			if !insideTargetCockpit && part.CockpitOnly {
				continue
			}
			for _, line := range g.pipeline.Render(part.Mesh, object.WorldMatrix()) {
				objectVisible = true
				drawLine(screen, line, part.Color, part.LineWidth)
			}
		}
		if objectVisible {
			visibleObjects++
		}
	}
	for _, runtime := range g.environments {
		if runtime.bound.FrameID != viewFrame {
			continue
		}
		for _, tile := range runtime.tiles {
			for _, part := range tile.Parts {
				for _, line := range g.pipeline.Render(part.Mesh, math3d.Identity()) {
					drawLine(screen, line, part.Color, part.LineWidth)
				}
			}
			for _, feature := range tile.Features {
				if runtime.destroyed[feature.ID] {
					continue
				}
				for _, part := range feature.Parts {
					for _, line := range g.pipeline.Render(part.Mesh, feature.Pose.Matrix()) {
						drawLine(screen, line, part.Color, part.LineWidth)
					}
				}
			}
		}
	}
	g.drawTransitionEnvironment(screen)
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

func (g *Game) drawShowcase(screen *ebiten.Image) {
	for _, object := range g.showcaseObjects {
		for _, part := range object.Parts {
			for _, line := range g.pipeline.Render(part.Mesh, object.WorldMatrix()) {
				drawLine(screen, line, part.Color, part.LineWidth)
			}
		}
	}
	title := "FIGHTER SHOWCASE"
	if len(g.showcaseObjects) > 0 {
		if spec, ok := catalog.SpecificationFor(g.showcaseObjects[g.showcaseSelected%len(g.showcaseObjects)].Definition); ok {
			title = spec.Title
		}
	}
	titleColor := color.RGBA{R: 80, G: 180, B: 255, A: 255}
	drawVectorText(screen, ScreenWidth/2, 18, title, titleColor)
	drawVectorText(screen, ScreenWidth/2, 34, "LEFT RIGHT SELECT", titleColor)
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
		if character == ' ' { total += spaceWidth + glyphGap } else { total += glyphWidth + glyphGap }
	}
	left := centerX - (total-glyphGap)/2
	for _, character := range text {
		if character == ' ' { left += spaceWidth + glyphGap; continue }
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

func (g *Game) drawTransitionEnvironment(screen *ebiten.Image) {
	if len(g.transitions) == 0 || g.world == nil {
		return
	}
	for _, transition := range g.transitions {
		if transition.objectID != g.viewCamera.TargetID {
			continue
		}
		var runtime *localEnvironment
		for index := range g.environments {
			if g.environments[index].bound.FrameID == transition.destination {
				runtime = &g.environments[index]
				break
			}
		}
		if runtime == nil {
			continue
		}
		framePose, err := g.world.FramePose(transition.destination)
		if err != nil {
			continue
		}
		// Preload a compact patch around the declared entry corridor. It is
		// rendered in host/world coordinates during the exterior transition;
		// normal surface streaming takes over after the frame transfer.
		for tileX := -2; tileX <= 2; tileX++ {
			for tileZ := -2; tileZ <= 2; tileZ++ {
				coordinate := environment.TileCoordinate{X: tileX, Z: tileZ}
				tile, exists := runtime.tiles[coordinate]
				if !exists {
					tile = runtime.bound.Definition.Tile(coordinate)
				}
				for _, part := range tile.Parts {
					for _, line := range g.pipeline.Render(part.Mesh, framePose.Matrix()) {
						drawLine(screen, line, part.Color, part.LineWidth)
					}
				}
				for _, feature := range tile.Features {
					if runtime.destroyed[feature.ID] {
						continue
					}
					for _, part := range feature.Parts {
						worldMatrix := framePose.Matrix().Mul(feature.Pose.Matrix())
						for _, line := range g.pipeline.Render(part.Mesh, worldMatrix) {
							drawLine(screen, line, part.Color, part.LineWidth)
						}
					}
				}
			}
		}
	}
}

func (g *Game) drawBillboard(screen *ebiten.Image, object scene.Object, billboard appearance.Billboard) bool {
	center, visible := g.pipeline.ProjectPoint(object.Pose.Position)
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
	for _, line := range billboard.Lines(reveal) {
		vector.StrokeLine(
			screen,
			float32(center.X+line.A.X*projectedRadius), float32(center.Y-line.A.Y*projectedRadius),
			float32(center.X+line.B.X*projectedRadius), float32(center.Y-line.B.Y*projectedRadius),
			line.Width, line.Color, true,
		)
	}
	return true
}

func (g *Game) drawBillboardOcclusion(screen *ebiten.Image, object scene.Object) {
	center, visible := g.pipeline.ProjectPoint(object.Pose.Position)
	if !visible || object.VisualRadius <= 0 || center.Depth <= g.pipeline.Near {
		return
	}
	radius := object.VisualRadius / center.Depth * g.pipeline.Projection[1][1] * float64(g.pipeline.Height) * 0.5
	if radius > 0 {
		vector.DrawFilledCircle(screen, float32(center.X), float32(center.Y), float32(radius), background, true)
	}
}

func (g *Game) activeViewFrame() scene.FrameID {
	if target := g.objectByID(g.viewCamera.TargetID); target != nil && g.viewCamera.Mode != camera.Fixed {
		return normalizedObjectFrame(*target)
	}
	return scene.ExteriorFrame
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
	g.detailLevels[object.ID] = current
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
	labels := []string{"ARCADE", "CULLED", "HIDDEN LINE", "DEPTH CUE"}
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

func controlsText(startPrompt bool) string {
	text := "CONTROLS\n" +
		"W/S  throttle    Arrows  steer\n" +
		"Q/E  roll        Mouse  aim\n" +
		"Right mouse button  steer fighter\n" +
		"F / left mouse  fire    G  mouse flight\n" +
		"V  view   Shift  follow fighter   Space  HUD\n" +
		"C  fighter showcase\n" +
		"[ / ]  rendering realism\n" +
		"?  show / hide controls\n\n"
	if startPrompt {
		return text + "PRESS S OR FIRE TO START"
	}
	return text + "PRESS R TO RESTART"
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

func (g *Game) drawStarfield(screen *ebiten.Image) {
	for _, star := range g.starField.Project(g.pipeline) {
		starColor := color.RGBA{R: star.Brightness, G: star.Brightness, B: star.Brightness, A: 255}
		vector.DrawFilledCircle(screen, float32(star.X), float32(star.Y), star.Size, starColor, false)
	}
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
	return fmt.Sprintf(
		"Profile: %s | Mode: %s | %s | View: %s | Pointer: %s | Captured: %s | Tempo: %.1fx | Bolts: %d\nSpeed: %+0.2f  Yaw: %+0.2f  Pitch: %+0.2f  Roll: %+0.2f\n"+
			"Swarm: %d active, %d returning | Objects: %d total, %d visible | Shield: %d/%d | Kills: %d | Collisions: %d\n"+
			"W/S throttle  Mouse/arrows yaw/pitch  Q/E roll  Space stop\nF/left-click fire  G mouse  M mode  V view  P pause  R reset  +/- or wheel zoom",
		g.profile.Name,
		g.mode,
		status,
		g.viewCamera.Mode,
		pointerMode,
		mouseStatus,
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
