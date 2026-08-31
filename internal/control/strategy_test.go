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
	motion := controller.Step(testContext(self, target, 1))
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
		firstMotion := first.Step(testContext(self, target, 1.0/60))
		secondMotion := second.Step(testContext(self, target, 1.0/60))
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
	motion := controller.Step(testContext(self, target, 1))
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
	controller.Step(testContext(self, target, 0.02))
	if !controller.excursion || controller.excursionTime <= 0 {
		t.Fatal("pursuit did not enter its scheduled excursion")
	}
	controller.Step(testContext(self, target, 0.6))
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
	motion := controller.Step(testContext(self, target, 0.11))
	if controller.targetSpeedOffset == beforeTarget {
		t.Fatal("speed target did not change after its scheduled interval")
	}
	maximumChange := config.Acceleration * 0.11
	if math.Abs(motion.Speed-self.Motion.Speed) > maximumChange+1e-9 {
		t.Fatalf("speed changed by %v, exceeding acceleration limit %v", motion.Speed-self.Motion.Speed, maximumChange)
	}
}

func TestPursuitAttackRunSelectsRadiusAndRequestsFire(t *testing.T) {
	config := DefaultPursuitConfig()
	config.AttackMinGap = 0
	config.AttackMaxGap = 0
	config.AttackMinTime = 2
	config.AttackMaxTime = 2
	config.AttackMinRadius = 5
	config.AttackMaxRadius = 14
	config.AttackFireMinGap = 0
	config.AttackFireMaxGap = 0
	config.AttackRange = 30
	config.AttackAimDot = 0.5
	controller := NewPursuit(71, config)
	self := scene.Object{Pose: kinematics.Pose{Orientation: math3d.IdentityQuaternion()}}
	target := scene.Object{Pose: kinematics.Pose{Position: math3d.Vec3{Z: 10}}}

	controller.Step(testContext(self, target, 0.01))
	if !controller.attacking || controller.attackRadius < config.AttackMinRadius || controller.attackRadius > config.AttackMaxRadius {
		t.Fatalf("controller did not begin a valid attack arc: attacking=%v radius=%v", controller.attacking, controller.attackRadius)
	}
	controller.Step(testContext(self, target, 0.01))
	if !controller.AttackIntent() {
		t.Fatal("aligned controller did not request fire during its attack run")
	}
}

func TestPursuitAttackCanFireWhileFlyingAnArc(t *testing.T) {
	config := DefaultPursuitConfig()
	config.AttackMinGap = 0
	config.AttackMaxGap = 0
	config.AttackMinTime = 2
	config.AttackMaxTime = 2
	config.AttackMinRadius = 8
	config.AttackMaxRadius = 8
	config.AttackFireMinGap = 0
	config.AttackFireMaxGap = 0
	config.AttackRange = 30
	config.AttackAimDot = -1
	controller := NewPursuit(73, config)
	self := scene.Object{Pose: kinematics.Pose{Orientation: math3d.IdentityQuaternion()}}
	target := scene.Object{Pose: kinematics.Pose{Position: math3d.Vec3{Z: 10}}}
	controller.Step(testContext(self, target, 0.01))
	controller.Step(testContext(self, target, 0.01))
	if !controller.AttackIntent() {
		t.Fatal("attack arc did not request fire when target was in range")
	}
}

func TestIndependentPursuitAttackWindowsCanOverlap(t *testing.T) {
	config := DefaultPursuitConfig()
	config.AttackMinGap = 1.5
	config.AttackMaxGap = 6
	config.AttackMinTime = 3
	config.AttackMaxTime = 6
	controllers := []*Pursuit{
		NewPursuit(101, config), NewPursuit(102, config), NewPursuit(103, config),
		NewPursuit(104, config), NewPursuit(105, config),
	}
	self := scene.Object{Pose: kinematics.Pose{Orientation: math3d.IdentityQuaternion()}}
	target := scene.Object{Pose: kinematics.Pose{Position: math3d.Vec3{Z: 10}}}
	maximumConcurrent := 0
	for range 1200 {
		concurrent := 0
		for _, controller := range controllers {
			controller.Step(testContext(self, target, 1.0/60))
			if controller.attacking {
				concurrent++
			}
		}
		maximumConcurrent = max(maximumConcurrent, concurrent)
	}
	if maximumConcurrent < 2 {
		t.Fatalf("independent attack schedules never overlapped: maximum=%d", maximumConcurrent)
	}
	if controllers[0].attackRadius == controllers[1].attackRadius {
		t.Fatal("independent attackers selected identical arc radii")
	}
}

