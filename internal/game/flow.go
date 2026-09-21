package game

import (
	"fmt"
	"image/color"
	"strings"

	"github.com/edwardwillis/starwars-vector-game/internal/environment"
	"github.com/edwardwillis/starwars-vector-game/internal/profile"
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/hajimehoshi/ebiten/v2/vector"
)

// applicationFlow describes the application shell around an active mission.
// Mission progress remains authoritative in sim.MissionPhase.
type applicationFlow uint8

const (
	flowTitle applicationFlow = iota
	flowBriefing
	flowLaunching
	flowPlaying
	flowOutcome
	flowResult
)

type missionDescriptor struct {
	ID       string
	Title    string
	Playable bool
}

var missionSelection = []missionDescriptor{
	{ID: yavinMissionID, Title: "BATTLE OF YAVIN", Playable: true},
	{ID: "battle-of-hoth", Title: "BATTLE OF HOTH", Playable: false},
	{ID: "battle-of-endor", Title: "BATTLE OF ENDOR", Playable: false},
}

type shellAction uint8

const (
	shellActionNone shellAction = iota
	shellActionMissionPrevious
	shellActionMissionNext
	shellActionDifficultyPrevious
	shellActionDifficultyNext
	shellActionContinue
	shellActionBack
	shellActionLaunch
	shellActionSurfaceDevelopment
)

func difficultyIndex(difficulties []profile.GameProfile, name string) int {
	for index := range difficulties {
		if difficulties[index].Name == name {
			return index
		}
	}
	for index := range difficulties {
		if difficulties[index].Name == profile.PilotName {
			return index
		}
	}
	return 0
}

func difficultyLabel(gameProfile profile.GameProfile) string {
	return strings.ToUpper(strings.TrimPrefix(gameProfile.Name, "builtin/"))
}

func (g *Game) applyShellAction(action shellAction) error {
	switch g.flow {
	case flowTitle:
		switch action {
		case shellActionMissionPrevious:
			if len(missionSelection) > 0 {
				g.selectedMission = (g.selectedMission + len(missionSelection) - 1) % len(missionSelection)
			}
		case shellActionMissionNext:
			if len(missionSelection) > 0 {
				g.selectedMission = (g.selectedMission + 1) % len(missionSelection)
			}
		case shellActionDifficultyPrevious:
			if len(g.curatedDifficulties) > 0 {
				g.selectedDifficulty = (g.selectedDifficulty + len(g.curatedDifficulties) - 1) % len(g.curatedDifficulties)
				g.launchProfile = g.curatedDifficulties[g.selectedDifficulty].Clone()
			}
		case shellActionDifficultyNext:
			if len(g.curatedDifficulties) > 0 {
				g.selectedDifficulty = (g.selectedDifficulty + 1) % len(g.curatedDifficulties)
				g.launchProfile = g.curatedDifficulties[g.selectedDifficulty].Clone()
			}
		case shellActionContinue:
			return g.continueToSelectedBriefing()
		case shellActionSurfaceDevelopment:
			g.selectedMission = 0
			return g.launchYavinSession(true)
		}
	case flowBriefing:
		switch action {
		case shellActionBack:
			g.flow = flowTitle
		case shellActionLaunch:
			return g.launchYavinSession(false)
		}
	}
	return nil
}

func (g *Game) continueToSelectedBriefing() error {
	if g.selectedMission < 0 || g.selectedMission >= len(missionSelection) {
		return fmt.Errorf("selected mission index %d is out of range", g.selectedMission)
	}
	mission := missionSelection[g.selectedMission]
	if !mission.Playable {
		return nil
	}
	if mission.ID != yavinMissionID {
		return fmt.Errorf("mission %q is marked playable but has no launch implementation", mission.ID)
	}
	g.flow = flowBriefing
	return nil
}

