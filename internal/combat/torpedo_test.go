package combat

import (
	"testing"

	"github.com/edwardwillis/starwars-vector-game/internal/catalog"
	"github.com/edwardwillis/starwars-vector-game/internal/kinematics"
	"github.com/edwardwillis/starwars-vector-game/internal/math3d"
	"github.com/edwardwillis/starwars-vector-game/internal/scene"
)

func TestFireProtonTorpedoTowardPreservesOwnershipAndFaction(t *testing.T) {
	shooter := catalog.XWing(4, kinematics.Pose{Orientation: math3d.IdentityQuaternion()})
	shooter.Team = scene.TeamAlliance
	shooter.Frame = "surface"
	spawn, err := FireProtonTorpedoToward(shooter, 9, "muzzle-lower-left", math3d.Vec3{Y: -8, Z: 40}, DefaultTorpedoConfig())
	if err != nil {
		t.Fatal(err)
	}
	if spawn.OwnerID != shooter.ID || spawn.Object.Team != scene.TeamAlliance || spawn.Object.Frame != shooter.Frame ||
		spawn.Object.ProjectileKind != scene.ProjectileProtonTorpedo || spawn.Lifetime != DefaultTorpedoConfig().Lifetime {
		t.Fatalf("spawn=%+v", spawn)
	}
	if spawn.Object.Pose.Forward().Y >= 0 || spawn.Object.Pose.Forward().Z <= 0 {
		t.Fatalf("torpedo direction=%+v", spawn.Object.Pose.Forward())
	}
}
