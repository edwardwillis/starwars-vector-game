package control

import (
	"math"
	"testing"

	"github.com/edwardwillis/starwars-vector-game/internal/kinematics"
	"github.com/edwardwillis/starwars-vector-game/internal/math3d"
)

func TestApplyChangesSpeedGradually(t *testing.T) {
	config := DefaultManualConfig()
	motion := Apply(kinematics.Motion{}, Intent{Throttle: 1}, config, 1)
	if motion.Speed != config.Acceleration {
		t.Fatalf("speed is %v, want %v", motion.Speed, config.Acceleration)
	}

	motion = Apply(motion, Intent{Throttle: -1}, config, 1)
	if motion.Speed != 0 {
		t.Fatalf("speed is %v after opposing throttle, want 0", motion.Speed)
	}
}

func TestAutoLevelRollRateCorrectsBankWithoutChangingHeading(t *testing.T) {
	config := AutoLevelConfig{Enabled: true, CorrectionGain: 2, MaxRollRate: 1, AngleDeadzone: 0.01}
	banked := math3d.QuaternionFromYawPitchRoll(0.4, -0.2, 0.5)
	rate := AutoLevelRollRate(banked, math3d.Vec3{Y: 1}, config)
	if rate >= 0 || math.Abs(rate+1) > 1e-9 {
		t.Fatalf("positive bank correction=%v, want capped negative roll", rate)
	}
	pose := kinematics.Pose{Orientation: banked}
	beforeForward := pose.Forward()
	pose = kinematics.Integrate(pose, kinematics.Motion{RollRate: rate}, 0.1)
	if beforeForward.Dot(pose.Forward()) < 0.999999 {
		t.Fatalf("roll correction changed heading: before=%+v after=%+v", beforeForward, pose.Forward())
	}
}

func TestAutoLevelRollRateHonorsDeadzoneAndUndefinedHorizon(t *testing.T) {
	config := AutoLevelConfig{Enabled: true, CorrectionGain: 2, MaxRollRate: 1, AngleDeadzone: 0.05}
	if got := AutoLevelRollRate(math3d.QuaternionFromYawPitchRoll(0, 0, 0.02), math3d.Vec3{Y: 1}, config); got != 0 {
		t.Fatalf("small bank produced correction %v", got)
	}
	vertical := math3d.QuaternionFromYawPitchRoll(0, math.Pi/2, 0)
	if got := AutoLevelRollRate(vertical, math3d.Vec3{Y: 1}, config); got != 0 {
		t.Fatalf("undefined vertical horizon produced correction %v", got)
	}
	config.Enabled = false
	if got := AutoLevelRollRate(math3d.QuaternionFromYawPitchRoll(0, 0, 0.5), math3d.Vec3{Y: 1}, config); got != 0 {
		t.Fatalf("disabled assist produced correction %v", got)
	}
}

func TestApplySupportsReverseAndSpeedLimits(t *testing.T) {
	config := DefaultManualConfig()
	motion := Apply(kinematics.Motion{}, Intent{Throttle: -1}, config, 100)
	if motion.Speed != -config.MaxReverse {
		t.Fatalf("reverse speed is %v, want %v", motion.Speed, -config.MaxReverse)
	}
	motion = Apply(motion, Intent{Throttle: 1}, config, 100)
	if motion.Speed != config.MaxForward {
		t.Fatalf("forward speed is %v, want %v", motion.Speed, config.MaxForward)
	}
}

func TestApplyMapsAndReleasesAngularControls(t *testing.T) {
	config := DefaultManualConfig()
	motion := Apply(kinematics.Motion{}, Intent{Yaw: 1, Pitch: -1, Roll: 0.5}, config, 1)
	if motion.YawRate != config.MaxYawRate || motion.PitchRate != -config.MaxPitchRate || motion.RollRate != config.MaxRollRate/2 {
		t.Fatalf("unexpected angular rates: %+v", motion)
	}
	motion = Apply(motion, Intent{}, config, 1)
	if motion.YawRate != 0 || motion.PitchRate != 0 || motion.RollRate != 0 {
		t.Fatalf("released controls left angular rates: %+v", motion)
	}
}

func TestApplyWithLimitsRampsAutonomousAngularRates(t *testing.T) {
	limits := Limits{
		AngularAcceleration: 2,
		MaxYawRate:          1,
		MaxPitchRate:        1,
		MaxRollRate:         1,
	}
	motion := ApplyWithLimits(kinematics.Motion{}, Intent{Yaw: 1, Pitch: -1, Roll: 1}, limits, 0.1)
	if motion.YawRate != 0.2 || motion.PitchRate != -0.2 || motion.RollRate != 0.2 {
		t.Fatalf("angular rates jumped instead of ramping: %+v", motion)
	}
	motion = ApplyWithLimits(motion, Intent{Yaw: -1, Pitch: 1, Roll: -1}, limits, 0.1)
	if motion.YawRate != 0 || motion.PitchRate != 0 || motion.RollRate != 0 {
		t.Fatalf("angular rates reversed too quickly: %+v", motion)
	}
}

func TestStopTakesPrecedence(t *testing.T) {
	motion := Apply(kinematics.Motion{Speed: 1}, Intent{Throttle: 1, Stop: true}, DefaultManualConfig(), 1)
	if motion.Speed != 0 {
		t.Fatalf("stop produced speed %v, want 0", motion.Speed)
	}
}