func (g *Game) constructYavinSession() (*Game, error) {
	freshEnvironments := environment.NewRegistry()
	for _, definition := range g.environmentDefinitions {
		if err := freshEnvironments.Register(definition); err != nil {
			return nil, fmt.Errorf("prepare Battle of Yavin environment: %w", err)
		}
	}
	fresh, err := NewWithRegistriesAndAppearances(g.launchProfile, g.controllerRegistry, g.catalogRegistry, freshEnvironments, g.appearanceRegistry)
	if err != nil {
		return nil, fmt.Errorf("prepare Battle of Yavin session: %w", err)
	}
	return fresh, nil
}

// adoptYavinSession replaces run-scoped state while deliberately retaining the
// application shell, selected launch profile, renderer/input preferences, and
// reusable presentation caches owned by the application.
func (g *Game) adoptYavinSession(fresh *Game) {
	g.profile = fresh.profile
	g.environmentRegistry = fresh.environmentRegistry
	g.environments = fresh.environments
	g.transitions = fresh.transitions
	g.transitionCommitments = fresh.transitionCommitments
	g.objects = fresh.objects
	g.pipeline = fresh.pipeline
	g.initialPose = fresh.initialPose
	g.autoMotion = fresh.autoMotion
	g.mode = fresh.mode
	g.paused = fresh.paused
	g.quitPrompt = fresh.quitPrompt
	g.swarmLaunched = fresh.swarmLaunched
	g.viewCamera = fresh.viewCamera
	g.nextObjectID = fresh.nextObjectID
	g.projectiles = fresh.projectiles
	g.owners = fresh.owners
	g.fireCooldown = fresh.fireCooldown
	g.torpedoCooldown = fresh.torpedoCooldown
	g.torpedoesRemaining = fresh.torpedoesRemaining
	g.simulationTime = fresh.simulationTime
	g.fireHistory = fresh.fireHistory
	g.nextMuzzlePair = fresh.nextMuzzlePair
	g.laserBeamTime = fresh.laserBeamTime
	g.laserBeamPair = fresh.laserBeamPair
	g.starField = fresh.starField
	g.controllers = fresh.controllers
	g.controllerTargets = fresh.controllerTargets
	g.debris = fresh.debris
	g.surfaceEffects = fresh.surfaceEffects
	g.environmentContacts = fresh.environmentContacts
	g.respawns = fresh.respawns
	g.respawnSequence = fresh.respawnSequence
	g.playerDestroyed = fresh.playerDestroyed
	g.playerViewMode = fresh.playerViewMode
	g.kills = fresh.kills
	g.collisions = fresh.collisions
	g.visibleObjects = 0
	g.renderStats = fresh.renderStats
	g.depthBuffer = fresh.depthBuffer
	g.billboardBatches = g.billboardBatches[:0]
	g.worldBatches = g.worldBatches[:0]
	g.worldJobs = g.worldJobs[:0]
	g.prepared = preparedFrame{}
	g.viewContext = fresh.viewContext
	g.visibleObjectIDs = fresh.visibleObjectIDs
	g.starVertices = g.starVertices[:0]
	g.starIndices = g.starIndices[:0]
	g.surfaceTriangles = g.surfaceTriangles[:0]
	g.surfaceVertices = g.surfaceVertices[:0]
	g.surfaceIndices = g.surfaceIndices[:0]
	g.starPoints = g.starPoints[:0]
	g.starOccluders = fresh.starOccluders
	g.world = fresh.world
	g.detailLevels = fresh.detailLevels
	g.shieldStrength = fresh.shieldStrength
	g.shieldQuietTime = fresh.shieldQuietTime
	g.destructionViewRemaining = fresh.destructionViewRemaining
	g.destructionVictim = fresh.destructionVictim
	g.controlsRemaining = 0
	g.hyperspaceArrival = fresh.hyperspaceArrival

	// Pipeline construction follows the selected gameplay profile, while the
	// user's runtime realism choice remains an application preference.
	g.setRealismLevel(g.realismLevel)
	g.pipeline.Stats = &g.renderStats
	g.reconcileMouseFlightPreference()
	g.refreshViewContext()
}

