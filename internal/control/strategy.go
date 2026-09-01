package control

import (
	"math"

	"github.com/edwardwillis/starwars-vector-game/internal/kinematics"
	"github.com/edwardwillis/starwars-vector-game/internal/math3d"
	"github.com/edwardwillis/starwars-vector-game/internal/scene"
)

// Context is the read-only world information available to a controller tick.
// Nearby contains physical objects other than Self.
type Context struct {
	Self        scene.Object
	Target      scene.Object
	Nearby      []scene.Object
	Seconds     float64
	MotionScale float64
}

// Decision is the complete request produced by a controller for one tick. The
// simulation remains authoritative over applying flight limits, firing, and
// all world mutations.
type Decision struct {
	Flight Intent
	Aim    math3d.Vec3
	Fire   bool
}

// Strategy supplies a validated decision for one object simulation tick. The
// game owns poses and integration; strategies never mutate world state.
type Strategy interface {
	Decide(context Context) Decision
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
	AvoidanceHorizon    float64
	AvoidanceMargin     float64
	AvoidanceMinTime    float64
	AvoidanceMaxTime    float64
	AvoidanceCooldown   float64
	AvoidanceSlowdown   float64
	AttackMinGap        float64
	AttackMaxGap        float64
	AttackMinTime       float64
	AttackMaxTime       float64
	AttackMinRadius     float64
	AttackMaxRadius     float64
	AttackFireMinGap    float64
	AttackFireMaxGap    float64
	AttackRange         float64
	AttackAimDot        float64
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
		AvoidanceHorizon:    1.25,
		AvoidanceMargin:     1.0,
		AvoidanceMinTime:    0.55,
		AvoidanceMaxTime:    0.9,
		AvoidanceCooldown:   0.4,
		AvoidanceSlowdown:   0.8,
		AttackMinGap:        3,
		AttackMaxGap:        8,
		AttackMinTime:       2.5,
		AttackMaxTime:       5,
		AttackMinRadius:     4,
		AttackMaxRadius:     10,
		AttackFireMinGap:    0.55,
		AttackFireMaxGap:    1.1,
		AttackRange:         30,
		AttackAimDot:        0.82,
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
	avoidanceTime      float64
	avoidanceCooldown  float64
	avoidanceYaw       float64
	avoidancePitch     float64
	avoidanceRoll      float64
	avoidanceObject    scene.ObjectID
	attacking          bool
	attackGap          float64
	attackTime         float64
	attackRadius       float64
	attackAngle        float64
	attackDirection    float64
	attackFireGap      float64
	attackFire         bool
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
	pursuit.attackGap = pursuit.randomRange(config.AttackMinGap, config.AttackMaxGap)
	return pursuit
}

// AttackIntent reports whether the most recent Step selected a firing moment.
func (p *Pursuit) AttackIntent() bool {
	return p.attackFire
}

// Decide returns an atomic flight, aim, and fire decision. Step remains below
// as a compatibility helper for package users migrating to the decision API.
func (p *Pursuit) Decide(context Context) Decision {
	motion := p.stepMotion(context)
	return Decision{
		Flight: intentForMotion(context.Self.Motion, motion, p.config, context.Seconds),
		Aim:    context.Target.Pose.Position,
		Fire:   p.attackFire,
	}
}

func (p *Pursuit) Step(context Context) kinematics.Motion {
	return p.stepMotion(context)
}

