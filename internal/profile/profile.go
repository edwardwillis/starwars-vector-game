// Package profile defines validated, immutable-by-convention game-session
// configuration. A profile is cloned when a game is created so caller-owned
// slices cannot change a running session.
package profile

import (
	"fmt"
	"math"

	"github.com/edwardwillis/starwars-vector-game/internal/combat"
	"github.com/edwardwillis/starwars-vector-game/internal/control"
	"github.com/edwardwillis/starwars-vector-game/internal/kinematics"
	"github.com/edwardwillis/starwars-vector-game/internal/math3d"
	"github.com/edwardwillis/starwars-vector-game/internal/scene"
)

const (
	CadetName             = "builtin/cadet"
	PilotName             = "builtin/pilot"
	AceName               = "builtin/ace"
	NightmareName         = "builtin/nightmare"
	StarfieldModeWorld    = "world"
	StarfieldModeSkyfield = "skyfield"
)

type SimulationConfig struct {
	TickSeconds               float64
	MotionScale               float64
	DisintegrationTime        float64
	PlayerDestructionViewTime float64
	// HyperspaceArrivalTime controls the short orbital-space presentation
	// played when the player starts or restarts. Surface-frame starts skip it.
	HyperspaceArrivalTime float64
}

type DisplayConfig struct {
	ZoomSpeed               float64
	ControlsDisplayDuration float64
	RenderingProfile        string
	VerticalFOV             float64
	NearPlane               float64
	FarPlane                float64
}

type InputConfig struct {
	MouseDeadzone    float64
	MouseSensitivity float64
}

type StarfieldConfig struct {
	Count  int
	Radius float64
	Seed   int64
	// Mode controls whether stars retain world-space parallax or form a
	// distant directional skyfield. Empty values resolve to skyfield.
	Mode string
}

type TargetingConfig struct {
	AimRadius      float64
	AimConvergence float64
}

type CombatConfig struct {
	Laser         combat.LaserConfig
	Torpedo       combat.TorpedoConfig
	FireInterval  float64
	FireWindow    float64
	MaxFireEvents int
	BeamTime      float64
}

type ShieldConfig struct {
	Maximum          int
	LaserDamage      int
	CollisionDamage  int
	RechargeInterval float64
}

type PlayerConfig struct {
	Object          string
	Team            scene.TeamID
	InitialPose     kinematics.Pose
	AutopilotMotion kinematics.Motion
	Flight          control.ManualConfig
	AutoLevel       control.AutoLevelConfig
	Shield          ShieldConfig
}

type DifficultyConfig struct {
	Name string
}

type SwarmConfig struct {
	Object           string
	Team             scene.TeamID
	Count            int
	Controller       string
	Flight           control.Limits
	InitialPositions []math3d.Vec3
	InitialSpeed     float64
	SpeedStep        float64
	AimError         float64
	RespawnDelay     float64
	SpawnRadius      float64
	RespawnDistance  float64
	Pursuit          control.PursuitConfig
}

// SurfaceCombatConfig tunes authoritative local-environment gameplay without
// changing orbital motion or the global simulation clock. Values are profile
// data so difficulty modes and downstream games can select a different pace.
type SurfaceCombatConfig struct {
	CruiseSpeed         float64
	MaxForward          float64
	Acceleration        float64
	InitialAttackers    int
	MaxAttackers        int
	ReinforcementDelay  float64
	CannonRange         float64
	CannonFireMinGap    float64
	CannonFireMaxGap    float64
	CannonAimError      float64
	CannonTraverseSpeed float64
	CannonYawLimit      float64
	CannonPitchLimit    float64
	CannonFireTolerance float64
	CannonBoltLifetime  float64
	MaxActiveCannons    int
	MinimumAltitude     float64
	TerrainLookAhead    float64
	GuidanceStrength    float64
}

type ObjectPlacement struct {
	Definition string
	Appearance string
	Pose       kinematics.Pose
}

type WorldConfig struct {
	Objects []ObjectPlacement
}

// GameProfile is the fully resolved configuration for one game session.
type GameProfile struct {
	Name       string
	Version    int
	Simulation SimulationConfig
	Display    DisplayConfig
	Input      InputConfig
	Starfield  StarfieldConfig
	Targeting  TargetingConfig
	Combat     CombatConfig
	Player     PlayerConfig
	Swarm      SwarmConfig
	Surface    SurfaceCombatConfig
	World      WorldConfig
	Difficulty DifficultyConfig
}

