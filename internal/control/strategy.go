package control

import (
	"math"

	"github.com/edwardwillis/starwars-vector-game/internal/kinematics"
	"github.com/edwardwillis/starwars-vector-game/internal/math3d"
	"github.com/edwardwillis/starwars-vector-game/internal/scene"
)

// Strategy supplies motion for one autonomous object simulation tick. The
// game owns poses and integration; strategies only decide how an object moves.
type Strategy interface {
	Step(self, target scene.Object, seconds float64) kinematics.Motion
}

type PursuitConfig struct {
	PreferredDistance   float64
	MinSpeed            float64
	MaxSpeed            float64
	Acceleration        float64
	ApproachGain        float64
	TurnGain            float64
	MaxYawRate          float64
	MaxPitchRate        float64
	MaxRollRate         float64
	WanderStrength      float64
	WanderInterval      float64
	ScatterRadius       float64
	ExcursionMinGap     float64
	ExcursionMaxGap     float64
	ExcursionMinTime    float64
	ExcursionMaxTime    float64
	SpeedVariation      float64
	SpeedChangeInterval float64
	SpeedBlendRate      float64
}

func DefaultPursuitConfig() PursuitConfig {
	return PursuitConfig{
		PreferredDistance:   4.5,
		MinSpeed:            0.15,
		MaxSpeed:            1.15,
		Acceleration:        0.55,
		ApproachGain:        0.12,
		TurnGain:            1.35,
		MaxYawRate:          0.7,
		MaxPitchRate:        0.55,
		MaxRollRate:         0.8,
		WanderStrength:      0.16,
		WanderInterval:      0.8,
		ScatterRadius:       2.5,
		ExcursionMinGap:     4,
		ExcursionMaxGap:     8,
		ExcursionMinTime:    1.5,
		ExcursionMaxTime:    3.5,
		SpeedVariation:      0.25,
		SpeedChangeInterval: 5,
		SpeedBlendRate:      0.65,
	}
}

// Pursuit keeps an object near its target while adding smooth deterministic
// steering variation. Each instance owns its own pseudo-random state.
type Pursuit struct {
	config             PursuitConfig
	randomState        uint64
	wanderTime         float64
	wanderYaw          float64
	wanderPitch        float64
	targetYaw          float64
	targetPitch        float64
	targetOffset       math3d.Vec3
	excursionDirection math3d.Vec3
	excursion          bool
	excursionTime      float64
	excursionGap       float64
	speedChangeTime    float64
	speedOffset        float64
	targetSpeedOffset  float64
}

func NewPursuit(seed uint64, config PursuitConfig) *Pursuit {
	if seed == 0 {
		seed = 1
	}
	pursuit := &Pursuit{config: config, randomState: seed}
	pursuit.targetOffset = pursuit.randomOffset()
	pursuit.excursionGap = pursuit.randomRange(config.ExcursionMinGap, config.ExcursionMaxGap)
	pursuit.targetSpeedOffset = pursuit.randomSigned() * config.SpeedVariation
	pursuit.speedChangeTime = pursuit.randomUnit() * config.SpeedChangeInterval
	return pursuit
}

func (p *Pursuit) Step(self, target scene.Object, seconds float64) kinematics.Motion {
	if seconds <= 0 {
		return self.Motion
	}
	p.updateWander(seconds)
	p.updateExcursion(self, target, seconds)
	p.updateSpeedVariation(seconds)

	toTarget := target.Pose.Position.Add(p.targetOffset).Sub(self.Pose.Position)
	if p.excursion {
		toTarget = p.excursionDirection
	}
	distance := toTarget.Length()
	localTarget := self.Pose.Orientation.Normalize().Conjugate().Rotate(toTarget.Normalize())
	yawError := math.Atan2(localTarget.X, localTarget.Z)
	horizontal := math.Hypot(localTarget.X, localTarget.Z)
	pitchError := -math.Atan2(localTarget.Y, horizontal)

	desiredSpeed := target.Motion.Speed + (distance-p.config.PreferredDistance)*p.config.ApproachGain + p.speedOffset
	if p.excursion {
		desiredSpeed = p.config.MaxSpeed*0.85 + p.speedOffset
	}
	desiredSpeed = clamp(desiredSpeed, p.config.MinSpeed, p.config.MaxSpeed)
	motion := self.Motion
	motion.Speed = moveToward(motion.Speed, desiredSpeed, p.config.Acceleration*seconds)
	motion.YawRate = clamp(yawError*p.config.TurnGain+p.wanderYaw, -p.config.MaxYawRate, p.config.MaxYawRate)
	motion.PitchRate = clamp(pitchError*p.config.TurnGain+p.wanderPitch, -p.config.MaxPitchRate, p.config.MaxPitchRate)
	motion.RollRate = clamp(-motion.YawRate*0.7, -p.config.MaxRollRate, p.config.MaxRollRate)
	return motion
}

