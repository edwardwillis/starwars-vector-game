// Package kinematics advances object poses using signed axial speed and local
// yaw, pitch, and roll rates.
package kinematics

import "github.com/edwardwillis/starwars-vector-game/internal/math3d"

var LocalForward = math3d.Vec3{Z: 1}

type Pose struct {
	Position    math3d.Vec3
	Orientation math3d.Quaternion
}

func (p Pose) Forward() math3d.Vec3 {
	return p.Orientation.Normalize().Rotate(LocalForward).Normalize()
}

func (p Pose) Matrix() math3d.Mat4 {
	return math3d.Translation(p.Position.X, p.Position.Y, p.Position.Z).
		Mul(p.Orientation.Matrix())
}

// Compose returns a world pose formed by applying local relative to parent.
func Compose(parent, local Pose) Pose {
	parentOrientation := parent.Orientation.Normalize()
	return Pose{
		Position:    parent.Position.Add(parentOrientation.Rotate(local.Position)),
		Orientation: parentOrientation.Mul(local.Orientation.Normalize()).Normalize(),
	}
}

// Relative expresses world in parent's local coordinate frame.
func Relative(parent, world Pose) Pose {
	inverse := parent.Orientation.Normalize().Conjugate()
	return Pose{
		Position:    inverse.Rotate(world.Position.Sub(parent.Position)),
		Orientation: inverse.Mul(world.Orientation.Normalize()).Normalize(),
	}
}

// ViewMatrix returns the inverse of the pose's world transform.
func (p Pose) ViewMatrix() math3d.Mat4 {
	inverseRotation := p.Orientation.Normalize().Conjugate().Matrix()
	inverseTranslation := math3d.Translation(-p.Position.X, -p.Position.Y, -p.Position.Z)
	return inverseRotation.Mul(inverseTranslation)
}

type Motion struct {
	Speed float64
	// Velocity adds world-space linear motion for drifting debris and other
	// objects that do not travel only along their local forward axis.
	Velocity  math3d.Vec3
	YawRate   float64
	PitchRate float64
	RollRate  float64
}

// Integrate advances a pose by one fixed-duration simulation tick. Angular
// rates are local to the object, and signed speed moves along its updated local
// forward axis.
func Integrate(pose Pose, motion Motion, seconds float64) Pose {
	if seconds <= 0 {
		return pose
	}
	delta := math3d.QuaternionFromYawPitchRoll(
		motion.YawRate*seconds,
		motion.PitchRate*seconds,
		motion.RollRate*seconds,
	)
	pose.Orientation = pose.Orientation.Normalize().Mul(delta).Normalize()
	linearVelocity := pose.Forward().Scale(motion.Speed).Add(motion.Velocity)
	pose.Position = pose.Position.Add(linearVelocity.Scale(seconds))
	return pose
}