// Pilot returns the current balanced game as a named profile. Its values match
// the gameplay tuning that preceded profile extraction.
func Pilot() GameProfile {
	manual := control.DefaultManualConfig()
	// Gameplay units use the X-Wing's 1050 km/h specification as the baseline;
	// the TIE's 1200 km/h maximum is represented as a 15% advantage.
	manual.MaxForward = 3.0
	autoLevel := control.AutoLevelConfig{
		Enabled: true, CorrectionGain: 2.4, MaxRollRate: 1.2,
		AngleDeadzone: math.Pi / 180, TurnDeadzone: 0.05,
	}
	pursuit := control.DefaultPursuitConfig()
	pursuit.PreferredDistance = 10.0
	pursuit.MinSpeed = 2.80
	pursuit.MaxSpeed = manual.MaxForward * (1200.0 / 1050.0)
	pursuit.Acceleration = 2.20
	pursuit.TurnAcceleration = 2.40
	pursuit.ApproachGain = 0.28
	pursuit.MaxYawRate = 1.15
	pursuit.MaxPitchRate = 0.90
	pursuit.MaxRollRate = 1.35
	pursuit.WanderStrength = 0.30
	pursuit.WanderInterval = 0.55
	pursuit.ScatterRadius = 4.0
	pursuit.ExcursionMinGap = 2.5
	pursuit.ExcursionMaxGap = 6.0
	pursuit.ExcursionMinTime = 1.5
	pursuit.ExcursionMaxTime = 3.5
	pursuit.SpeedVariation = 0.35
	pursuit.SpeedChangeInterval = 5.0
	pursuit.SpeedBlendRate = 0.65
	pursuit.AvoidanceHorizon = 2.75
	pursuit.AvoidanceMargin = 6.0
	pursuit.AvoidanceMinTime = 1.4
	pursuit.AvoidanceMaxTime = 2.0
	pursuit.AvoidanceCooldown = 0.08
	pursuit.AvoidanceSlowdown = 0.35
	pursuit.AttackMinGap = 1.5
	pursuit.AttackMaxGap = 6.0
	pursuit.AttackMinTime = 3.0
	pursuit.AttackMaxTime = 6.0
	pursuit.AttackMinRadius = 5.0
	pursuit.AttackMaxRadius = 14.0
	pursuit.AttackFireMinGap = 0.90
	pursuit.AttackFireMaxGap = 1.60
	pursuit.AttackRange = 68.0
	// Attack runs may fire throughout the arc; projectile aiming applies the
	// configured deterministic error independently.
	pursuit.AttackAimDot = -1.0

	return GameProfile{
		Name:    PilotName,
		Version: 1,
		Simulation: SimulationConfig{
			TickSeconds:               1.0 / 60.0,
			MotionScale:               2.0,
			DisintegrationTime:        2.0,
			PlayerDestructionViewTime: 3.0,
			HyperspaceArrivalTime:     1.8,
		},
		Display: DisplayConfig{
			ZoomSpeed:               1.5,
			ControlsDisplayDuration: 10.0,
			// Start gameplay at the highest available detail level. The HUD
			// realism slider still allows the player to select a cheaper mode.
			RenderingProfile: "builtin/maximum",
			VerticalFOV:      math.Pi / 3,
			NearPlane:        0.1,
			FarPlane:         1000,
		},
		Input: InputConfig{
			MouseDeadzone:    0.08,
			MouseSensitivity: 1.25,
		},
		Starfield: StarfieldConfig{Count: 500, Radius: 40, Seed: 42, Mode: StarfieldModeSkyfield},
		Targeting: TargetingConfig{AimRadius: 190, AimConvergence: 30},
		Combat: CombatConfig{
			Laser:         combat.DefaultLaserConfig(),
			Torpedo:       combat.DefaultTorpedoConfig(),
			FireInterval:  0.12,
			FireWindow:    1.5,
			MaxFireEvents: 3,
			BeamTime:      0.08,
		},
		Player: PlayerConfig{
			Object: "builtin/x-wing", Team: scene.TeamAlliance,
			InitialPose: kinematics.Pose{
				// Start close enough for the Death Star to read as the immediate
				// play-space landmark while remaining safely outside its forward
				// surface and hangar launch formation.
				Position:    math3d.Vec3{Z: -150},
				Orientation: math3d.QuaternionFromYawPitchRoll(0, 0, 0),
			},
			AutopilotMotion: kinematics.Motion{
				Speed:    manual.MaxForward,
				YawRate:  0.22,
				RollRate: 0.16,
			},
			Flight:    manual,
			AutoLevel: autoLevel,
			Shield: ShieldConfig{
				Maximum:          8,
				LaserDamage:      1,
				CollisionDamage:  3,
				RechargeInterval: 20,
			},
		},
		Swarm: SwarmConfig{
			Object: "builtin/tie-fighter", Team: scene.TeamEmpire,
			Count:      5,
			Controller: control.PursuitName,
			Flight: control.Limits{
				Acceleration:        pursuit.Acceleration,
				AngularAcceleration: pursuit.TurnAcceleration,
				MaxForward:          pursuit.MaxSpeed,
				MaxReverse:          pursuit.MaxSpeed,
				MaxYawRate:          pursuit.MaxYawRate,
				MaxPitchRate:        pursuit.MaxPitchRate,
				MaxRollRate:         pursuit.MaxRollRate,
			},
			// Launch formation just outside the Death Star's forward hangar.
			InitialPositions: []math3d.Vec3{{X: -14, Y: -10, Z: 88}, {X: 0, Y: -10, Z: 88}, {X: 14, Y: -10, Z: 88}, {X: -7, Y: 8, Z: 88}, {X: 7, Y: 8, Z: 88}},
			InitialSpeed:     2.85,
			SpeedStep:        0.12,
			AimError:         3.2,
			RespawnDelay:     3.0,
			SpawnRadius:      12.0,
			RespawnDistance:  48.0,
			Pursuit:          pursuit,
		},
		Surface: SurfaceCombatConfig{
			CruiseSpeed: 4.5, MaxForward: 5.5, Acceleration: 3.4,
			InitialAttackers: 3, MaxAttackers: 5, ReinforcementDelay: 4,
			CannonRange: 82, CannonFireMinGap: 0.8, CannonFireMaxGap: 1.45,
			CannonAimError: 4.8, CannonBoltLifetime: 2.3, MaxActiveCannons: 4,
			CannonTraverseSpeed: 2.4, CannonYawLimit: 1.55, CannonPitchLimit: 1.5, CannonFireTolerance: 0.09,
			MinimumAltitude: 5.5, TerrainLookAhead: 18, GuidanceStrength: 0.85,
		},
		World: WorldConfig{Objects: []ObjectPlacement{{
			Definition: "builtin/death-star",
			Appearance: "builtin/death-star-arcade-billboard",
			Pose:       kinematics.Pose{Position: math3d.Vec3{Z: 400}, Orientation: math3d.IdentityQuaternion()},
		}}},
		Difficulty: DifficultyConfig{Name: PilotName},
	}
}

