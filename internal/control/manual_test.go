package control

import (
	"testing"

	"github.com/edwardwillis/starwars-vector-game/internal/kinematics"
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

func TestStopTakesPrecedence(t *testing.T) {
	motion := Apply(kinematics.Motion{Speed: 1}, Intent{Throttle: 1, Stop: true}, DefaultManualConfig(), 1)
	if motion.Speed != 0 {
		t.Fatalf("stop produced speed %v, want 0", motion.Speed)
	}
}
