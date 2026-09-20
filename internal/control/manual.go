// Package control converts user or autonomous decisions into kinematic motion.
package control

import (
	"math"

	"github.com/edwardwillis/starwars-vector-game/internal/kinematics"
	"github.com/edwardwillis/starwars-vector-game/internal/math3d"
)

// Intent contains normalized flight controls in the range [-1, 1]. Stop takes
// precedence over throttle. Controllers produce intentions, never poses.
type Intent struct {
	Throttle float64
	Yaw      float64
	Pitch    float64
	Roll     float64
	Stop     bool
}

type ManualConfig struct {
	Acceleration float64
	MaxForward   float64
	MaxReverse   float64
	MaxYawRate   float64
	MaxPitchRate float64
	MaxRollRate  float64
}

// AutoLevelConfig describes optional horizon assistance. It is kept separate
// from ordinary flight limits because environments decide whether they expose
// a meaningful local level reference.
type AutoLevelConfig struct {
	Enabled        bool
	CorrectionGain float64
	MaxRollRate    float64
	AngleDeadzone  float64
	TurnDeadzone   float64
}

// Limits are the movement constraints applied to any controller decision.
// Keeping these separate from a strategy lets manual, scripted, and external
// controllers share the same authoritative flight rules.
type Limits struct {
	Acceleration float64
	// AngularAcceleration limits how quickly an object's commanded yaw, pitch,
	// and roll rates can change. Zero preserves direct/manual control mapping.
	AngularAcceleration float64
	MaxForward          float64
	MaxReverse          float64
	MaxYawRate          float64
	MaxPitchRate        float64
	MaxRollRate         float64
}

func DefaultManualConfig() ManualConfig {
	return ManualConfig{
		Acceleration: 2.0,
		MaxForward:   3.6,
		MaxReverse:   2.0,
		MaxYawRate:   1.5,
		MaxPitchRate: 1.25,
		MaxRollRate:  1.8,
	}
}

// Apply converts an intent into motion for one simulation tick. Speed persists
// and changes gradually; angular rates follow the current controls and return to
// zero when those controls are released.
func Apply(motion kinematics.Motion, intent Intent, config ManualConfig, seconds float64) kinematics.Motion {
	return ApplyWithLimits(motion, intent, Limits{
		Acceleration: config.Acceleration,
		MaxForward:   config.MaxForward,
		MaxReverse:   config.MaxReverse,
		MaxYawRate:   config.MaxYawRate,
		MaxPitchRate: config.MaxPitchRate,
		MaxRollRate:  config.MaxRollRate,
	}, seconds)
}

// ApplyWithLimits validates and applies any controller intent using shared
// authoritative movement limits.
func ApplyWithLimits(motion kinematics.Motion, intent Intent, limits Limits, seconds float64) kinematics.Motion {
	intent.Throttle = clamp(intent.Throttle, -1, 1)
	intent.Yaw = clamp(intent.Yaw, -1, 1)
	intent.Pitch = clamp(intent.Pitch, -1, 1)
	intent.Roll = clamp(intent.Roll, -1, 1)

	if intent.Stop {
		motion.Speed = 0
	} else if seconds > 0 {
		motion.Speed += intent.Throttle * limits.Acceleration * seconds
		motion.Speed = clamp(motion.Speed, -limits.MaxReverse, limits.MaxForward)
	}
	desiredYaw := intent.Yaw * limits.MaxYawRate
	desiredPitch := intent.Pitch * limits.MaxPitchRate
	desiredRoll := intent.Roll * limits.MaxRollRate
	if limits.AngularAcceleration > 0 && seconds > 0 {
		maximumDelta := limits.AngularAcceleration * seconds
		motion.YawRate = moveToward(motion.YawRate, desiredYaw, maximumDelta)
		motion.PitchRate = moveToward(motion.PitchRate, desiredPitch, maximumDelta)
		motion.RollRate = moveToward(motion.RollRate, desiredRoll, maximumDelta)
	} else {
		motion.YawRate = desiredYaw
		motion.PitchRate = desiredPitch
		motion.RollRate = desiredRoll
	}
	return motion
}

// AutoLevelRollRate returns the shortest local-roll correction that aligns the
// craft's up vector with referenceUp while preserving its current heading and
// pitch. A vertical craft has no defined horizon roll and receives no command.
func AutoLevelRollRate(orientation math3d.Quaternion, referenceUp math3d.Vec3, config AutoLevelConfig) float64 {
	if !config.Enabled || config.CorrectionGain <= 0 || config.MaxRollRate <= 0 {
		return 0
	}
	orientation = orientation.Normalize()
	forward := orientation.Rotate(kinematics.LocalForward).Normalize()
	currentUp := orientation.Rotate(math3d.Vec3{Y: 1}).Normalize()
	referenceUp = referenceUp.Normalize()
	if forward == (math3d.Vec3{}) || currentUp == (math3d.Vec3{}) || referenceUp == (math3d.Vec3{}) {
		return 0
	}
	desiredUp := referenceUp.Sub(forward.Scale(referenceUp.Dot(forward))).Normalize()
	if desiredUp == (math3d.Vec3{}) {
		return 0
	}
	angle := math.Atan2(forward.Dot(currentUp.Cross(desiredUp)), currentUp.Dot(desiredUp))
	if math.Abs(angle) <= config.AngleDeadzone {
		return 0
	}
	return clamp(angle*config.CorrectionGain, -config.MaxRollRate, config.MaxRollRate)
}

func clamp(value, minimum, maximum float64) float64 {
	return min(max(value, minimum), maximum)
}