// Cadet resolves the forgiving built-in difficulty overlay into a complete
// profile suitable for validation, serialization, and session creation.
func Cadet() GameProfile {
	profile := Pilot()
	profile.Name = CadetName
	profile.Difficulty.Name = CadetName
	profile.Swarm.Count = 3
	profile.Swarm.InitialPositions = append([]math3d.Vec3(nil), profile.Swarm.InitialPositions[:3]...)
	profile.Swarm.InitialSpeed = 2.55
	profile.Swarm.SpeedStep = 0.08
	profile.Swarm.AimError = 5.2
	profile.Swarm.RespawnDelay = 5
	profile.Swarm.Pursuit.MinSpeed = 2.45
	profile.Swarm.Pursuit.MaxSpeed = 3.0
	profile.Swarm.Pursuit.AttackMinGap = 3.5
	profile.Swarm.Pursuit.AttackMaxGap = 8
	profile.Swarm.Pursuit.AttackFireMinGap = 1.3
	profile.Swarm.Pursuit.AttackFireMaxGap = 2.1
	profile.Surface.InitialAttackers = 2
	profile.Surface.MaxAttackers = 3
	profile.Surface.ReinforcementDelay = 5.5
	profile.Surface.MaxActiveCannons = 2
	profile.Surface.CannonFireMinGap = 1.2
	profile.Surface.CannonFireMaxGap = 2.0
	syncSwarmFlight(&profile)
	return profile
}

