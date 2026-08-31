package control

import (
	"math"
	"testing"

	"github.com/edwardwillis/starwars-vector-game/internal/kinematics"
	"github.com/edwardwillis/starwars-vector-game/internal/math3d"
	"github.com/edwardwillis/starwars-vector-game/internal/scene"
)

func TestPursuitTurnsTowardTargetAndAccelerates(t *testing.T) {
	config := DefaultPursuitConfig()
	config.WanderStrength = 0
	controller := NewPursuit(1, config)
	self := scene.Object{Pose: kinematics.Pose{Orientation: math3d.IdentityQuaternion()}}
	target := scene.Object{Pose: kinematics.Pose{Position: math3d.Vec3{X: 10, Y: 2, Z: 10}}}
	motion := controller.Step(self, target, 1)
	if motion.YawRate <= 0 || motion.PitchRate >= 0 {
		t.Fatalf("pursuit did not turn toward upper-right target: %+v", motion)
	}
	if motion.Speed <= 0 {
		t.Fatalf("pursuit did not accelerate toward distant target: %+v", motion)
	}
}

func TestPursuitIsDeterministicForSeed(t *testing.T) {
	first := NewPursuit(99, DefaultPursuitConfig())
	second := NewPursuit(99, DefaultPursuitConfig())
	self := scene.Object{Pose: kinematics.Pose{Orientation: math3d.IdentityQuaternion()}}
	target := scene.Object{Pose: kinematics.Pose{Position: math3d.Vec3{Z: 8}}}
	for tick := 0; tick < 180; tick++ {
		firstMotion := first.Step(self, target, 1.0/60)
		secondMotion := second.Step(self, target, 1.0/60)
		if firstMotion != secondMotion {
			t.Fatalf("same seed diverged at tick %d: %+v != %+v", tick, firstMotion, secondMotion)
		}
		self.Motion = firstMotion
	}
}

func TestPursuitRatesRemainBounded(t *testing.T) {
	config := DefaultPursuitConfig()
	controller := NewPursuit(7, config)
	self := scene.Object{Pose: kinematics.Pose{Orientation: math3d.IdentityQuaternion()}}
	target := scene.Object{Pose: kinematics.Pose{Position: math3d.Vec3{X: -100, Y: -100, Z: -100}}}
	motion := controller.Step(self, target, 1)
	if motion.YawRate < -config.MaxYawRate || motion.YawRate > config.MaxYawRate ||
		motion.PitchRate < -config.MaxPitchRate || motion.PitchRate > config.MaxPitchRate ||
		motion.RollRate < -config.MaxRollRate || motion.RollRate > config.MaxRollRate {
		t.Fatalf("pursuit exceeded configured rate: %+v", motion)
	}
}

func TestPursuitFliesExcursionThenReturns(t *testing.T) {
	config := DefaultPursuitConfig()
	config.WanderStrength = 0
	config.ScatterRadius = 0
	config.ExcursionMinGap = 0.01
	config.ExcursionMaxGap = 0.01
	config.ExcursionMinTime = 0.5
	config.ExcursionMaxTime = 0.5
	controller := NewPursuit(17, config)
	self := scene.Object{Pose: kinematics.Pose{
		Position:    math3d.Vec3{Z: 2},
		Orientation: math3d.IdentityQuaternion(),
	}}
	target := scene.Object{Pose: kinematics.Pose{Orientation: math3d.IdentityQuaternion()}}
	controller.Step(self, target, 0.02)
	if !controller.excursion || controller.excursionTime <= 0 {
		t.Fatal("pursuit did not enter its scheduled excursion")
	}
	controller.Step(self, target, 0.6)
	if controller.excursion {
		t.Fatal("pursuit did not return after its excursion duration")
	}
}

func TestPursuitSeedsProduceDifferentReturnOffsets(t *testing.T) {
	first := NewPursuit(1, DefaultPursuitConfig())
	second := NewPursuit(2, DefaultPursuitConfig())
	if first.targetOffset == second.targetOffset {
		t.Fatal("different pursuit seeds produced the same return offset")
	}
}

func TestPursuitChangesSpeedTargetSmoothly(t *testing.T) {
	config := DefaultPursuitConfig()
	config.SpeedVariation = 0.4
	config.SpeedChangeInterval = 0.1
	config.SpeedBlendRate = 1
	config.ExcursionMinGap = 100
	config.ExcursionMaxGap = 100
	controller := NewPursuit(23, config)
	self := scene.Object{
		Pose:   kinematics.Pose{Orientation: math3d.IdentityQuaternion()},
		Motion: kinematics.Motion{Speed: 0.5},
	}
	target := scene.Object{Pose: kinematics.Pose{Position: math3d.Vec3{Z: 8}}}
	beforeTarget := controller.targetSpeedOffset
	motion := controller.Step(self, target, 0.11)
	if controller.targetSpeedOffset == beforeTarget {
		t.Fatal("speed target did not change after its scheduled interval")
	}
	maximumChange := config.Acceleration * 0.11
	if math.Abs(motion.Speed-self.Motion.Speed) > maximumChange+1e-9 {
		t.Fatalf("speed changed by %v, exceeding acceleration limit %v", motion.Speed-self.Motion.Speed, maximumChange)
	}
}
