package game

import (
	"image/color"
	"math"

	"github.com/edwardwillis/starwars-vector-game/internal/math3d"
	"github.com/edwardwillis/starwars-vector-game/internal/render"
	"github.com/edwardwillis/starwars-vector-game/internal/scene"
	"github.com/hajimehoshi/ebiten/v2"
)

// Ship explosions are deliberately presentation-only. They use their own
// compact PRNG rather than the simulation's deterministic inputs, and are not
// part of the replay checksum. Their seed is nevertheless stable, so a replay
// produces the same image for a given destruction event.
const (
	maxShipExplosions         = 8
	maxExplosionLobes         = 6
	maxExplosionSparks        = 36
	explosionFlashDuration    = 0.050
	explosionDuration         = 0.45
	explosionSparkMinLifetime = 0.20
	explosionSparkMaxLifetime = 0.70
)

type cosmeticPRNG struct{ state uint64 }

func newCosmeticPRNG(seed uint64) cosmeticPRNG {
	if seed == 0 {
		seed = 0x9e3779b97f4a7c15
	}
	return cosmeticPRNG{state: seed}
}

func (r *cosmeticPRNG) next() uint64 {
	r.state ^= r.state << 13
	r.state ^= r.state >> 7
	r.state ^= r.state << 17
	return r.state
}

func (r *cosmeticPRNG) unit() float64 {
	return float64(r.next()>>11) / float64(uint64(1)<<53)
}

func (r *cosmeticPRNG) signed() float64 { return 2*r.unit() - 1 }

type explosionLobe struct {
	offsetX, offsetY float64
	radius           float64
	rotation         float64
	start, duration  float64
	points           [7]float64
}

type explosionSpark struct {
	position, velocity           math3d.Vec3
	lifetime, brightness, streak float64
}

type shipExplosion struct {
	rootID             scene.ObjectID
	frame              scene.FrameID
	position, velocity math3d.Vec3
	age, size          float64
	lobes              [maxExplosionLobes]explosionLobe
	lobeCount          int
	sparks             [maxExplosionSparks]explosionSpark
	sparkCount         int
}

func destructionEffectSeed(object scene.Object, tick uint64) uint64 {
	seed := uint64(object.ID)*0x9e3779b97f4a7c15 ^ (tick+1)*0x517cc1b727220a95
	for index := range len(object.Definition) {
		seed = (seed ^ uint64(object.Definition[index])) * 1099511628211
	}
	return seed
}

func (g *Game) spawnShipExplosion(object scene.Object) {
	if len(g.shipExplosions) >= maxShipExplosions {
		// Prefer the newest destruction to an invisible no-op when a dense
		// skirmish reaches the cosmetic budget.
		copy(g.shipExplosions, g.shipExplosions[1:])
		g.shipExplosions = g.shipExplosions[:len(g.shipExplosions)-1]
	}
	tick := uint64(0)
	if g.world != nil {
		tick = g.world.Tick
	}
	random := newCosmeticPRNG(destructionEffectSeed(object, tick))
	velocity := object.Pose.Forward().Scale(object.Motion.Speed).Add(object.Motion.Velocity)
	size := max(0.8, object.VisualRadius)
	// Catalog space ships use the zero frame as shorthand for the exterior;
	// normalise it before comparing against the camera's explicit active frame.
	effect := shipExplosion{rootID: object.ID, frame: normalizedObjectFrame(object), position: object.Pose.Position, velocity: velocity, size: size}
	effect.lobeCount = 3 + int(random.unit()*4)
	for index := 0; index < effect.lobeCount; index++ {
		lobe := explosionLobe{
			offsetX:  random.signed() * 0.34,
			offsetY:  random.signed() * 0.28,
			radius:   0.62 + random.unit()*0.52,
			rotation: random.unit() * 2 * math.Pi,
			start:    random.unit() * 0.055,
			duration: 0.25 + random.unit()*(explosionDuration-0.25-0.05),
		}
		for point := range lobe.points {
			lobe.points[point] = 0.70 + random.unit()*0.48
		}
		effect.lobes[index] = lobe
	}
	effect.sparkCount = min(maxExplosionSparks, 20+int(random.unit()*17))
	for index := 0; index < effect.sparkCount; index++ {
		direction := math3d.Vec3{X: random.signed(), Y: random.signed(), Z: random.signed()}.Normalize()
		if direction == (math3d.Vec3{}) {
			direction = math3d.Vec3{Z: -1}
		}
		speed := size * (4.6 + random.unit()*9.0)
		effect.sparks[index] = explosionSpark{
			position:   effect.position.Add(direction.Scale(size * (0.08 + random.unit()*0.20))),
			velocity:   velocity.Add(direction.Scale(speed)),
			lifetime:   explosionSparkMinLifetime + random.unit()*(explosionSparkMaxLifetime-explosionSparkMinLifetime),
			brightness: 0.55 + random.unit()*0.45,
			streak:     size * (0.28 + random.unit()*0.42),
		}
	}
	g.shipExplosions = append(g.shipExplosions, effect)
}