// Ace resolves the aggressive built-in difficulty overlay.
func Ace() GameProfile {
	profile := Pilot()
	profile.Name = AceName
	profile.Difficulty.Name = AceName
	profile.Swarm.InitialSpeed = 3.05
	profile.Swarm.SpeedStep = 0.13
	profile.Swarm.AimError = 2.2
	profile.Swarm.RespawnDelay = 2.5
	profile.Swarm.Pursuit.MinSpeed = 3.0
	profile.Swarm.Pursuit.MaxSpeed = 3.75
	profile.Swarm.Pursuit.AttackMinGap = 1.0
	profile.Swarm.Pursuit.AttackMaxGap = 4.0
	profile.Swarm.Pursuit.AttackFireMinGap = 0.65
	profile.Swarm.Pursuit.AttackFireMaxGap = 1.1
	profile.Surface.ReinforcementDelay = 3.2
	profile.Surface.MaxActiveCannons = 5
	profile.Surface.CannonFireMinGap = 0.65
	profile.Surface.CannonFireMaxGap = 1.15
	syncSwarmFlight(&profile)
	return profile
}

// Nightmare resolves the maximum-pressure built-in difficulty overlay.
func Nightmare() GameProfile {
	profile := Ace()
	profile.Name = NightmareName
	profile.Difficulty.Name = NightmareName
	profile.Swarm.Count = 7
	profile.Swarm.InitialPositions = append(profile.Swarm.InitialPositions,
		math3d.Vec3{X: -21, Y: 8, Z: 88},
		math3d.Vec3{X: 21, Y: 8, Z: 88},
	)
	profile.Swarm.InitialSpeed = 3.25
	profile.Swarm.AimError = 1.25
	profile.Swarm.RespawnDelay = 1.5
	profile.Swarm.Pursuit.MinSpeed = 3.2
	profile.Swarm.Pursuit.MaxSpeed = 4.1
	profile.Swarm.Pursuit.AttackMinGap = 0.5
	profile.Swarm.Pursuit.AttackMaxGap = 2.5
	profile.Swarm.Pursuit.AttackFireMinGap = 0.45
	profile.Swarm.Pursuit.AttackFireMaxGap = 0.85
	profile.Surface.InitialAttackers = 4
	profile.Surface.MaxAttackers = 7
	profile.Surface.ReinforcementDelay = 2.2
	profile.Surface.MaxActiveCannons = 6
	profile.Surface.CannonFireMinGap = 0.45
	profile.Surface.CannonFireMaxGap = 0.9
	syncSwarmFlight(&profile)
	return profile
}

func syncSwarmFlight(profile *GameProfile) {
	config := profile.Swarm.Pursuit
	profile.Swarm.Flight = control.Limits{
		Acceleration:        config.Acceleration,
		AngularAcceleration: config.TurnAcceleration,
		MaxForward:          config.MaxSpeed,
		MaxReverse:          config.MaxSpeed,
		MaxYawRate:          config.MaxYawRate,
		MaxPitchRate:        config.MaxPitchRate,
		MaxRollRate:         config.MaxRollRate,
	}
}

// Builtin resolves a stable built-in profile name.
func Builtin(name string) (GameProfile, error) {
	switch name {
	case CadetName, "cadet":
		return Cadet(), nil
	case PilotName, "pilot":
		return Pilot(), nil
	case AceName, "ace":
		return Ace(), nil
	case NightmareName, "nightmare":
		return Nightmare(), nil
	default:
		return GameProfile{}, fmt.Errorf("unknown built-in profile %q", name)
	}
}

// Builtins returns the curated difficulty profiles in their intended menu
// order. Each call constructs independent profile values so callers may select
// or validate them without sharing mutable slice storage.
func Builtins() []GameProfile {
	return []GameProfile{Cadet(), Pilot(), Ace(), Nightmare()}
}

// Clone protects a running session from mutations to caller-owned slices.
func (profile GameProfile) Clone() GameProfile {
	profile.Swarm.InitialPositions = append([]math3d.Vec3(nil), profile.Swarm.InitialPositions...)
	profile.World.Objects = append([]ObjectPlacement(nil), profile.World.Objects...)
	return profile
}

