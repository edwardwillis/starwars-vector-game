// Package control converts user or autonomous decisions into kinematic motion.
package control

import "github.com/edwardwillis/starwars-vector-game/internal/kinematics"

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

// Limits are the movement constraints applied to any controller decision.
// Keeping these separate from a strategy lets manual, scripted, and external
// controllers share the same authoritative flight rules.
type Limits struct {
	Acceleration float64
	MaxForward   float64
	MaxReverse   float64
	MaxYawRate   float64
	MaxPitchRate float64
	MaxRollRate  float64
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
	motion.YawRate = intent.Yaw * limits.MaxYawRate
	motion.PitchRate = intent.Pitch * limits.MaxPitchRate
	motion.RollRate = intent.Roll * limits.MaxRollRate
	return motion
}

func clamp(value, minimum, maximum float64) float64 {
	return min(max(value, minimum), maximum)
}
