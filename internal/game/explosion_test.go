package game

import (
	"reflect"
	"testing"

	"github.com/edwardwillis/starwars-vector-game/internal/kinematics"
	"github.com/edwardwillis/starwars-vector-game/internal/math3d"
	"github.com/edwardwillis/starwars-vector-game/internal/render"
	"github.com/edwardwillis/starwars-vector-game/internal/scene"
)

func TestShipExplosionIsDeterministicAndCosmetic(t *testing.T) {
	first, second := New(), New()
	object := scene.Object{
		ID:           77,
		Definition:   "builtin/test-fighter",
		Frame:        scene.ExteriorFrame,
		Pose:         kinematics.Pose{Position: math3d.Vec3{X: 3, Y: -2, Z: -8}, Orientation: math3d.IdentityQuaternion()},
		Motion:       kinematics.Motion{Speed: 4, Velocity: math3d.Vec3{X: .5, Y: -.25}},
		VisualRadius: 2,
	}
	first.world.Tick, second.world.Tick = 31, 31
	before, err := first.simulationChecksum()
	if err != nil {
		t.Fatal(err)
	}
	first.spawnShipExplosion(object)
	second.spawnShipExplosion(object)
	after, err := first.simulationChecksum()
	if err != nil {
		t.Fatal(err)
	}
	if before != after {
		t.Fatal("cosmetic explosion changed authoritative checksum")
	}
	if !reflect.DeepEqual(first.shipExplosions, second.shipExplosions) {
		t.Fatal("same destruction event produced different cosmetic explosion state")
	}
	effect := first.shipExplosions[0]
	if effect.lobeCount < 3 || effect.lobeCount > maxExplosionLobes || effect.sparkCount < 20 || effect.sparkCount > maxExplosionSparks {
		t.Fatalf("effect budget lobes=%d sparks=%d", effect.lobeCount, effect.sparkCount)
	}
	if got, want := effect.velocity, object.Pose.Forward().Scale(object.Motion.Speed).Add(object.Motion.Velocity); got != want {
		t.Fatalf("explosion velocity=%+v, want inherited %+v", got, want)
	}
}

func TestShipExplosionsAdvanceExpireAndReset(t *testing.T) {
	g := New()
	object := *g.objectByID(fighterID)
	object.Motion = kinematics.Motion{Velocity: math3d.Vec3{X: 2}}
	g.spawnShipExplosion(object)
	start := g.shipExplosions[0].position
	g.updateShipExplosions(.1)
	if got := g.shipExplosions[0].position; got == start {
		t.Fatal("explosion did not advance in fixed simulation time")
	}
	g.updateShipExplosions(explosionSparkMaxLifetime)
	if len(g.shipExplosions) != 0 {
		t.Fatalf("expired explosions retained: %d", len(g.shipExplosions))
	}
	g.spawnShipExplosion(object)
	g.resetFighter()
	if len(g.shipExplosions) != 0 {
		t.Fatalf("reset retained %d ship explosions", len(g.shipExplosions))
	}
}

func TestShipExplosionBudgetEvictsOldest(t *testing.T) {
	g := New()
	for index := 0; index < maxShipExplosions+2; index++ {
		g.spawnShipExplosion(scene.Object{ID: scene.ObjectID(index + 1), Definition: "builtin/test", VisualRadius: 1, Pose: kinematics.Pose{Orientation: math3d.IdentityQuaternion()}})
	}
	if len(g.shipExplosions) != maxShipExplosions {
		t.Fatalf("active explosions=%d, want cap %d", len(g.shipExplosions), maxShipExplosions)
	}
	if g.shipExplosions[0].rootID != 3 {
		t.Fatalf("oldest retained explosion root=%d, want 3", g.shipExplosions[0].rootID)
	}
}

func TestShipExplosionIgnoresOnlyItsOwnDebrisDepth(t *testing.T) {
	const rootID = scene.ObjectID(77)
	const fragmentID = scene.ObjectID(91)
	g := &Game{debris: map[scene.ObjectID]destructionTransient{fragmentID: {rootObjectID: rootID}}}
	depth := render.NewDepthBuffer(2, 2)
	depth.Values[0] = 1
	depth.Owners[0] = uint64(fragmentID)<<32 | 1
	point := render.Point{X: .1, Y: .1, Depth: 2}
	if g.explosionOccluded(depth, rootID, point, .1) {
		t.Fatal("explosion was hidden by its own fragment")
	}
	depth.Owners[0] = uint64(fragmentID+1)<<32 | 1
	if !g.explosionOccluded(depth, rootID, point, .1) {
		t.Fatal("explosion ignored a different object occluder")
	}
}
