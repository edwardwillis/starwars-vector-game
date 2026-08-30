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

type Motion struct {
	Speed     float64
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
	pose.Position = pose.Position.Add(pose.Forward().Scale(motion.Speed * seconds))
	return pose
}
