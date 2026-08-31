package combat

import (
	"testing"

	"github.com/edwardwillis/starwars-vector-game/internal/catalog"
	"github.com/edwardwillis/starwars-vector-game/internal/kinematics"
	"github.com/edwardwillis/starwars-vector-game/internal/math3d"
	"github.com/edwardwillis/starwars-vector-game/internal/scene"
)

func TestFireLaserUsesMuzzlePoseAndInheritedSpeed(t *testing.T) {
	shooter := catalog.TwinPanelFighter(1, kinematics.Pose{
		Position:    math3d.Vec3{X: 10},
		Orientation: math3d.IdentityQuaternion(),
	})
	shooter.Motion.Speed = -0.5

	spawn, err := FireLaser(shooter, 2, "muzzle-upper-left")
	if err != nil {
		t.Fatalf("FireLaser returned an error: %v", err)
	}
	wantPose, _ := shooter.Anchor("muzzle-upper-left")
	if spawn.Object.Pose != wantPose {
		t.Fatalf("bolt pose is %+v, want %+v", spawn.Object.Pose, wantPose)
	}
	if spawn.Object.Motion.Speed != LaserSpeed-0.5 {
		t.Fatalf("bolt speed is %v, want %v", spawn.Object.Motion.Speed, LaserSpeed-0.5)
	}
	if spawn.Object.Motion.RollRate != LaserSpinRate {
		t.Fatalf("bolt spin is %v, want %v", spawn.Object.Motion.RollRate, LaserSpinRate)
	}
	if spawn.OwnerID != shooter.ID || spawn.Lifetime != LaserLifetime {
		t.Fatalf("unexpected spawn metadata: %+v", spawn)
	}
}

func TestFireLaserRejectsMissingMuzzle(t *testing.T) {
	shooter := scene.Object{ID: 1}
	if _, err := FireLaser(shooter, 2, "missing"); err == nil {
		t.Fatal("FireLaser accepted a missing muzzle")
	}
}

func TestFireLaserTowardOrientsBoltAtConvergencePoint(t *testing.T) {
	shooter := catalog.TwinPanelFighter(1, kinematics.Pose{Orientation: math3d.IdentityQuaternion()})
	target := math3d.Vec3{X: 4, Y: 2, Z: 20}
	spawn, err := FireLaserToward(shooter, 2, "muzzle-upper-left", target)
	if err != nil {
		t.Fatalf("FireLaserToward returned an error: %v", err)
	}
	want := target.Sub(spawn.Object.Pose.Position).Normalize()
	if spawn.Object.Pose.Forward().Sub(want).Length() > 1e-9 {
		t.Fatalf("bolt points %+v, want %+v", spawn.Object.Pose.Forward(), want)
	}
}
