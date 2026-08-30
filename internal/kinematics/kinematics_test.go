package kinematics

import (
	"math"
	"testing"

	"github.com/edwardwillis/starwars-vector-game/internal/math3d"
)

func TestSignedSpeedMovesAlongForwardAxis(t *testing.T) {
	pose := Pose{Orientation: math3d.IdentityQuaternion()}
	forward := Integrate(pose, Motion{Speed: 2}, 0.5)
	backward := Integrate(pose, Motion{Speed: -2}, 0.5)
	assertVec3(t, forward.Position, math3d.Vec3{Z: 1})
	assertVec3(t, backward.Position, math3d.Vec3{Z: -1})
}

func TestYawRedirectsMovement(t *testing.T) {
	pose := Pose{Orientation: math3d.IdentityQuaternion()}
	result := Integrate(pose, Motion{Speed: 1, YawRate: math.Pi / 2}, 1)
	assertVec3(t, result.Position, math3d.Vec3{X: 1})
	assertVec3(t, result.Forward(), math3d.Vec3{X: 1})
}

func TestRollAtRestChangesOrientationNotPosition(t *testing.T) {
	pose := Pose{Orientation: math3d.IdentityQuaternion()}
	result := Integrate(pose, Motion{RollRate: math.Pi / 2}, 1)
	assertVec3(t, result.Position, math3d.Vec3{})
	assertVec3(t, result.Orientation.Rotate(math3d.Vec3{Y: 1}), math3d.Vec3{X: -1})
}

func TestFixedStepSubdivisionIsStable(t *testing.T) {
	initial := Pose{Orientation: math3d.IdentityQuaternion()}
	motion := Motion{Speed: 1, YawRate: 0.2, PitchRate: 0.1, RollRate: 0.05}
	oneStep := Integrate(initial, motion, 1)
	manySteps := initial
	for range 100 {
		manySteps = Integrate(manySteps, motion, 0.01)
	}
	if oneStep.Position.Sub(manySteps.Position).Length() > 0.12 {
		t.Fatalf("subdivided integration diverged: one=%+v many=%+v", oneStep.Position, manySteps.Position)
	}
	if math.Abs(manySteps.Orientation.Length()-1) > 1e-9 {
		t.Fatalf("orientation length is %v, want 1", manySteps.Orientation.Length())
	}
}

func TestComposeAndViewMatrix(t *testing.T) {
	parent := Pose{
		Position:    math3d.Vec3{X: 10},
		Orientation: math3d.QuaternionFromAxisAngle(math3d.Vec3{Y: 1}, math.Pi/2),
	}
	local := Pose{Position: math3d.Vec3{Z: 2}, Orientation: math3d.IdentityQuaternion()}
	world := Compose(parent, local)
	assertVec3(t, world.Position, math3d.Vec3{X: 12})

	pointInFrontOfCamera := world.Position.Add(world.Forward().Scale(3))
	cameraPoint := world.ViewMatrix().TransformPoint(pointInFrontOfCamera)
	assertVec3(t, cameraPoint, math3d.Vec3{Z: 3})
}

func assertVec3(t *testing.T, got, want math3d.Vec3) {
	t.Helper()
	if got.Sub(want).Length() > 1e-9 {
		t.Fatalf("got %+v, want %+v", got, want)
	}
}