func TestPursuitPredictsHeadOnCollisionAndAvoids(t *testing.T) {
	config := DefaultPursuitConfig()
	controller := NewPursuit(41, config)
	self := scene.Object{
		ID:              1,
		Pose:            kinematics.Pose{Orientation: math3d.IdentityQuaternion()},
		Motion:          kinematics.Motion{Speed: 2},
		CollisionRole:   scene.CollisionSolid,
		CollisionRadius: 1,
	}
	threat := scene.Object{
		ID: 2,
		Pose: kinematics.Pose{
			Position:    math3d.Vec3{Z: 8},
			Orientation: math3d.QuaternionFromYawPitchRoll(math.Pi, 0, 0),
		},
		Motion:          kinematics.Motion{Speed: 2},
		CollisionRole:   scene.CollisionSolid,
		CollisionRadius: 1,
	}
	motion := controller.Step(Context{
		Self: self, Target: threat, Nearby: []scene.Object{threat}, Seconds: 1.0 / 60, MotionScale: 2,
	})
	if controller.avoidanceObject != threat.ID || controller.avoidanceTime <= 0 {
		t.Fatal("head-on approach did not start avoidance")
	}
	if motion.YawRate == 0 || motion.RollRate == 0 {
		t.Fatalf("avoidance did not add yaw and roll: %+v", motion)
	}
}

func TestPursuitDoesNotAvoidDivergingObject(t *testing.T) {
	controller := NewPursuit(43, DefaultPursuitConfig())
	self := scene.Object{
		ID: 1, Pose: kinematics.Pose{Orientation: math3d.IdentityQuaternion()},
		Motion: kinematics.Motion{Speed: 2}, CollisionRole: scene.CollisionSolid, CollisionRadius: 1,
	}
	departing := scene.Object{
		ID: 2, Pose: kinematics.Pose{Position: math3d.Vec3{Z: 6}, Orientation: math3d.IdentityQuaternion()},
		Motion: kinematics.Motion{Speed: 3}, CollisionRole: scene.CollisionSolid, CollisionRadius: 1,
	}
	controller.Step(Context{
		Self: self, Target: departing, Nearby: []scene.Object{departing}, Seconds: 1.0 / 60, MotionScale: 2,
	})
	if controller.avoidanceTime > 0 {
		t.Fatal("controller avoided an object that was moving away")
	}
}

func TestPursuitUsesProximityFallbackForCurvedTrajectories(t *testing.T) {
	config := DefaultPursuitConfig()
	config.AvoidanceMargin = 6
	controller := NewPursuit(45, config)
	self := scene.Object{
		ID: 1, Pose: kinematics.Pose{Orientation: math3d.IdentityQuaternion()},
		Motion: kinematics.Motion{Speed: 2}, CollisionRole: scene.CollisionSolid, CollisionRadius: 1.8,
	}
	nearby := scene.Object{
		ID: 2, Pose: kinematics.Pose{Position: math3d.Vec3{X: 8}, Orientation: math3d.IdentityQuaternion()},
		Motion: kinematics.Motion{Speed: 2}, CollisionRole: scene.CollisionSolid, CollisionRadius: 1.8,
	}
	controller.Step(Context{Self: self, Target: nearby, Nearby: []scene.Object{nearby}, Seconds: 0.02, MotionScale: 2})
	if controller.avoidanceObject != nearby.ID || controller.avoidanceTime <= 0 {
		t.Fatal("nearby curved-trajectory risk did not trigger proximity avoidance")
	}
}

func TestPursuitAvoidancePersistsAfterThreatClears(t *testing.T) {
	config := DefaultPursuitConfig()
	config.AvoidanceMinTime = 0.7
	config.AvoidanceMaxTime = 0.7
	controller := NewPursuit(47, config)
	self := scene.Object{
		ID: 1, Pose: kinematics.Pose{Orientation: math3d.IdentityQuaternion()},
		Motion: kinematics.Motion{Speed: 2}, CollisionRole: scene.CollisionSolid, CollisionRadius: 1,
	}
	threat := scene.Object{
		ID:     2,
		Pose:   kinematics.Pose{Position: math3d.Vec3{Z: 8}, Orientation: math3d.QuaternionFromYawPitchRoll(math.Pi, 0, 0)},
		Motion: kinematics.Motion{Speed: 2}, CollisionRole: scene.CollisionSolid, CollisionRadius: 1,
	}
	controller.Step(Context{Self: self, Target: threat, Nearby: []scene.Object{threat}, Seconds: 0.02, MotionScale: 2})
	controller.Step(Context{Self: self, Target: threat, Seconds: 0.2, MotionScale: 2})
	if controller.avoidanceTime <= 0 || controller.avoidanceObject != threat.ID {
		t.Fatal("avoidance did not persist through its hysteresis interval")
	}
}