func (g *Game) updateShipExplosions(seconds float64) {
	if seconds <= 0 {
		return
	}
	kept := g.shipExplosions[:0]
	for index := range g.shipExplosions {
		effect := g.shipExplosions[index]
		effect.age += seconds
		if effect.age >= explosionSparkMaxLifetime {
			continue
		}
		effect.position = effect.position.Add(effect.velocity.Scale(seconds))
		for sparkIndex := 0; sparkIndex < effect.sparkCount; sparkIndex++ {
			spark := &effect.sparks[sparkIndex]
			if effect.age <= spark.lifetime {
				spark.position = spark.position.Add(spark.velocity.Scale(seconds))
			}
		}
		kept = append(kept, effect)
	}
	g.shipExplosions = kept
}

func (g *Game) clearShipExplosions() { g.shipExplosions = g.shipExplosions[:0] }

func (g *Game) debrisExplosionGlow(objectID scene.ObjectID) float64 {
	transient, debris := g.debris[objectID]
	if !debris {
		return 0
	}
	for index := range g.shipExplosions {
		effect := &g.shipExplosions[index]
		if effect.rootID != transient.rootObjectID || effect.age > 0.48 {
			continue
		}
		// Only a subset becomes incandescent, and their cooling is staggered
		// without adding state to the authoritative debris lifecycle.
		if transient.componentIndex == 1 {
			return 0
		}
		phase := float64((uint64(objectID)*0x9e3779b97f4a7c15)>>60) * 0.025
		cooling := effect.age - phase
		if cooling < 0 {
			return 0.85
		}
		return max(0, 1-cooling/0.42)
	}
	return 0
}

func (g *Game) debrisColor(object scene.Object, base color.RGBA) color.RGBA {
	if object.DestructionStage < scene.DestructionComponent {
		return base
	}
	intensity := g.debrisExplosionGlow(object.ID)
	if intensity <= 0 {
		return base
	}
	// The first half is nearly white-yellow; it cools to orange before the
	// authored vector colour takes over.
	heat := color.RGBA{R: 255, G: uint8(104 + 142*min(1, intensity*2)), B: uint8(18 + 196*min(1, intensity*2)), A: base.A}
	return color.RGBA{
		R: uint8(float64(base.R)*(1-intensity) + float64(heat.R)*intensity),
		G: uint8(float64(base.G)*(1-intensity) + float64(heat.G)*intensity),
		B: uint8(float64(base.B)*(1-intensity) + float64(heat.B)*intensity), A: base.A,
	}
}

func (g *Game) explosionDepthForPrepared(prepared *preparedFrame) *render.DepthBuffer {
	if len(g.shipExplosions) == 0 || prepared == nil {
		return nil
	}
	if g.explosionDepth == nil || g.explosionDepth.ViewWidth != g.pipeline.Width || g.explosionDepth.ViewHeight != g.pipeline.Height {
		g.explosionDepth = render.NewScaledDepthBuffer(g.pipeline.Width, g.pipeline.Height, 0.5)
	}
	g.explosionDepth.Clear()
	for index := range prepared.candidates {
		candidate := &prepared.candidates[index]
		if candidate.writesDepth {
			g.pipeline.RasterizePreparedDepthOwned(candidate.geometry, g.explosionDepth, candidate.owner)
		}
	}
	return g.explosionDepth
}