func (p *Pursuit) updateSpeedVariation(seconds float64) {
	p.speedChangeTime -= seconds
	if p.speedChangeTime <= 0 {
		p.targetSpeedOffset = p.randomSigned() * p.config.SpeedVariation
		p.speedChangeTime += p.config.SpeedChangeInterval
		if p.speedChangeTime <= 0 {
			p.speedChangeTime = p.config.SpeedChangeInterval
		}
	}
	blend := min(1, seconds*p.config.SpeedBlendRate)
	p.speedOffset += (p.targetSpeedOffset - p.speedOffset) * blend
}

func (p *Pursuit) updateExcursion(self, target scene.Object, seconds float64) {
	if p.excursion {
		p.excursionTime -= seconds
		if p.excursionTime <= 0 {
			p.excursion = false
			p.targetOffset = p.randomOffset()
			p.excursionGap = p.randomRange(p.config.ExcursionMinGap, p.config.ExcursionMaxGap)
		}
		return
	}
	p.excursionGap -= seconds
	if p.excursionGap > 0 {
		return
	}
	p.excursion = true
	p.excursionTime = p.randomRange(p.config.ExcursionMinTime, p.config.ExcursionMaxTime)
	away := self.Pose.Position.Sub(target.Pose.Position).Normalize()
	variation := math3d.Vec3{
		X: p.randomSigned(),
		Y: p.randomSigned() * 0.7,
		Z: p.randomSigned(),
	}.Normalize()
	p.excursionDirection = away.Add(variation.Scale(0.65)).Normalize()
	if p.excursionDirection == (math3d.Vec3{}) {
		p.excursionDirection = self.Pose.Forward()
	}
}

func (p *Pursuit) updateWander(seconds float64) {
	p.wanderTime -= seconds
	if p.wanderTime <= 0 {
		p.targetYaw = p.randomSigned() * p.config.WanderStrength
		p.targetPitch = p.randomSigned() * p.config.WanderStrength * 0.65
		p.wanderTime += p.config.WanderInterval
		if p.wanderTime <= 0 {
			p.wanderTime = p.config.WanderInterval
		}
	}
	blend := min(1, seconds*2.5)
	p.wanderYaw += (p.targetYaw - p.wanderYaw) * blend
	p.wanderPitch += (p.targetPitch - p.wanderPitch) * blend
}

func (p *Pursuit) randomSigned() float64 {
	state := p.randomState
	state ^= state << 13
	state ^= state >> 7
	state ^= state << 17
	p.randomState = state
	return 2*(float64(state>>11)/float64(uint64(1)<<53)) - 1
}

func (p *Pursuit) randomOffset() math3d.Vec3 {
	if p.config.ScatterRadius <= 0 {
		return math3d.Vec3{}
	}
	direction := math3d.Vec3{
		X: p.randomSigned(),
		Y: p.randomSigned() * 0.55,
		Z: p.randomSigned(),
	}.Normalize()
	radius := p.config.ScatterRadius * (0.45 + 0.55*p.randomUnit())
	return direction.Scale(radius)
}

func (p *Pursuit) randomRange(minimum, maximum float64) float64 {
	if maximum <= minimum {
		return minimum
	}
	return minimum + (maximum-minimum)*p.randomUnit()
}

func (p *Pursuit) randomUnit() float64 {
	return (p.randomSigned() + 1) * 0.5
}

func moveToward(value, target, maximumDelta float64) float64 {
	if value < target {
		return min(value+maximumDelta, target)
	}
	return max(value-maximumDelta, target)
}