func TestHeadOnPairChoosesCompatibleLocalYawDirection(t *testing.T) {
	config := DefaultPursuitConfig()
	firstController := NewPursuit(51, config)
	secondController := NewPursuit(52, config)
	first := scene.Object{
		ID: 1, Pose: kinematics.Pose{Orientation: math3d.IdentityQuaternion()},
		Motion: kinematics.Motion{Speed: 2}, CollisionRole: scene.CollisionSolid, CollisionRadius: 1,
	}
	second := scene.Object{
		ID:     2,
		Pose:   kinematics.Pose{Position: math3d.Vec3{Z: 8}, Orientation: math3d.QuaternionFromYawPitchRoll(math.Pi, 0, 0)},
		Motion: kinematics.Motion{Speed: 2}, CollisionRole: scene.CollisionSolid, CollisionRadius: 1,
	}
	firstMotion := firstController.Step(Context{Self: first, Target: second, Nearby: []scene.Object{second}, Seconds: 0.02, MotionScale: 2})
	secondMotion := secondController.Step(Context{Self: second, Target: first, Nearby: []scene.Object{first}, Seconds: 0.02, MotionScale: 2})
	if firstMotion.YawRate*secondMotion.YawRate <= 0 {
		t.Fatalf("head-on pair chose incompatible local yaw rates %v and %v", firstMotion.YawRate, secondMotion.YawRate)
	}
}

func TestAvoidancePreventsIntegratedHeadOnCollision(t *testing.T) {
	config := DefaultPursuitConfig()
	config.WanderStrength = 0
	config.ScatterRadius = 0
	config.ExcursionMinGap = 100
	config.ExcursionMaxGap = 100
	config.AvoidanceHorizon = 2.75
	config.AvoidanceMargin = 3.5
	config.AvoidanceMinTime = 1.4
	config.AvoidanceMaxTime = 2.0
	config.AvoidanceCooldown = 0.08
	config.AvoidanceSlowdown = 0.35
	firstController := NewPursuit(61, config)
	secondController := NewPursuit(62, config)
	first := scene.Object{
		ID: 1, Pose: kinematics.Pose{Position: math3d.Vec3{Z: -8}, Orientation: math3d.IdentityQuaternion()},
		Motion: kinematics.Motion{Speed: 3}, CollisionRole: scene.CollisionSolid, CollisionRadius: 1.8,
	}
	second := scene.Object{
		ID:     2,
		Pose:   kinematics.Pose{Position: math3d.Vec3{Z: 8}, Orientation: math3d.QuaternionFromYawPitchRoll(math.Pi, 0, 0)},
		Motion: kinematics.Motion{Speed: 3}, CollisionRole: scene.CollisionSolid, CollisionRadius: 1.8,
	}
	const (
		seconds     = 1.0 / 60.0
		motionScale = 2.0
	)
	minimumDistance := math.Inf(1)
	for range 180 {
		firstMotion := firstController.Step(Context{Self: first, Target: second, Nearby: []scene.Object{second}, Seconds: seconds, MotionScale: motionScale})
		secondMotion := secondController.Step(Context{Self: second, Target: first, Nearby: []scene.Object{first}, Seconds: seconds, MotionScale: motionScale})
		first.Motion, second.Motion = firstMotion, secondMotion
		first.Pose = kinematics.Integrate(first.Pose, first.Motion, seconds*motionScale)
		second.Pose = kinematics.Integrate(second.Pose, second.Motion, seconds*motionScale)
		distance := first.Pose.Position.Sub(second.Pose.Position).Length()
		minimumDistance = min(minimumDistance, distance)
	}
	contactDistance := first.CollisionRadius + second.CollisionRadius
	if minimumDistance <= contactDistance {
		t.Fatalf("head-on fighters approached to %v, collision distance is %v", minimumDistance, contactDistance)
	}
}

func testContext(self, target scene.Object, seconds float64) Context {
	return Context{Self: self, Target: target, Seconds: seconds, MotionScale: 1}
}