func explosionColor(progress, brightness float64) (float32, float32, float32, float32) {
	alpha := float32(brightness * max(0, 1-progress))
	if progress < 0.22 {
		cool := progress / 0.22
		return 1, float32(1 - 0.10*cool), float32(0.55 - 0.22*cool), alpha
	}
	cool := (progress - 0.22) / 0.78
	return 1, float32(0.90 - 0.38*cool), float32(0.28 - 0.20*cool), alpha
}

func (g *Game) explosionOccluded(depth *render.DepthBuffer, rootID scene.ObjectID, point render.Point, bias float64) bool {
	if depth == nil || point.Depth <= 0 {
		return false
	}
	nearest, owner := depth.NearestScreenPoint(point.X, point.Y)
	if nearest >= point.Depth-bias {
		return false
	}
	// Object owners encode their ID in the high 32 bits. Debris carries its
	// intact source as rootObjectID, so a fireball can sit over its own breakup
	// while terrain, structures, and every other object still occlude it.
	objectID := scene.ObjectID(owner >> 32)
	if transient, debris := g.debris[objectID]; debris && transient.rootObjectID == rootID {
		return false
	}
	return true
}

func (g *Game) explosionVisible(effect shipExplosion, radius float64, depth *render.DepthBuffer) (render.Point, float64, bool) {
	if effect.frame != g.viewContext.FrameID || radius <= 0 {
		return render.Point{}, 0, false
	}
	cameraPoint := g.pipeline.View.TransformPoint(effect.position)
	distance := -cameraPoint.Z
	if distance+radius <= g.pipeline.Near || (g.pipeline.Far > g.pipeline.Near && distance-radius >= g.pipeline.Far) {
		return render.Point{}, 0, false
	}
	projectionDepth := max(distance, g.pipeline.Near*1.01)
	center := render.Point{
		X:     (cameraPoint.X/projectionDepth*g.pipeline.Projection[0][0] + 1) * 0.5 * float64(g.pipeline.Width),
		Y:     (1 - cameraPoint.Y/projectionDepth*g.pipeline.Projection[1][1]) * 0.5 * float64(g.pipeline.Height),
		Depth: projectionDepth,
	}
	if center.X < -float64(g.pipeline.Width) || center.X > 2*float64(g.pipeline.Width) || center.Y < -float64(g.pipeline.Height) || center.Y > 2*float64(g.pipeline.Height) {
		return render.Point{}, 0, false
	}
	if g.explosionOccluded(depth, effect.rootID, center, 0.10) {
		return render.Point{}, 0, false
	}
	pixels := radius / projectionDepth * g.pipeline.Projection[1][1] * float64(g.pipeline.Height) * 0.5
	if distance <= g.pipeline.Near {
		pixels = min(pixels, math.Hypot(float64(g.pipeline.Width), float64(g.pipeline.Height))*1.5)
	}
	return center, pixels, pixels > 0
}

func appendExplosionTriangle(vertices []ebiten.Vertex, indices []uint16, a, b, c render.Point, red, green, blue, alpha float32) ([]ebiten.Vertex, []uint16) {
	if len(vertices)+3 > math.MaxUint16 {
		return vertices, indices
	}
	base := uint16(len(vertices))
	vertices = append(vertices,
		ebiten.Vertex{DstX: float32(a.X), DstY: float32(a.Y), SrcX: .5, SrcY: .5, ColorR: red, ColorG: green, ColorB: blue, ColorA: alpha},
		ebiten.Vertex{DstX: float32(b.X), DstY: float32(b.Y), SrcX: .5, SrcY: .5, ColorR: red, ColorG: green, ColorB: blue, ColorA: alpha},
		ebiten.Vertex{DstX: float32(c.X), DstY: float32(c.Y), SrcX: .5, SrcY: .5, ColorR: red, ColorG: green, ColorB: blue, ColorA: alpha},
	)
	return vertices, append(indices, base, base+1, base+2)
}