func (profile GameProfile) Validate() error {
	if profile.Name == "" {
		return fmt.Errorf("profile name is required")
	}
	if profile.Version <= 0 {
		return fmt.Errorf("profile version must be positive")
	}
	if profile.Difficulty.Name == "" {
		return fmt.Errorf("difficulty name is required")
	}
	if profile.Player.Object == "" {
		return fmt.Errorf("player object definition is required")
	}
	if profile.Player.Team == scene.TeamNeutral {
		return fmt.Errorf("player team is required")
	}
	if profile.Swarm.Object == "" {
		return fmt.Errorf("swarm object definition is required")
	}
	if profile.Swarm.Team == scene.TeamNeutral || profile.Swarm.Team == profile.Player.Team {
		return fmt.Errorf("swarm requires a team opposing the player")
	}
	for index, placement := range profile.World.Objects {
		if placement.Definition == "" {
			return fmt.Errorf("world object %d definition is required", index)
		}
	}
	if err := validatePositive("zoom speed", profile.Display.ZoomSpeed); err != nil {
		return err
	}
	if profile.Display.ControlsDisplayDuration < 0 || math.IsNaN(profile.Display.ControlsDisplayDuration) || math.IsInf(profile.Display.ControlsDisplayDuration, 0) {
		return fmt.Errorf("controls display duration must be a finite non-negative value")
	}
	if err := validatePositive("simulation tick", profile.Simulation.TickSeconds); err != nil {
		return err
	}
	if err := validatePositive("motion scale", profile.Simulation.MotionScale); err != nil {
		return err
	}
	if err := validatePositive("disintegration time", profile.Simulation.DisintegrationTime); err != nil {
		return err
	}
	if err := validatePositive("player destruction view time", profile.Simulation.PlayerDestructionViewTime); err != nil {
		return err
	}
	if err := validatePositive("hyperspace arrival time", profile.Simulation.HyperspaceArrivalTime); err != nil {
		return err
	}
	if profile.Display.RenderingProfile == "" {
		return fmt.Errorf("rendering profile is required")
	}
	if err := validatePositive("vertical field of view", profile.Display.VerticalFOV); err != nil {
		return err
	}
	if profile.Display.VerticalFOV >= math.Pi {
		return fmt.Errorf("vertical field of view must be less than pi")
	}
	if err := validatePositive("near plane", profile.Display.NearPlane); err != nil {
		return err
	}
	if profile.Display.FarPlane <= profile.Display.NearPlane {
		return fmt.Errorf("far plane must be greater than near plane")
	}
	if profile.Input.MouseDeadzone < 0 || profile.Input.MouseDeadzone >= 1 {
		return fmt.Errorf("mouse deadzone must be in [0, 1)")
	}
	if err := validatePositive("mouse sensitivity", profile.Input.MouseSensitivity); err != nil {
		return err
	}
	if profile.Starfield.Count < 0 {
		return fmt.Errorf("star count cannot be negative")
	}
	if err := validatePositive("starfield radius", profile.Starfield.Radius); err != nil {
		return err
	}
	if profile.Starfield.Mode != "" && profile.Starfield.Mode != StarfieldModeWorld && profile.Starfield.Mode != StarfieldModeSkyfield {
		return fmt.Errorf("unknown starfield mode %q", profile.Starfield.Mode)
	}
	if err := validatePositive("aim radius", profile.Targeting.AimRadius); err != nil {
		return err
	}
	if err := validatePositive("aim convergence", profile.Targeting.AimConvergence); err != nil {
		return err
	}
	if err := profile.Combat.Laser.Validate(); err != nil {
		return fmt.Errorf("combat: %w", err)
	}
	if err := profile.Combat.Torpedo.Validate(); err != nil {
		return fmt.Errorf("combat: %w", err)
	}
	if err := validatePositive("fire interval", profile.Combat.FireInterval); err != nil {
		return err
	}
	if err := validatePositive("fire window", profile.Combat.FireWindow); err != nil {
		return err
	}
	if profile.Combat.MaxFireEvents <= 0 {
		return fmt.Errorf("maximum fire events must be positive")
	}
	if err := validatePositive("laser beam time", profile.Combat.BeamTime); err != nil {
		return err
	}
	if err := validateManual(profile.Player.Flight); err != nil {
		return fmt.Errorf("player flight: %w", err)
	}
	if err := validateAutoLevel(profile.Player.AutoLevel); err != nil {
		return fmt.Errorf("player auto-level: %w", err)
	}
	if err := validateSurfaceCombat(profile.Surface); err != nil {
		return fmt.Errorf("surface combat: %w", err)
	}
	if profile.Player.Shield.Maximum <= 0 || profile.Player.Shield.LaserDamage <= 0 || profile.Player.Shield.CollisionDamage <= 0 {
		return fmt.Errorf("shield maximum and damage values must be positive")
	}
	if err := validatePositive("shield recharge interval", profile.Player.Shield.RechargeInterval); err != nil {
		return err
	}
	if profile.Swarm.Count < 0 {
		return fmt.Errorf("swarm count cannot be negative")
	}
	if profile.Swarm.Controller == "" {
		return fmt.Errorf("swarm controller is required")
	}
	if err := validateLimits(profile.Swarm.Flight); err != nil {
		return fmt.Errorf("swarm flight: %w", err)
	}
	if len(profile.Swarm.InitialPositions) != profile.Swarm.Count {
		return fmt.Errorf("swarm has %d initial positions for %d fighters", len(profile.Swarm.InitialPositions), profile.Swarm.Count)
	}
	if profile.Swarm.Count > 0 {
		if err := validatePositive("swarm initial speed", profile.Swarm.InitialSpeed); err != nil {
			return err
		}
		if err := validatePositive("swarm respawn delay", profile.Swarm.RespawnDelay); err != nil {
			return err
		}
		if err := validatePositive("swarm spawn radius", profile.Swarm.SpawnRadius); err != nil {
			return err
		}
		if err := validatePositive("swarm respawn distance", profile.Swarm.RespawnDistance); err != nil {
			return err
		}
		if err := validateNonNegative("swarm speed step", profile.Swarm.SpeedStep); err != nil {
			return err
		}
	}
	if err := validateNonNegative("swarm aim error", profile.Swarm.AimError); err != nil {
		return err
	}
	if err := validatePursuit(profile.Swarm.Pursuit); err != nil {
		return fmt.Errorf("swarm pursuit: %w", err)
	}
	return nil
}