func (g *Game) reconcileMouseFlightPreference() {
	if g.mouseFlight {
		g.mode = modeManual
		ebiten.SetCursorMode(ebiten.CursorModeCaptured)
		g.mouseNeutralX, g.mouseNeutralY = ebiten.CursorPosition()
		return
	}
	ebiten.SetCursorMode(ebiten.CursorModeVisible)
}

func (g *Game) launchYavinSession(surfaceDevelopment bool) error {
	if g.selectedMission < 0 || g.selectedMission >= len(missionSelection) {
		return fmt.Errorf("selected mission index %d is out of range", g.selectedMission)
	}
	mission := missionSelection[g.selectedMission]
	if mission.ID != yavinMissionID || !mission.Playable {
		return fmt.Errorf("Battle of Yavin launch requested for unsupported mission %q", mission.ID)
	}
	if !surfaceDevelopment && g.flow != flowBriefing {
		return nil
	}
	fresh, err := g.constructYavinSession()
	if err != nil {
		return err
	}
	g.adoptYavinSession(fresh)
	surfaceStarted := surfaceDevelopment && g.startInSurfaceMode()
	if err := g.startYavinMission(surfaceStarted); err != nil {
		return err
	}
	g.controlsRemaining = 0
	g.flow = flowPlaying
	if !surfaceStarted {
		fighter := g.objectByID(fighterID)
		if fighter != nil && g.beginHyperspaceArrival(fighter.Pose) {
			g.flow = flowLaunching
		}
	}
	return nil
}

func (g *Game) updateApplicationFlow(seconds float64) (bool, error) {
	switch g.flow {
	case flowTitle:
		action := shellActionNone
		switch {
		case inpututil.IsKeyJustPressed(ebiten.KeyArrowUp):
			action = shellActionMissionPrevious
		case inpututil.IsKeyJustPressed(ebiten.KeyArrowDown):
			action = shellActionMissionNext
		case inpututil.IsKeyJustPressed(ebiten.KeyArrowLeft) || inpututil.IsKeyJustPressed(ebiten.KeyA):
			action = shellActionDifficultyPrevious
		case inpututil.IsKeyJustPressed(ebiten.KeyArrowRight) || inpututil.IsKeyJustPressed(ebiten.KeyD):
			action = shellActionDifficultyNext
		case inpututil.IsKeyJustPressed(ebiten.KeyN):
			action = shellActionSurfaceDevelopment
		case inpututil.IsKeyJustPressed(ebiten.KeyEnter) || inpututil.IsKeyJustPressed(ebiten.KeySpace) || inpututil.IsKeyJustPressed(ebiten.KeyF) || inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft):
			action = shellActionContinue
		}
		return true, g.applyShellAction(action)
	case flowBriefing:
		action := shellActionNone
		switch {
		case inpututil.IsKeyJustPressed(ebiten.KeyBackspace):
			action = shellActionBack
		case inpututil.IsKeyJustPressed(ebiten.KeyEnter) || inpututil.IsKeyJustPressed(ebiten.KeySpace) || inpututil.IsKeyJustPressed(ebiten.KeyF) || inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft):
			action = shellActionLaunch
		}
		return true, g.applyShellAction(action)
	case flowLaunching:
		if !g.paused {
			g.advanceHyperspaceArrival(seconds)
		}
		return true, nil
	case flowOutcome, flowResult:
		return true, nil
	default:
		return false, nil
	}
}

func (g *Game) drawApplicationShell(screen *ebiten.Image) bool {
	switch g.flow {
	case flowTitle:
		g.drawTitleScreen(screen)
		return true
	case flowBriefing:
		g.drawYavinBriefing(screen)
		return true
	case flowOutcome, flowResult:
		return true
	default:
		return false
	}
}