// drawShipExplosions is intentionally called after wireframe submission: the
// short fireball can cover a few fragment edges, then fades to reveal the
// existing breakup. The CPU depth surface rejects explosions hidden by terrain
// and solid structures before this luminous final pass.
func (g *Game) drawShipExplosions(screen *ebiten.Image, depth *render.DepthBuffer) {
	if len(g.shipExplosions) == 0 {
		return
	}
	g.ensureWhitePixel()
	g.explosionVertices = g.explosionVertices[:0]
	g.explosionIndices = g.explosionIndices[:0]
	for _, effect := range g.shipExplosions {
		center, scale, visible := g.explosionVisible(effect, effect.size*2.8, depth)
		if !visible {
			continue
		}
		if effect.age < explosionFlashDuration {
			flash := 0.22 + effect.age/explosionFlashDuration*0.34
			r, gr, b, alpha := explosionColor(0, 1-effect.age/explosionFlashDuration*0.12)
			for index := 0; index < 5; index++ {
				angleA := 2 * math.Pi * float64(index) / 5
				angleB := 2 * math.Pi * float64(index+1) / 5
				pointA := render.Point{X: center.X + math.Cos(angleA)*scale*flash, Y: center.Y + math.Sin(angleA)*scale*flash}
				bpt := render.Point{X: center.X + math.Cos(angleB)*scale*flash, Y: center.Y + math.Sin(angleB)*scale*flash}
				g.explosionVertices, g.explosionIndices = appendExplosionTriangle(g.explosionVertices, g.explosionIndices, center, pointA, bpt, r, gr, b, alpha)
			}
		}
		for index := 0; index < effect.lobeCount; index++ {
			lobe := effect.lobes[index]
			progress := (effect.age - lobe.start) / lobe.duration
			if progress < 0 || progress >= 1 {
				continue
			}
			ease := 1 - math.Pow(1-progress, 2)
			radius := scale * lobe.radius * (0.25 + 1.05*ease)
			lobeCenter := render.Point{X: center.X + lobe.offsetX*scale*(0.25+ease), Y: center.Y + lobe.offsetY*scale*(0.25+ease), Depth: center.Depth}
			r, gr, b, a := explosionColor(progress, 0.56+0.30*(1-progress))
			var points [7]render.Point
			for point := range points {
				angle := lobe.rotation + 2*math.Pi*float64(point)/float64(len(points))
				points[point] = render.Point{X: lobeCenter.X + math.Cos(angle)*radius*lobe.points[point], Y: lobeCenter.Y + math.Sin(angle)*radius*lobe.points[point]}
			}
			for point := range points {
				g.explosionVertices, g.explosionIndices = appendExplosionTriangle(g.explosionVertices, g.explosionIndices, lobeCenter, points[point], points[(point+1)%len(points)], r, gr, b, a)
			}
		}
		for index := 0; index < effect.sparkCount; index++ {
			spark := effect.sparks[index]
			if effect.age > spark.lifetime {
				continue
			}
			cameraPoint := g.pipeline.View.TransformPoint(spark.position)
			distance := -cameraPoint.Z
			if distance <= g.pipeline.Near || (g.pipeline.Far > g.pipeline.Near && distance >= g.pipeline.Far) {
				continue
			}
			point, projected := g.pipeline.ProjectPointUnclipped(spark.position)
			if !projected || g.explosionOccluded(depth, effect.rootID, point, 0.06) {
				continue
			}
			direction := spark.velocity.Normalize()
			tail, tailProjected := g.pipeline.ProjectPointUnclipped(spark.position.Sub(direction.Scale(spark.streak)))
			if !tailProjected {
				continue
			}
			progress := effect.age / spark.lifetime
			r, gr, b, a := explosionColor(progress, spark.brightness)
			dx, dy := point.X-tail.X, point.Y-tail.Y
			length := math.Hypot(dx, dy)
			if length < 0.5 {
				dx, dy, length = 1, 0, 1
			}
			half := 0.75 + 1.25*(1-progress)
			perpX, perpY := -dy/length*half, dx/length*half
			aPoint := render.Point{X: tail.X + perpX, Y: tail.Y + perpY}
			bPoint := render.Point{X: tail.X - perpX, Y: tail.Y - perpY}
			g.explosionVertices, g.explosionIndices = appendExplosionTriangle(g.explosionVertices, g.explosionIndices, point, aPoint, bPoint, r, gr, b, a)
		}
	}
	if len(g.explosionIndices) == 0 {
		return
	}
	options := &ebiten.DrawTrianglesOptions{
		Filter:         ebiten.FilterNearest,
		ColorScaleMode: ebiten.ColorScaleModeStraightAlpha,
		Blend:          ebiten.BlendLighter,
	}
	screen.DrawTriangles(g.explosionVertices, g.explosionIndices, g.whitePixel, options)
}
