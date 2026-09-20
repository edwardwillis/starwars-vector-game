package combat

import (
	"fmt"
	"math"

	"github.com/edwardwillis/starwars-vector-game/internal/catalog"
	"github.com/edwardwillis/starwars-vector-game/internal/kinematics"
	"github.com/edwardwillis/starwars-vector-game/internal/math3d"
	"github.com/edwardwillis/starwars-vector-game/internal/scene"
)

// TorpedoConfig contains both flight tuning and the explicit attack-run
// acceptance envelope. Keeping these values in the profile makes mission
// difficulty configurable without coupling it to rendering or catalog data.
type TorpedoConfig struct {
	Speed             float64
	Lifetime          float64
	Cooldown          float64
	Ammunition        int
	MinimumTravel     float64
	MaximumTravel     float64
	MaximumAlignment  float64
	MinimumForwardDot float64
	MinimumDownDot    float64
}

func DefaultTorpedoConfig() TorpedoConfig {
	return TorpedoConfig{
		Speed: 13, Lifetime: 6, Cooldown: 0.75, Ammunition: 2,
		MinimumTravel: 4, MaximumTravel: 90, MaximumAlignment: 1.4,
		MinimumForwardDot: 0.55, MinimumDownDot: 0.08,
	}
}

func (config TorpedoConfig) Validate() error {
	finitePositive := func(name string, value float64) error {
		if value <= 0 || math.IsNaN(value) || math.IsInf(value, 0) {
			return fmt.Errorf("torpedo %s must be positive", name)
		}
		return nil
	}
	for _, value := range []struct {
		name  string
		value float64
	}{{"speed", config.Speed}, {"lifetime", config.Lifetime}, {"cooldown", config.Cooldown}, {"maximum travel", config.MaximumTravel}, {"alignment", config.MaximumAlignment}} {
		if err := finitePositive(value.name, value.value); err != nil {
			return err
		}
	}
	if config.Ammunition <= 0 {
		return fmt.Errorf("torpedo ammunition must be positive")
	}
	if config.MinimumTravel < 0 || config.MinimumTravel >= config.MaximumTravel {
		return fmt.Errorf("torpedo travel envelope is invalid")
	}
	if config.MinimumForwardDot < -1 || config.MinimumForwardDot > 1 || config.MinimumDownDot < -1 || config.MinimumDownDot > 1 {
		return fmt.Errorf("torpedo approach thresholds must be in [-1, 1]")
	}
	return nil
}

// FireProtonTorpedoToward creates one authoritative payload aimed from a
// named launcher toward a point in the shooter's current simulation frame.
func FireProtonTorpedoToward(shooter scene.Object, id scene.ObjectID, muzzle string, target math3d.Vec3, config TorpedoConfig) (Spawn, error) {
	if err := config.Validate(); err != nil {
		return Spawn{}, err
	}
	pose, ok := shooter.Anchor(muzzle)
	if !ok {
		return Spawn{}, fmt.Errorf("object %d has no %q anchor", shooter.ID, muzzle)
	}
	direction := target.Sub(pose.Position).Normalize()
	if direction == (math3d.Vec3{}) {
		return Spawn{}, fmt.Errorf("object %d launcher %q is already at its target", shooter.ID, muzzle)
	}
	pose.Orientation = math3d.QuaternionFromYawPitchRoll(
		math.Atan2(direction.X, direction.Z),
		-math.Asin(max(-1, min(1, direction.Y))),
		0,
	)
	torpedo := catalog.ProtonTorpedo(id, pose)
	torpedo.Frame = shooter.Frame
	torpedo.Team = shooter.Team
	torpedo.Motion = kinematics.Motion{Speed: shooter.Motion.Speed + config.Speed}
	return Spawn{Object: torpedo, OwnerID: shooter.ID, Lifetime: config.Lifetime}, nil
}