func drawShellPanel(screen *ebiten.Image, left, top, right, bottom float32) {
	panel := color.RGBA{R: 32, G: 210, B: 255, A: 255}
	vector.StrokeLine(screen, left, top, right, top, 2, panel, true)
	vector.StrokeLine(screen, right, top, right, bottom, 2, panel, true)
	vector.StrokeLine(screen, right, bottom, left, bottom, 2, panel, true)
	vector.StrokeLine(screen, left, bottom, left, top, 2, panel, true)
}

func drawSelectionMarker(screen *ebiten.Image, x, y float32) {
	selection := color.RGBA{R: 255, G: 224, B: 32, A: 255}
	vector.StrokeLine(screen, x, y, x+8, y+6, 2, selection, true)
	vector.StrokeLine(screen, x+8, y+6, x, y+12, 2, selection, true)
}

func (g *Game) drawTitleScreen(screen *ebiten.Image) {
	blue := color.RGBA{R: 64, G: 220, B: 255, A: 255}
	green := color.RGBA{R: 64, G: 255, B: 128, A: 255}
	muted := color.RGBA{R: 96, G: 112, B: 128, A: 255}
	yellow := color.RGBA{R: 255, G: 224, B: 32, A: 255}
	drawShellPanel(screen, 230, 58, 730, 482)
	drawVectorText(screen, ScreenWidth/2, 82, "STAR WARS", yellow)
	drawVectorText(screen, ScreenWidth/2, 112, "VECTOR GAME", blue)
	drawVectorText(screen, ScreenWidth/2, 158, "SELECT MISSION", blue)
	for index, mission := range missionSelection {
		y := float32(196 + index*34)
		lineColor := green
		label := mission.Title
		if !mission.Playable {
			lineColor = muted
			label += "  LOCKED"
		}
		drawVectorText(screen, ScreenWidth/2, y, label, lineColor)
		if index == g.selectedMission {
			drawSelectionMarker(screen, 302, y)
		}
	}
	drawVectorText(screen, ScreenWidth/2, 318, "DIFFICULTY", blue)
	drawVectorText(screen, ScreenWidth/2, 346, difficultyLabel(g.launchProfile), green)
	drawVectorText(screen, ScreenWidth/2, 394, "ARROWS SELECT", muted)
	drawVectorText(screen, ScreenWidth/2, 418, "ENTER CONTINUE", yellow)
	drawVectorText(screen, ScreenWidth/2, 450, "N SURFACE DEVELOPMENT", muted)
}

func (g *Game) drawYavinBriefing(screen *ebiten.Image) {
	blue := color.RGBA{R: 64, G: 220, B: 255, A: 255}
	green := color.RGBA{R: 64, G: 255, B: 128, A: 255}
	muted := color.RGBA{R: 128, G: 176, B: 192, A: 255}
	yellow := color.RGBA{R: 255, G: 224, B: 32, A: 255}
	drawShellPanel(screen, 210, 78, 750, 462)
	drawVectorText(screen, ScreenWidth/2, 104, "BATTLE OF YAVIN", blue)
	drawVectorText(screen, ScreenWidth/2, 156, "DEATH STAR ASSAULT", yellow)
	drawVectorText(screen, ScreenWidth/2, 210, "ENGAGE IMPERIAL FORCES", green)
	drawVectorText(screen, ScreenWidth/2, 238, "APPROACH THE DEATH STAR", green)
	drawVectorText(screen, ScreenWidth/2, 286, "DIFFICULTY", muted)
	drawVectorText(screen, ScreenWidth/2, 312, difficultyLabel(g.launchProfile), blue)
	drawVectorText(screen, ScreenWidth/2, 376, "ENTER LAUNCH", yellow)
	drawVectorText(screen, ScreenWidth/2, 410, "BACKSPACE MISSION SELECT", muted)
}
