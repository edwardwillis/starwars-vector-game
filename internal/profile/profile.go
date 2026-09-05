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
)

const (
	CadetName     = "builtin/cadet"
	PilotName     = "builtin/pilot"
	AceName       = "builtin/ace"
	NightmareName = "builtin/nightmare"
)

type SimulationConfig struct {
	TickSeconds               float64
	MotionScale               float64
	DisintegrationTime        float64
	PlayerDestructionViewTime float64
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
}

type TargetingConfig struct {
	AimRadius      float64
	AimConvergence float64
}

type CombatConfig struct {
	Laser         combat.LaserConfig
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
	InitialPose     kinematics.Pose
	AutopilotMotion kinematics.Motion
	Flight          control.ManualConfig
	Shield          ShieldConfig
}

type DifficultyConfig struct {
	Name string
}

type SwarmConfig struct {
	Object           string
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
	pursuit := control.DefaultPursuitConfig()
	pursuit.PreferredDistance = 10.0
	pursuit.MinSpeed = 2.80
	pursuit.MaxSpeed = manual.MaxForward * (1200.0 / 1050.0)
	pursuit.Acceleration = 2.20
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
		},
		Display: DisplayConfig{
			ZoomSpeed:               1.5,
			ControlsDisplayDuration: 10.0,
			RenderingProfile:        "builtin/arcade",
			VerticalFOV:             math.Pi / 3,
			NearPlane:               0.1,
			FarPlane:                1000,
		},
		Input: InputConfig{
			MouseDeadzone:    0.08,
			MouseSensitivity: 1.25,
		},
		Starfield: StarfieldConfig{Count: 500, Radius: 40, Seed: 42},
		Targeting: TargetingConfig{AimRadius: 190, AimConvergence: 30},
		Combat: CombatConfig{
			Laser:         combat.DefaultLaserConfig(),
			FireInterval:  0.12,
			FireWindow:    1.5,
			MaxFireEvents: 3,
			BeamTime:      0.08,
		},
		Player: PlayerConfig{
			Object: "builtin/x-wing",
			InitialPose: kinematics.Pose{
				Position:    math3d.Vec3{Z: -450},
				Orientation: math3d.QuaternionFromYawPitchRoll(0, 0, 0),
			},
			AutopilotMotion: kinematics.Motion{
				Speed:    manual.MaxForward,
				YawRate:  0.22,
				RollRate: 0.16,
			},
			Flight: manual,
			Shield: ShieldConfig{
				Maximum:          8,
				LaserDamage:      1,
				CollisionDamage:  3,
				RechargeInterval: 20,
			},
		},
		Swarm: SwarmConfig{
			Object:     "builtin/tie-fighter",
			Count:      5,
			Controller: control.PursuitName,
			Flight: control.Limits{
				Acceleration: pursuit.Acceleration,
				MaxForward:   pursuit.MaxSpeed,
				MaxReverse:   pursuit.MaxSpeed,
				MaxYawRate:   pursuit.MaxYawRate,
				MaxPitchRate: pursuit.MaxPitchRate,
				MaxRollRate:  pursuit.MaxRollRate,
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
	syncSwarmFlight(&profile)
	return profile
}

func syncSwarmFlight(profile *GameProfile) {
	config := profile.Swarm.Pursuit
	profile.Swarm.Flight = control.Limits{
		Acceleration: config.Acceleration,
		MaxForward:   config.MaxSpeed,
		MaxReverse:   config.MaxSpeed,
		MaxYawRate:   config.MaxYawRate,
		MaxPitchRate: config.MaxPitchRate,
		MaxRollRate:  config.MaxRollRate,
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
	if profile.Swarm.Object == "" {
		return fmt.Errorf("swarm object definition is required")
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
	if err := validatePositive("aim radius", profile.Targeting.AimRadius); err != nil {
		return err
	}
	if err := validatePositive("aim convergence", profile.Targeting.AimConvergence); err != nil {
		return err
	}
	if err := profile.Combat.Laser.Validate(); err != nil {
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