func (p *Pursuit) stepMotion(context Context) kinematics.Motion {
	self, target, seconds := context.Self, context.Target, context.Seconds
	if seconds <= 0 {
		return self.Motion
	}
	p.updateWander(seconds)
	p.updateAttack(self, target, seconds)
	p.updateExcursion(self, target, seconds)
	p.updateSpeedVariation(seconds)
	p.updateAvoidance(context)

	toTarget := target.Pose.Position.Add(p.targetOffset).Sub(self.Pose.Position)
	if p.attacking {
		p.attackAngle += p.attackDirection * p.config.MaxSpeed / max(0.1, p.attackRadius) * seconds * max(1, context.MotionScale)
		arcOffset := math3d.Vec3{
			X: math.Cos(p.attackAngle) * p.attackRadius,
			Y: math.Sin(p.attackAngle*0.7) * p.attackRadius * 0.22,
			Z: math.Sin(p.attackAngle) * p.attackRadius,
		}
		toTarget = target.Pose.Position.Add(arcOffset).Sub(self.Pose.Position)
	} else if p.excursion {
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
	if p.avoidanceTime > 0 {
		desiredSpeed *= p.config.AvoidanceSlowdown
	}
	desiredSpeed = clamp(desiredSpeed, p.config.MinSpeed, p.config.MaxSpeed)
	motion := self.Motion
	motion.Speed = moveToward(motion.Speed, desiredSpeed, p.config.Acceleration*seconds)
	motion.YawRate = clamp(yawError*p.config.TurnGain+p.wanderYaw, -p.config.MaxYawRate, p.config.MaxYawRate)
	motion.PitchRate = clamp(pitchError*p.config.TurnGain+p.wanderPitch, -p.config.MaxPitchRate, p.config.MaxPitchRate)
	motion.RollRate = clamp(-motion.YawRate*0.7, -p.config.MaxRollRate, p.config.MaxRollRate)
	if p.avoidanceTime > 0 {
		fade := min(1, p.avoidanceTime/0.2)
		motion.YawRate = blend(motion.YawRate, p.avoidanceYaw, 0.95*fade)
		motion.PitchRate = blend(motion.PitchRate, p.avoidancePitch, 0.85*fade)
		motion.RollRate = blend(motion.RollRate, p.avoidanceRoll, 0.95*fade)
	}
	return motion
}

func intentForMotion(current, desired kinematics.Motion, config PursuitConfig, seconds float64) Intent {
	intent := Intent{
		Yaw:   desired.YawRate / max(0.000001, config.MaxYawRate),
		Pitch: desired.PitchRate / max(0.000001, config.MaxPitchRate),
		Roll:  desired.RollRate / max(0.000001, config.MaxRollRate),
	}
	if seconds > 0 && config.Acceleration > 0 {
		intent.Throttle = (desired.Speed - current.Speed) / (config.Acceleration * seconds)
	}
	return intent
}

func (p *Pursuit) updateAttack(self, target scene.Object, seconds float64) {
	p.attackFire = false
	if p.attacking {
		p.attackTime -= seconds
		p.attackFireGap -= seconds
		if p.attackTime <= 0 {
			p.attacking = false
			p.attackGap = p.randomRange(p.config.AttackMinGap, p.config.AttackMaxGap)
			p.targetOffset = p.randomOffset()
			return
		}
		toTarget := target.Pose.Position.Sub(self.Pose.Position)
		distance := toTarget.Length()
		if p.attackFireGap <= 0 && distance <= p.config.AttackRange {
			alignment := self.Pose.Forward().Dot(toTarget.Normalize())
			if alignment >= p.config.AttackAimDot {
				p.attackFire = true
				p.attackFireGap = p.randomRange(p.config.AttackFireMinGap, p.config.AttackFireMaxGap)
			}
		}
		return
	}
	p.attackGap -= seconds
	if p.attackGap > 0 {
		return
	}
	p.attacking = true
	p.excursion = false
	p.attackTime = p.randomRange(p.config.AttackMinTime, p.config.AttackMaxTime)
	p.attackRadius = p.randomRange(p.config.AttackMinRadius, p.config.AttackMaxRadius)
	p.attackAngle = p.randomRange(0, 2*math.Pi)
	p.attackDirection = signOrRandom(p.randomSigned(), 1)
	p.attackFireGap = p.randomRange(0, p.config.AttackFireMaxGap)
}

func (p *Pursuit) updateAvoidance(context Context) {
	if p.avoidanceTime > 0 {
		p.avoidanceTime -= context.Seconds
		if p.avoidanceTime <= 0 {
			p.avoidanceTime = 0
			p.avoidanceCooldown = p.config.AvoidanceCooldown
			p.avoidanceObject = 0
		}
		return
	}
	p.avoidanceCooldown = max(0, p.avoidanceCooldown-context.Seconds)
	if p.avoidanceCooldown > 0 {
		return
	}

	motionScale := context.MotionScale
	if motionScale <= 0 {
		motionScale = 1
	}
	selfVelocity := objectVelocity(context.Self).Scale(motionScale)
	nearestTime := math.Inf(1)
	var threat scene.Object
	for _, nearby := range context.Nearby {
		if nearby.ID == context.Self.ID || nearby.CollisionRole != scene.CollisionSolid {
			continue
		}
		relativePosition := nearby.Pose.Position.Sub(context.Self.Pose.Position)
		safeDistance := context.Self.CollisionRadius + nearby.CollisionRadius + p.config.AvoidanceMargin
		if relativePosition.Length() < safeDistance {
			nearestTime = 0
			threat = nearby
			break
		}
		relativeVelocity := objectVelocity(nearby).Scale(motionScale).Sub(selfVelocity)
		speedSquared := relativeVelocity.Dot(relativeVelocity)
		if speedSquared == 0 {
			continue
		}
		closestTime := -relativePosition.Dot(relativeVelocity) / speedSquared
		if closestTime <= 0 || closestTime > p.config.AvoidanceHorizon {
			continue
		}
		closestOffset := relativePosition.Add(relativeVelocity.Scale(closestTime))
		if closestOffset.Length() < safeDistance && closestTime < nearestTime {
			nearestTime = closestTime
			threat = nearby
		}
	}
	if threat.ID == 0 {
		return
	}
	p.beginAvoidance(context.Self, threat)
}

func (p *Pursuit) beginAvoidance(self, threat scene.Object) {
	localThreat := self.Pose.Orientation.Normalize().Conjugate().Rotate(
		threat.Pose.Position.Sub(self.Pose.Position).Normalize(),
	)
	yawSign := -signOrRandom(localThreat.X, p.randomSigned())
	if math.Abs(localThreat.X) < 0.12 {
		lower, upper := self.ID, threat.ID
		if lower > upper {
			lower, upper = upper, lower
		}
		yawSign = 1
		if (uint64(lower)*31+uint64(upper))%2 == 0 {
			yawSign = -1
		}
	}
	pitchSign := signOrRandom(localThreat.Y, p.randomSigned())
	p.avoidanceYaw = yawSign * p.config.MaxYawRate * p.randomRange(0.78, 0.98)
	p.avoidancePitch = 0
	if math.Abs(localThreat.Y) > 0.12 || p.randomUnit() > 0.45 {
		p.avoidancePitch = pitchSign * p.config.MaxPitchRate * p.randomRange(0.5, 0.75)
	}
	p.avoidanceRoll = -yawSign * p.config.MaxRollRate * p.randomRange(0.7, 0.95)
	p.avoidanceTime = p.randomRange(p.config.AvoidanceMinTime, p.config.AvoidanceMaxTime)
	p.avoidanceObject = threat.ID
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
	if p.attacking {
		return
	}
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

func objectVelocity(object scene.Object) math3d.Vec3 {
	return object.Pose.Forward().Scale(object.Motion.Speed).Add(object.Motion.Velocity)
}

func signOrRandom(value, random float64) float64 {
	if math.Abs(value) >= 0.1 {
		return math.Copysign(1, value)
	}
	return math.Copysign(1, random)
}

func blend(from, to, amount float64) float64 {
	return from + (to-from)*clamp(amount, 0, 1)
}