func validateManual(config control.ManualConfig) error {
	return validateLimits(control.Limits{
		Acceleration: config.Acceleration,
		MaxForward:   config.MaxForward,
		MaxReverse:   config.MaxReverse,
		MaxYawRate:   config.MaxYawRate,
		MaxPitchRate: config.MaxPitchRate,
		MaxRollRate:  config.MaxRollRate,
	})
}

func validateAutoLevel(config control.AutoLevelConfig) error {
	if config.CorrectionGain < 0 || config.MaxRollRate < 0 || config.AngleDeadzone < 0 || config.TurnDeadzone < 0 || config.TurnDeadzone > 1 {
		return fmt.Errorf("rates and deadzones must be non-negative and turn deadzone must not exceed 1")
	}
	if !config.Enabled {
		return nil
	}
	if config.CorrectionGain == 0 || config.MaxRollRate == 0 {
		return fmt.Errorf("enabled assistance requires positive correction gain and maximum roll rate")
	}
	return nil
}

func validateLimits(config control.Limits) error {
	values := []struct {
		name  string
		value float64
	}{
		{"acceleration", config.Acceleration},
		{"maximum forward speed", config.MaxForward},
		{"maximum reverse speed", config.MaxReverse},
		{"maximum yaw rate", config.MaxYawRate},
		{"maximum pitch rate", config.MaxPitchRate},
		{"maximum roll rate", config.MaxRollRate},
	}
	for _, value := range values {
		if err := validatePositive(value.name, value.value); err != nil {
			return err
		}
	}
	return nil
}

func validateSurfaceCombat(config SurfaceCombatConfig) error {
	positive := []struct {
		name  string
		value float64
	}{
		{"cruise speed", config.CruiseSpeed},
		{"maximum forward speed", config.MaxForward},
		{"acceleration", config.Acceleration},
		{"reinforcement delay", config.ReinforcementDelay},
		{"cannon range", config.CannonRange},
		{"cannon minimum fire gap", config.CannonFireMinGap},
		{"cannon maximum fire gap", config.CannonFireMaxGap},
		{"cannon bolt lifetime", config.CannonBoltLifetime},
		{"cannon traverse speed", config.CannonTraverseSpeed},
		{"cannon yaw limit", config.CannonYawLimit},
		{"cannon pitch limit", config.CannonPitchLimit},
		{"cannon fire tolerance", config.CannonFireTolerance},
		{"minimum altitude", config.MinimumAltitude},
		{"terrain look-ahead", config.TerrainLookAhead},
		{"guidance strength", config.GuidanceStrength},
	}
	for _, value := range positive {
		if err := validatePositive(value.name, value.value); err != nil {
			return err
		}
	}
	if config.MaxForward < config.CruiseSpeed {
		return fmt.Errorf("maximum forward speed must not be below cruise speed")
	}
	if config.InitialAttackers < 0 || config.MaxAttackers < config.InitialAttackers {
		return fmt.Errorf("attacker counts are invalid")
	}
	if config.MaxActiveCannons < 0 {
		return fmt.Errorf("maximum active cannons cannot be negative")
	}
	if config.CannonFireMaxGap < config.CannonFireMinGap {
		return fmt.Errorf("cannon fire gap range is invalid")
	}
	if config.CannonYawLimit > math.Pi || config.CannonPitchLimit > math.Pi/2 || config.CannonFireTolerance > math.Pi/2 {
		return fmt.Errorf("cannon traverse angle is invalid")
	}
	if err := validateNonNegative("cannon aim error", config.CannonAimError); err != nil {
		return err
	}
	return nil
}

func validatePursuit(config control.PursuitConfig) error {
	if err := validateNonNegative("minimum speed", config.MinSpeed); err != nil {
		return err
	}
	if err := validateNonNegative("maximum speed", config.MaxSpeed); err != nil {
		return err
	}
	if config.MaxSpeed < config.MinSpeed {
		return fmt.Errorf("speed range is invalid")
	}
	positive := []struct {
		name  string
		value float64
	}{
		{"acceleration", config.Acceleration},
		{"turn acceleration", config.TurnAcceleration},
		{"turn gain", config.TurnGain},
		{"maximum yaw rate", config.MaxYawRate},
		{"maximum pitch rate", config.MaxPitchRate},
		{"maximum roll rate", config.MaxRollRate},
		{"wander interval", config.WanderInterval},
		{"speed change interval", config.SpeedChangeInterval},
		{"speed blend rate", config.SpeedBlendRate},
		{"avoidance horizon", config.AvoidanceHorizon},
		{"avoidance slowdown", config.AvoidanceSlowdown},
		{"attack range", config.AttackRange},
	}
	for _, value := range positive {
		if err := validatePositive(value.name, value.value); err != nil {
			return err
		}
	}
	nonNegative := []struct {
		name  string
		value float64
	}{
		{"preferred distance", config.PreferredDistance},
		{"approach gain", config.ApproachGain},
		{"wander strength", config.WanderStrength},
		{"scatter radius", config.ScatterRadius},
		{"speed variation", config.SpeedVariation},
		{"avoidance margin", config.AvoidanceMargin},
		{"avoidance cooldown", config.AvoidanceCooldown},
	}
	for _, value := range nonNegative {
		if err := validateNonNegative(value.name, value.value); err != nil {
			return err
		}
	}
	ranges := []struct {
		name    string
		minimum float64
		maximum float64
	}{
		{"excursion gap", config.ExcursionMinGap, config.ExcursionMaxGap},
		{"excursion duration", config.ExcursionMinTime, config.ExcursionMaxTime},
		{"avoidance duration", config.AvoidanceMinTime, config.AvoidanceMaxTime},
		{"attack gap", config.AttackMinGap, config.AttackMaxGap},
		{"attack duration", config.AttackMinTime, config.AttackMaxTime},
		{"attack radius", config.AttackMinRadius, config.AttackMaxRadius},
		{"attack fire gap", config.AttackFireMinGap, config.AttackFireMaxGap},
	}
	for _, value := range ranges {
		if err := validateNonNegative(value.name+" minimum", value.minimum); err != nil {
			return err
		}
		if err := validateNonNegative(value.name+" maximum", value.maximum); err != nil {
			return err
		}
		if value.maximum < value.minimum {
			return fmt.Errorf("%s range is invalid", value.name)
		}
	}
	return nil
}

func validatePositive(name string, value float64) error {
	if value <= 0 || math.IsNaN(value) || math.IsInf(value, 0) {
		return fmt.Errorf("%s must be a finite positive value", name)
	}
	return nil
}

func validateNonNegative(name string, value float64) error {
	if value < 0 || math.IsNaN(value) || math.IsInf(value, 0) {
		return fmt.Errorf("%s must be a finite non-negative value", name)
	}
	return nil
}
