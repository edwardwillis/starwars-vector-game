package environment

import (
	"math"
	"testing"

	"github.com/edwardwillis/starwars-vector-game/internal/catalog"
	"github.com/edwardwillis/starwars-vector-game/internal/kinematics"
	"github.com/edwardwillis/starwars-vector-game/internal/math3d"
	"github.com/edwardwillis/starwars-vector-game/internal/model"
	"github.com/edwardwillis/starwars-vector-game/internal/render"
	"github.com/edwardwillis/starwars-vector-game/internal/scene"
	"github.com/edwardwillis/starwars-vector-game/internal/view"
)

func TestDeathStarTrenchTileSharesRenderAndCollisionFeatures(t *testing.T) {
	definition := DeathStarTrench()
	tile := definition.Tile(TileCoordinate{})
	if len(tile.Parts) != 2 || len(tile.Planes) != 5 || len(tile.Boxes) == 0 || len(tile.Features) == 0 {
		t.Fatalf("parts=%d features=%d planes=%d boxes=%d", len(tile.Parts), len(tile.Features), len(tile.Planes), len(tile.Boxes))
	}
	for _, part := range tile.Parts {
		if err := part.Validate(); err != nil {
			t.Fatalf("%s: %v", part.Name, err)
		}
		if part.Name == "surface deck" && (part.Mesh.SkipDepth || part.Mesh.DepthTestOnly || !part.Mesh.PointOccluder) {
			t.Fatalf("%s should write a physical occlusion face", part.Name)
		}
		if !part.Surface.Opaque() {
			t.Fatalf("%s should use a real flat opaque surface", part.Name)
		}
	}
	if tile.Parts[0].SelfOccluding || tile.Parts[0].SelfOcclusion != scene.SelfOcclusionNone {
		t.Fatal("surface deck should not depth-test its coplanar grid against itself")
	}
	if tile.Parts[0].Surface.Mode != scene.SurfaceTexturedOpaque || tile.Parts[0].Surface.TextureID != "builtin/death-star-deck" {
		t.Fatalf("physical deck lost subtle texture material: %+v", tile.Parts[0].Surface)
	}
	if tile.Parts[1].Surface.Mode != scene.SurfaceTexturedOpaque || tile.Parts[1].Surface.TextureID != render.DeathStarTrenchTextureID {
		t.Fatalf("physical trench lost textured interior material: %+v", tile.Parts[1].Surface)
	}
	horizon := definition.HorizonTile(TileCoordinate{})
	if horizon.Parts[0].Surface.Mode != scene.SurfaceTexturedOpaque || horizon.Parts[0].Surface.TextureID != render.DeathStarDeckTextureID {
		t.Fatalf("distant horizon lost texture detail material: %+v", horizon.Parts[0].Surface)
	}
	surfaceFill, trenchFill := tile.Parts[0].Surface.Color, tile.Parts[1].Surface.Color
	if surfaceFill.R != surfaceFill.G || surfaceFill.G != surfaceFill.B ||
		trenchFill.R != trenchFill.G || trenchFill.G != trenchFill.B ||
		trenchFill.R >= surfaceFill.R {
		t.Fatalf("surface/trench fills are not graduated neutral greys: surface=%v trench=%v", surfaceFill, trenchFill)
	}
	if line := tile.Parts[0].Color; line != deathStarSurfaceLine || line.R <= surfaceFill.R || horizon.Parts[0].Surface != deathStarHorizonTexture {
		t.Fatalf("near/horizon surface materials lost their deliberate subdued palette: near=%v horizon=%v fill=%v", line, horizon.Parts[0].Surface.Color, surfaceFill)
	}
	if tile.Parts[0].LineWidth != deathStarDeckVectorWidth || tile.Parts[1].LineWidth != deathStarTrenchVectorWidth {
		t.Fatalf("surface vector widths deck=%v trench=%v", tile.Parts[0].LineWidth, tile.Parts[1].LineWidth)
	}
	if tile.Parts[1].Color != deathStarTrenchLine || horizon.Parts[1].Color != deathStarTrenchLine {
		t.Fatalf("trench line treatment differs: near=%v horizon=%v want=%v", tile.Parts[1].Color, horizon.Parts[1].Color, deathStarTrenchLine)
	}
	if deathStarTrenchLine.R >= deathStarSurfaceLine.R || deathStarTrenchLine.R <= trenchFill.R {
		t.Fatalf("trench line=%v should be subdued but remain legible over fill=%v", deathStarTrenchLine, trenchFill)
	}
}

func TestDeathStarDeckTextureUVsUseWorldAnchoredMaterialScales(t *testing.T) {
	definition := DeathStarTrench()
	near := definition.Tile(TileCoordinate{X: 1})
	nearMinU, nearMaxU, nearMinV, nearMaxV := deckUVBounds(near.Parts[0].Mesh)
	if math.Abs(nearMaxU-nearMinU-1) > 1e-9 || math.Abs(nearMaxV-nearMinV-1) > 1e-9 {
		t.Fatalf("near deck UV span=(%f,%f), want sixteen source sub-panels per structural tile", nearMaxU-nearMinU, nearMaxV-nearMinV)
	}
	if got, want := len(near.Parts[0].Mesh.Faces), 1; got != want {
		t.Fatalf("quiet near deck fill faces=%d, want %d", got, want)
	}

	horizon := definition.HorizonTile(TileCoordinate{X: 1})
	horizonUVs := horizon.Parts[0].Mesh.Faces[0].UVs
	if len(horizonUVs) != 4 || math.Abs(horizonUVs[1].U-horizonUVs[0].U-1) > 1e-9 || math.Abs(horizonUVs[0].V-horizonUVs[3].V-1) > 1e-9 {
		t.Fatalf("horizon deck UV scale=%+v, want the shared exterior pattern scale", horizonUVs)
	}

	trench := definition.Tile(TileCoordinate{})
	trenchUVs := trench.Parts[1].Mesh.Faces[0].UVs
	if len(trenchUVs) != 4 || trenchUVs[0] == trenchUVs[1] || trenchUVs[1] == trenchUVs[2] {
		t.Fatalf("trench wall has no meaningful material UVs: %+v", trenchUVs)
	}
}

func TestDeathStarDeckPanelsShareTextureCoordinatesAtTileBoundaries(t *testing.T) {
	definition := DeathStarTrench()
	leftMinU, leftMaxU, leftMinV, leftMaxV := deckUVBounds(definition.Tile(TileCoordinate{X: 1, Z: 1}).Parts[0].Mesh)
	rightMinU, rightMaxU, rightMinV, rightMaxV := deckUVBounds(definition.Tile(TileCoordinate{X: 2, Z: 1}).Parts[0].Mesh)
	if leftMaxU != rightMinU || leftMinV != rightMinV || leftMaxV != rightMaxV || rightMaxU-leftMinU != 2 {
		t.Fatalf("adjacent exterior tiles reset their panel family pattern: left=(%f,%f,%f,%f) right=(%f,%f,%f,%f)", leftMinU, leftMaxU, leftMinV, leftMaxV, rightMinU, rightMaxU, rightMinV, rightMaxV)
	}
	horizon := definition.HorizonTile(TileCoordinate{X: 3, Z: 1}).Parts[0]
	if horizon.Surface.TextureID != deathStarDeckPanelFamily.TextureID || horizon.Surface != deathStarSurfaceTexture {
		t.Fatalf("horizon deck uses a different exterior panel material: %+v", horizon.Surface)
	}
}

func deckUVBounds(mesh model.Model) (minU, maxU, minV, maxV float64) {
	minU, minV = math.Inf(1), math.Inf(1)
	maxU, maxV = math.Inf(-1), math.Inf(-1)
	for _, face := range mesh.Faces {
		for _, uv := range face.UVs {
			minU, maxU = math.Min(minU, uv.U), math.Max(maxU, uv.U)
			minV, maxV = math.Min(minV, uv.V), math.Max(maxV, uv.V)
		}
	}
	return minU, maxU, minV, maxV
}

func TestDeathStarNearDeckUsesReducedVectorGrid(t *testing.T) {
	deck := DeathStarTrench().Tile(TileCoordinate{X: 1}).Parts[0].Mesh
	// Four divisions give 40 decorative grid segments. The single physical
	// face contributes its four boundary edges, rather than 16 textured face
	// subdivisions introducing an additional internal edge network.
	if got, want := len(deck.Edges), 44; got != want {
		t.Fatalf("near deck edge count=%d, want %d", got, want)
	}
}

func TestDeathStarHorizonTilesAreVisualOnly(t *testing.T) {
	definition := DeathStarTrench()
	if definition.HorizonTile == nil || definition.HorizonTileRadius <= definition.TileRadius {
		t.Fatalf("horizon tile configuration=%+v", definition)
	}
	tile := definition.HorizonTile(TileCoordinate{X: 1, Z: 0})
	if len(tile.Parts) == 0 || !tile.Bounds.Valid() {
		t.Fatalf("horizon tile has no prepared visual geometry: %+v", tile)
	}
	if len(tile.Features) != 0 || len(tile.Planes) != 0 || len(tile.Boxes) != 0 {
		t.Fatalf("horizon tile retained physical content: features=%d planes=%d boxes=%d", len(tile.Features), len(tile.Planes), len(tile.Boxes))
	}
	for _, part := range tile.Parts {
		if !part.Mesh.SkipDepth || !part.Mesh.DepthTestOnly || part.Mesh.PointOccluder || part.SelfOccluding || part.SelfOcclusion != scene.SelfOcclusionNone {
			t.Fatalf("horizon part does not use line-test-only depth: %+v", part)
		}
	}
	deck := tile.Parts[0]
	for _, edge := range deck.Mesh.Edges {
		if edge.Kind == model.EdgeDecorative {
			t.Fatalf("horizon deck retained CPU vector grid edge: %+v", edge)
		}
	}
	if len(deck.Mesh.Verts) != 4 {
		t.Fatalf("horizon deck verts=%d, want texture quad", len(deck.Mesh.Verts))
	}
	if len(tile.Parts) != 2 {
		t.Fatalf("ordinary horizon parts=%d, want deck plus baked furniture", len(tile.Parts))
	}
	furniture := tile.Parts[1]
	if furniture.Name != "distant low-rise surface furniture" || furniture.Surface != deathStarPassiveInstallationSurface || len(furniture.Mesh.Faces) == 0 {
		t.Fatalf("horizon furniture is missing its low-rise material geometry: %+v", furniture)
	}
	// Seven simplified groups ensure a horizon tile reads as populated surface
	// infrastructure rather than a mostly empty deck with an occasional prop.
	if got, want := len(furniture.Mesh.Faces), 84; got != want {
		t.Fatalf("horizon furniture faces=%d, want dense seven-group layout with %d", got, want)
	}
	if furniture.Surface.Directional != deathStarFixtureDirectionalShading || !furniture.Surface.Directional.Enabled() {
		t.Fatalf("horizon furniture lost the shared fixture light: %+v", furniture.Surface.Directional)
	}
	for _, edge := range furniture.Mesh.Edges {
		if edge.Kind != model.EdgeInternal {
			t.Fatalf("horizon furniture emitted a distant vector edge: %+v", edge)
		}
	}
}

func TestDeathStarHorizonTrenchFurnitureStaysAboveSideDecks(t *testing.T) {
	tile := DeathStarTrench().HorizonTile(TileCoordinate{X: trenchTileX, Z: 0})
	if len(tile.Parts) != 3 {
		t.Fatalf("trench horizon parts=%d, want side decks, trench, and furniture", len(tile.Parts))
	}
	furniture := tile.Parts[2]
	for vertexIndex, vertex := range furniture.Mesh.Verts {
		if math.Abs(vertex.X) <= deathStarTrenchHalf {
			t.Fatalf("horizon trench furniture vertex %d enters corridor: %+v", vertexIndex, vertex)
		}
		if vertex.Y < -1e-9 {
			t.Fatalf("horizon trench furniture vertex %d enters trench wall/floor: %+v", vertexIndex, vertex)
		}
	}
}

func TestDeathStarDepthProxyCollapsesActiveDeckTiles(t *testing.T) {
	definition := DeathStarTrench()
	if definition.DepthProxy == nil {
		t.Fatal("Death Star surface has no coarse depth proxy")
	}
	coordinates := make([]TileCoordinate, 0, 25)
	for x := -2; x <= 2; x++ {
		for z := -2; z <= 2; z++ {
			coordinates = append(coordinates, TileCoordinate{X: x, Z: z})
		}
	}
	proxies := definition.DepthProxy(coordinates)
	if len(proxies) != 1 {
		t.Fatalf("depth proxy meshes=%d, want one merged surface", len(proxies))
	}
	proxy := proxies[0]
	if err := proxy.Validate(); err != nil {
		t.Fatalf("depth proxy is invalid: %v", err)
	}
	// Twenty-five deck patches plus their trench walls would previously enter
	// the raster pass independently. The merged proxy retains the deck opening
	// and trench surfaces in at most seven broad faces.
	if len(proxy.Faces) == 0 || len(proxy.Faces) > 8 {
		t.Fatalf("depth proxy faces=%d, want 1..8 merged faces", len(proxy.Faces))
	}
	deckFaces := 0
	for faceIndex, face := range proxy.Faces {
		topDeck := !face.DoubleSided
		for _, vertexIndex := range face.Vertices {
			if math.Abs(proxy.Verts[vertexIndex].Y) > 1e-9 {
				topDeck = false
				break
			}
		}
		if !topDeck {
			continue
		}
		deckFaces++
		if face.Normal.Y < 0.99 {
			t.Fatalf("depth proxy deck face %d normal=%+v, want upward-facing occluder", faceIndex, face.Normal)
		}
	}
	if deckFaces == 0 {
		t.Fatal("depth proxy has no upward-facing deck occluders")
	}
	tile := definition.Tile(TileCoordinate{})
	for _, part := range tile.Parts {
		if part.Name == "surface deck" || part.Name == "trench" {
			if !part.DepthWriteProxy {
				t.Fatalf("%s did not opt into the environment depth proxy", part.Name)
			}
		}
	}
}

func TestDeathStarTrenchOpaqueFillsUseLocalPainterCells(t *testing.T) {
	tile := DeathStarTrench().Tile(TileCoordinate{})
	deck, trench := tile.Parts[0].Mesh, tile.Parts[1].Mesh
	if want := 2 * deathStarTrenchFillColumns * deathStarTrenchFillRows; len(deck.Faces) != want {
		t.Fatalf("trench side-deck faces=%d, want %d local fill cells", len(deck.Faces), want)
	}
	if want := 3 * deathStarTrenchFillRows; len(trench.Faces) != want {
		t.Fatalf("trench faces=%d, want %d local wall/floor cells", len(trench.Faces), want)
	}
	maxSpan := deathStarTileSize/float64(deathStarTrenchFillRows) + 1e-9
	for _, check := range []struct {
		name string
		mesh model.Model
	}{
		{name: "side deck", mesh: deck},
		{name: "trench", mesh: trench},
	} {
		for faceIndex, face := range check.mesh.Faces {
			if len(face.UVs) != len(face.Vertices) {
				t.Fatalf("%s face %d lost valid material geometry: %+v", check.name, faceIndex, face)
			}
			minZ, maxZ := math.Inf(1), math.Inf(-1)
			for _, vertex := range face.Vertices {
				z := check.mesh.Verts[vertex].Z
				minZ, maxZ = min(minZ, z), max(maxZ, z)
			}
			if maxZ-minZ > maxSpan {
				t.Fatalf("%s face %d spans %.3f metres in depth, want <= %.3f", check.name, faceIndex, maxZ-minZ, maxSpan)
			}
		}
	}
}

func TestDeathStarTrenchRouteRequiresForwardPhysicalProgress(t *testing.T) {
	entry := DeathStarTrenchEntryPoint()
	if !DeathStarTrenchEntryContains(entry) || DeathStarTrenchEntryContains(math3d.Vec3{Y: -4, Z: 150}) {
		t.Fatalf("trench entry volume accepted the wrong route: entry=%+v", entry)
	}
	checkpoints := DeathStarTrenchCheckpoints()
	if len(checkpoints) < 2 {
		t.Fatalf("trench route is too short: %v", checkpoints)
	}
	previous := entry
	current := previous
	current.Z = checkpoints[0] + 1
	if !DeathStarTrenchCheckpointCrossed(0, previous, current) {
		t.Fatal("forward traversal did not cross first trench gate")
	}
	if DeathStarTrenchCheckpointCrossed(1, current, previous) {
		t.Fatal("reverse traversal crossed a trench gate")
	}
	if DeathStarTrenchCheckpointCrossed(0, math3d.Vec3{Y: 4, Z: checkpoints[0] - 1}, current) {
		t.Fatal("surface flight crossed a trench gate")
	}
	if !DeathStarTrenchEntryCrossed(entry.Add(math3d.Vec3{Y: 6}), entry) {
		t.Fatal("surface descent through the marked trench mouth did not register")
	}
	if DeathStarTrenchEntryCrossed(entry, entry) {
		t.Fatal("already being inside the trench registered as a new entry")
	}
}

func TestDeathStarTrenchEntryEnvelopeMarksTheLegalMouth(t *testing.T) {
	tile := DeathStarTrench().Tile(TileCoordinate{X: trenchTileX, Z: trenchFirstTileZ})
	foundEnvelope, foundArrows := false, false
	for _, part := range tile.Parts {
		switch part.Name {
		case "trench entry envelope":
			if len(part.Mesh.Edges) != 6 || part.Surface.Filled() || part.Color.A == 0 {
				t.Fatalf("entry envelope=%+v, want visible six-line approach marker", part)
			}
			foundEnvelope = true
		case "trench entry direction arrows":
			if len(part.Mesh.Edges) != 21 || part.Color.R < 220 || part.Color.G > 64 || part.Color.B > 64 {
				t.Fatalf("entry arrows=%+v, want three bright red floor arrows", part)
			}
			foundArrows = true
		}
	}
	if !foundEnvelope || !foundArrows {
		t.Fatalf("trench mouth markers envelope=%v arrows=%v", foundEnvelope, foundArrows)
	}
}

func TestDeathStarTrenchProvidesAttackRunAndClimbOutPastThePort(t *testing.T) {
	checkpoints := DeathStarTrenchCheckpoints()
	port := DeathStarExhaustPortPoint()
	if len(checkpoints) == 0 || port.Z-checkpoints[len(checkpoints)-1] < DeathStarExhaustPortFiringRange()+10 {
		t.Fatalf("attack run is too short: checkpoints=%v port=%+v", checkpoints, port)
	}
	attackGate := checkpoints[len(checkpoints)-1]
	if progress := DeathStarTrenchProgress(math3d.Vec3{Z: attackGate}); progress < DeathStarTrenchRequiredProgress()-1e-9 {
		t.Fatalf("attack gate progress=%v, want at least %v", progress, DeathStarTrenchRequiredProgress())
	}
	escape := DeathStarTrenchEscapePoint()
	if escape.Y <= 53 || escape.Z <= port.Z || escape.Z >= (float64(trenchLastTileZ)+.5)*deathStarTileSize {
		t.Fatalf("escape lane=%+v is not above and beyond port=%+v", escape, port)
	}
}

func TestDeathStarSurfaceFeaturesAreOpaque(t *testing.T) {
	tile := DeathStarTrench().Tile(TileCoordinate{X: 2, Z: 0})
	if len(tile.Features) == 0 {
		t.Fatal("ordinary surface tile has no features")
	}
	for _, feature := range tile.Features {
		prototype, known := installationPrototypes[feature.Kind]
		if !known {
			continue
		}
		if len(feature.Parts) == 0 {
			t.Fatalf("%s feature has no parts", feature.Kind)
		}
		for partIndex, part := range feature.Parts {
			if !part.SelfOccluding || part.SelfOcclusion != scene.SelfOcclusionAll {
				t.Fatalf("%s part %q is not fully self-occluding", feature.Kind, part.Name)
			}
			isHangarBay := feature.Kind == "quarter-hangar" && partIndex == 1
			expectTextured := (feature.Kind != "cannon" || partIndex != feature.BarrelPart) && !isHangarBay
			expectedSurface := deathStarInstallationSurface
			if prototype.visualOnly {
				expectedSurface = deathStarPassiveInstallationSurface
			}
			if isHangarBay {
				expectedSurface = deathStarHangarBaySurface
			}
			if (expectTextured || isHangarBay) && part.Surface != expectedSurface {
				t.Fatalf("%s part %q does not use the installation material", feature.Kind, part.Name)
			}
			if !expectTextured && !isHangarBay && part.Surface != deathStarInstallationBarrelSurface {
				t.Fatalf("cannon barrel part lost its readable flat material: %+v", part.Surface)
			}
			if isHangarBay {
				if part.Color != deathStarHangarBayLine || part.Surface.Directional.Enabled() {
					t.Fatalf("quarter hangar bay lost its dark recessed treatment: %+v", part)
				}
				continue
			}
			if part.Surface.Directional != deathStarFixtureDirectionalShading || !part.Surface.Directional.Enabled() {
				t.Fatalf("%s part %q does not share the fixture light: %+v", feature.Kind, part.Name, part.Surface.Directional)
			}
			if part.Color.R != part.Color.G || part.Color.G != part.Color.B {
				t.Fatalf("%s part %q does not use a neutral-grey line palette", feature.Kind, part.Name)
			}
			if expectTextured {
				if part.Color != deathStarInstallationLine || part.LineWidth != deathStarInstallationLineWidth {
					t.Fatalf("%s part %q retained a bright polygon outline: color=%+v width=%v", feature.Kind, part.Name, part.Color, part.LineWidth)
				}
				for faceIndex, face := range part.Mesh.Faces {
					if len(face.UVs) != len(face.Vertices) {
						t.Fatalf("%s part %q face %d has invalid installation UVs: %+v", feature.Kind, part.Name, faceIndex, face.UVs)
					}
				}
			}
		}
	}
}

func TestCannonHasSeparateTraversingPartsAndConservativeBounds(t *testing.T) {
	tile := DeathStarTrench().Tile(TileCoordinate{X: 2, Z: 0})
	for _, feature := range tile.Features {
		if feature.Kind != "cannon" {
			continue
		}
		if len(feature.Parts) != 3 || feature.TurretPart != 1 || feature.BarrelPart != 2 || len(feature.Muzzles) != 2 {
			t.Fatalf("cannon articulation is incomplete: parts=%d turret=%d barrels=%d muzzles=%d",
				len(feature.Parts), feature.TurretPart, feature.BarrelPart, len(feature.Muzzles))
		}
		if !feature.Bounds.Valid() {
			t.Fatal("articulated cannon has no aggregate bounds")
		}
		if feature.VisualTurretPivot != (math3d.Vec3{Y: 0.35}) || feature.VisualBarrelPivot != (math3d.Vec3{Y: 0.4225, Z: 0.068}) {
			t.Fatalf("cannon visual pivots=%+v/%+v, want tall-emplacement pivots", feature.VisualTurretPivot, feature.VisualBarrelPivot)
		}
		initial := feature.Matrix().Mul(feature.TurretMatrix(0, 0)).TransformPoint(feature.Muzzles[0])
		turned := feature.Matrix().Mul(feature.TurretMatrix(0.5, -0.3)).TransformPoint(feature.Muzzles[0])
		if initial.Sub(turned).Length() < 0.1 {
			t.Fatal("articulated muzzle did not move under yaw and pitch")
		}
		visualMount := feature.VisualTurretMatrix(0.5, -0.3).TransformPoint(math3d.Vec3{Y: 0.4225, Z: 0.068})
		combatMount := feature.TurretMatrix(0.5, -0.3).TransformPoint(math3d.Vec3{Y: 0.4225, Z: 0.068})
		if visualMount.Sub(combatMount).Length() < 1e-6 {
			t.Fatal("tall cannon visual articulation did not separate from combat anchors")
		}
		return
	}
	t.Fatal("ordinary Death Star tile has no cannon")
}

func TestTurbolaserFoundationHasThreeTaperedCoursesAndFaceRibs(t *testing.T) {
	foundation := turbolaserFoundation()
	if err := foundation.Validate(); err != nil {
		t.Fatalf("turbolaser foundation: %v", err)
	}

	// The broad-to-narrow course measurements are authored in the shared
	// prototype and must remain inside the pre-existing conservative collider.
	for _, want := range []struct {
		y, halfX, halfZ float64
	}{
		{-0.50, 0.42, 0.39},
		{-0.26, 0.225, 0.209},
		{0.025, 0.115, 0.107},
		{0.350, 0.10, 0.092},
	} {
		var halfX, halfZ float64
		for _, vertex := range foundation.Verts {
			if math.Abs(vertex.Y-want.y) > 1e-9 {
				continue
			}
			halfX = math.Max(halfX, math.Abs(vertex.X))
			halfZ = math.Max(halfZ, math.Abs(vertex.Z))
		}
		if math.Abs(halfX-want.halfX) > 1e-9 || math.Abs(halfZ-want.halfZ) > 1e-9 {
			t.Fatalf("course at y=%v has half extents (%v, %v), want (%v, %v)", want.y, halfX, halfZ, want.halfX, want.halfZ)
		}
	}

	decorative := 0
	for _, edge := range foundation.Edges {
		if edge.Kind == model.EdgeDecorative {
			decorative++
		}
	}
	if decorative != 3*4*4 {
		t.Fatalf("foundation decorative ribs=%d, want %d", decorative, 3*4*4)
	}
	for faceIndex, face := range foundation.Faces {
		if len(face.UVs) != len(face.Vertices) {
			t.Fatalf("foundation face %d has invalid UVs: %+v", faceIndex, face.UVs)
		}
	}
}

func TestTurbolaserHeadIsCompactCubeWithShortBarrels(t *testing.T) {
	head := turbolaserHead()
	if err := head.Validate(); err != nil {
		t.Fatalf("turbolaser head: %v", err)
	}
	if len(head.Faces) != 5 {
		t.Fatalf("turbolaser head faces=%d, want a bottom-open cube with 5 visible faces", len(head.Faces))
	}
	minimum := math3d.Vec3{X: math.Inf(1), Y: math.Inf(1), Z: math.Inf(1)}
	maximum := math3d.Vec3{X: math.Inf(-1), Y: math.Inf(-1), Z: math.Inf(-1)}
	for _, vertex := range head.Verts {
		minimum.X, minimum.Y, minimum.Z = math.Min(minimum.X, vertex.X), math.Min(minimum.Y, vertex.Y), math.Min(minimum.Z, vertex.Z)
		maximum.X, maximum.Y, maximum.Z = math.Max(maximum.X, vertex.X), math.Max(maximum.Y, vertex.Y), math.Max(maximum.Z, vertex.Z)
	}
	if math.Abs(minimum.X+0.0725) > 1e-9 || math.Abs(maximum.X-0.0725) > 1e-9 ||
		math.Abs(minimum.Y-0.35) > 1e-9 || math.Abs(maximum.Y-0.495) > 1e-9 ||
		math.Abs(minimum.Z+0.0725) > 1e-9 || math.Abs(maximum.Z-0.0725) > 1e-9 {
		t.Fatalf("turbolaser head bounds=(%+v, %+v), want compact cube", minimum, maximum)
	}
	for _, vertex := range head.Verts {
		if math.Abs(vertex.Y-0.495) > 1e-9 {
			continue
		}
		if math.Abs(vertex.X) > 0.065+1e-9 || math.Abs(vertex.Z) > 0.065+1e-9 {
			t.Fatalf("turbolaser roof vertex=%+v, want the roof inset from its base", vertex)
		}
	}
	prototype := installationPrototypes["cannon"]
	if got, want := prototype.muzzles[0], (math3d.Vec3{X: -0.19, Y: 0.20, Z: 0.91}); got != want {
		t.Fatalf("left muzzle=%+v, want %+v", got, want)
	}
}

func TestTurbolaserBarrelsUseOpenBackedHexagonalPrisms(t *testing.T) {
	barrel := turbolaserHexBarrel(math3d.Vec3{Z: 0.12}, 0.0115, 0.10)
	if err := barrel.Validate(); err != nil {
		t.Fatalf("hex barrel: %v", err)
	}
	if len(barrel.Verts) != 12 || len(barrel.Edges) != 18 || len(barrel.Faces) != 7 {
		t.Fatalf("hex barrel geometry verts=%d edges=%d faces=%d, want 12/18/7", len(barrel.Verts), len(barrel.Edges), len(barrel.Faces))
	}
	front := barrel.Faces[len(barrel.Faces)-1]
	if len(front.Vertices) != 6 || front.Normal.Dot(math3d.Vec3{Z: 1}) < 0.99 {
		t.Fatalf("hex barrel has invalid forward cap: %+v", front)
	}
	for _, face := range barrel.Faces[:len(barrel.Faces)-1] {
		if len(face.Vertices) != 4 {
			t.Fatalf("hex barrel side face has %d vertices, want 4", len(face.Vertices))
		}
	}
}

func TestTurbolaserInspectionReusesSurfaceCannonArtwork(t *testing.T) {
	inspection := TurbolaserInspection(91, kinematics.Pose{})
	if err := inspection.Validate(); err != nil {
		t.Fatalf("turbolaser inspection object: %v", err)
	}
	prototype := installationPrototypes["cannon"]
	if inspection.CollisionRole != scene.CollisionNone || inspection.Physical || inspection.Hittable {
		t.Fatalf("inspection object accidentally acquired gameplay state: %+v", inspection)
	}
	if len(inspection.Parts) != len(prototype.parts) {
		t.Fatalf("inspection parts=%d, production cannon parts=%d", len(inspection.Parts), len(prototype.parts))
	}
	for index := range prototype.parts {
		if inspection.Parts[index].Mesh.Topology != prototype.parts[index].Mesh.Topology {
			t.Fatalf("inspection part %d does not share production cannon topology", index)
		}
	}
}

func TestDeathStarTilesProvideAggregateBoundsAndSharedFeatureTopology(t *testing.T) {
	definition := DeathStarTrench()
	first := definition.Tile(TileCoordinate{X: 2, Z: 0})
	second := definition.Tile(TileCoordinate{X: 3, Z: 0})
	if !first.Bounds.Valid() || !second.Bounds.Valid() {
		t.Fatalf("tile bounds are not prepared: first=%+v second=%+v", first.Bounds, second.Bounds)
	}
	if len(first.Features) == 0 || len(second.Features) == 0 {
		t.Fatal("surface tiles have no reusable feature instances")
	}
	for _, feature := range first.Features {
		if !feature.Bounds.Valid() || feature.Scale == (math3d.Vec3{}) {
			t.Fatalf("feature %q lacks local bounds or instance scale: %+v", feature.ID, feature)
		}
		wantDetail := scene.DetailMedium
		if prototype, known := installationPrototypes[feature.Kind]; known && prototype.visualOnly {
			wantDetail = scene.DetailPrimary
		}
		if feature.Detail != wantDetail {
			t.Fatalf("feature %q detail=%v, want %v", feature.ID, feature.Detail, wantDetail)
		}
	}
	if first.Features[0].Parts[0].Mesh.Topology != second.Features[0].Parts[0].Mesh.Topology {
		t.Fatal("repeated installations do not share immutable model topology")
	}
}

func TestDeathStarInstallationCatalogHasDistinctValidSolidShapes(t *testing.T) {
	seen := make(map[string]bool)
	for x := -2; x <= 2; x++ {
		for z := -2; z <= 2; z++ {
			tile := DeathStarTrench().Tile(TileCoordinate{X: x, Z: z})
			// Four active installations (including the occasional companion
			// aerial) plus eleven passive pieces is the authored density cap.
			if len(tile.Features) > 15 {
				t.Fatalf("tile %+v expanded to %d features", tile.Coordinate, len(tile.Features))
			}
			for _, feature := range tile.Features {
				if feature.Kind == "exhaust-port" {
					continue
				}
				seen[feature.Kind] = true
				prototype := installationPrototypes[feature.Kind]
				if prototype.visualOnly {
					if feature.Targetable || feature.Hittable || feature.HitPoints != 0 || len(feature.Boxes) != 0 || len(feature.WreckParts) != 0 {
						t.Fatalf("passive scenery retained combat state: %+v", feature)
					}
				} else if feature.HitPoints < 1 || len(feature.Boxes) == 0 || len(feature.WreckParts) == 0 {
					t.Fatalf("%s lacks damage, collision or wreck definition: %+v", feature.Kind, feature)
				}
				if feature.Kind == "cannon" && (feature.DisableAfter <= 0 || len(feature.Muzzles) != 2) {
					t.Fatalf("cannon lacks disabled state or paired physical muzzles: %+v", feature)
				}
				for _, part := range feature.Parts {
					if err := part.Validate(); err != nil || !part.Surface.Opaque() || len(part.Mesh.Faces) == 0 {
						t.Fatalf("%s has invalid solid part %q: %v", feature.Kind, part.Name, err)
					}
					if len(part.Mesh.Faces) > 40 {
						t.Fatalf("%s exceeds sparse vector budget: %d faces", feature.Kind, len(part.Mesh.Faces))
					}
				}
				for _, box := range feature.Boxes {
					if string(box.FeatureID) != feature.ID || box.HalfExtents.X <= 0 || box.HalfExtents.Y <= 0 || box.HalfExtents.Z <= 0 {
						t.Fatalf("%s has unaligned collider %+v", feature.ID, box)
					}
				}
			}
		}
	}
	for _, kind := range []string{"tower", "cannon", "antenna", "vent", "equipment-bank", "vent-cluster", "power-pod", "service-conduit", "deck-cluster", "quarter-hangar"} {
		if !seen[kind] {
			t.Errorf("surface layout never generated %s", kind)
		}
	}
}

func TestDeathStarPassiveSceneryIsDeterministicAndNonInteractive(t *testing.T) {
	cases := []struct {
		name       string
		coordinate TileCoordinate
		minimum    int
		maximum    int
		trench     bool
	}{
		{name: "ordinary deck", coordinate: TileCoordinate{X: 2, Z: 0}, minimum: 11, maximum: 11},
		{name: "hangar surround", coordinate: hangarSurfaceTile, minimum: 11, maximum: 11},
		{name: "trench side decks", coordinate: TileCoordinate{X: trenchTileX, Z: 0}, minimum: 4, maximum: 4, trench: true},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			first := DeathStarTrench().Tile(testCase.coordinate)
			second := DeathStarTrench().Tile(testCase.coordinate)
			firstPassive := passiveSceneryFeatures(first.Features)
			secondPassive := passiveSceneryFeatures(second.Features)
			if len(firstPassive) < testCase.minimum || len(firstPassive) > testCase.maximum {
				t.Fatalf("passive feature count=%d, want [%d,%d]", len(firstPassive), testCase.minimum, testCase.maximum)
			}
			if len(firstPassive) != len(secondPassive) {
				t.Fatalf("regenerated passive feature count=%d, want %d", len(secondPassive), len(firstPassive))
			}
			for index, feature := range firstPassive {
				other := secondPassive[index]
				prototype := installationPrototypes[feature.Kind]
				if !prototype.visualOnly {
					t.Fatalf("%s is not marked visual-only", feature.Kind)
				}
				if feature.ID != other.ID || feature.Kind != other.Kind || feature.Pose != other.Pose || feature.Scale != other.Scale {
					t.Fatalf("passive feature %d is not deterministic: %+v vs %+v", index, feature, other)
				}
				if feature.Targetable || feature.Hittable || feature.HitPoints != 0 || len(feature.Boxes) != 0 || len(feature.WreckParts) != 0 {
					t.Fatalf("passive feature %s carries gameplay state: %+v", feature.ID, feature)
				}
				if feature.Detail != scene.DetailPrimary {
					t.Fatalf("passive feature %s detail=%v, want primary for active-tile coverage", feature.ID, feature.Detail)
				}
				if feature.Kind == "deck-cluster" && !testCase.trench {
					centerX := float64(testCase.coordinate.X) * deathStarTileSize
					centerZ := float64(testCase.coordinate.Z) * deathStarTileSize
					if math.Abs(feature.Pose.Position.X-centerX) > 1 || math.Abs(feature.Pose.Position.Z-centerZ) > 1 {
						t.Fatalf("deck cluster %s is not centred in its tile: position=%+v centre=(%v,%v)", feature.ID, feature.Pose.Position, centerX, centerZ)
					}
					if got, want := len(feature.Parts[0].Mesh.Faces), 40; got != want {
						t.Fatalf("deck cluster faces=%d, want %d low-rise faces", got, want)
					}
				}
				if math.Abs(feature.Pose.Position.Y-feature.Scale.Y/2) > 1e-12 {
					t.Fatalf("passive feature %s does not rest on deck: position=%+v scale=%+v", feature.ID, feature.Pose.Position, feature.Scale)
				}
				if len(feature.Parts) != len(prototype.parts) {
					t.Fatalf("passive feature %s did not reuse its shared prototype topology", feature.ID)
				}
				for partIndex := range feature.Parts {
					if feature.Parts[partIndex].Mesh.Topology != prototype.parts[partIndex].Mesh.Topology {
						t.Fatalf("passive feature %s part %d did not reuse shared prototype topology", feature.ID, partIndex)
					}
				}
				if testCase.trench {
					bounds := transformRenderBounds(feature.Bounds, feature.Matrix())
					if math.Abs(bounds.Center.X)-bounds.Radius <= deathStarTrenchHalf {
						t.Fatalf("passive feature %s intrudes into trench route: bounds=%+v", feature.ID, bounds)
					}
				}
			}
		})
	}
}

func TestQuarterCircleHangarHasFourSegmentedRoofSections(t *testing.T) {
	prototype, ok := installationPrototypes["quarter-hangar"]
	if !ok || !prototype.visualOnly || len(prototype.parts) != 2 {
		t.Fatalf("quarter hangar prototype=%+v, want shell plus recessed bay", prototype)
	}
	shell, bay := prototype.parts[0].Mesh, prototype.parts[1].Mesh
	if err := shell.Validate(); err != nil {
		t.Fatalf("quarter hangar shell is invalid: %v", err)
	}
	if err := bay.Validate(); err != nil {
		t.Fatalf("quarter hangar bay is invalid: %v", err)
	}
	if got, want := len(shell.Faces), 10; got != want {
		t.Fatalf("quarter hangar shell faces=%d, want %d for four roofs, four fascia sections, and two bulkheads", got, want)
	}
	if got, want := len(bay.Faces), 4; got != want {
		t.Fatalf("quarter hangar bay faces=%d, want %d recessed sections", got, want)
	}
	roofs := 0
	for _, face := range shell.Faces {
		if face.Normal.Y <= 0.1 {
			continue
		}
		roofs++
	}
	if roofs != 4 {
		t.Fatalf("quarter hangar roofs=%d, want four sections", roofs)
	}
	for section, face := range bay.Faces {
		if !face.DoubleSided {
			t.Fatalf("quarter hangar bay section %d is not visible from the open side", section)
		}
	}
	// Its footprint must remain that of a long oblong service building bent
	// through a quarter turn, not regress to a broad compact radial bay.
	innerRadius, outerRadius := math.Inf(1), 0.0
	for _, vertex := range shell.Verts {
		if vertex.Y < 0 {
			continue
		}
		radius := math.Hypot(vertex.X, vertex.Z)
		innerRadius = math.Min(innerRadius, radius)
		outerRadius = math.Max(outerRadius, radius)
	}
	width := outerRadius - innerRadius
	centreLineLength := math.Pi * (outerRadius + innerRadius) / 4
	if centreLineLength < width*5 {
		t.Fatalf("quarter hangar run=%v width=%v, want an oblong quarter-bent structure", centreLineLength, width)
	}
}

func TestQuarterCircleHangarOccupiesClearRearPortTileCorner(t *testing.T) {
	tile := DeathStarTrench().Tile(TileCoordinate{X: 1})
	var hangar *Feature
	hangarCount := 0
	for index := range tile.Features {
		if tile.Features[index].Kind == "quarter-hangar" {
			hangarCount++
			if hangar == nil {
				hangar = &tile.Features[index]
			}
		}
	}
	if hangar == nil {
		t.Fatal("ordinary surface tile has no quarter hangar")
	}
	if hangarCount != 3 {
		t.Fatalf("ordinary surface tile has %d quarter hangars, want three curved structures", hangarCount)
	}
	localPosition := hangar.Pose.Position.Sub(math3d.Vec3{X: deathStarTileSize})
	if localPosition.X >= -12 || localPosition.Z <= 12 {
		t.Fatalf("quarter hangar position=%+v, want clear rear-port corner", hangar.Pose.Position)
	}
	if hangar.Scale.Y < 4 {
		t.Fatalf("quarter hangar vertical scale=%v, want readable stepped roof", hangar.Scale.Y)
	}
}

func passiveSceneryFeatures(features []Feature) []Feature {
	passive := make([]Feature, 0, len(features))
	for _, feature := range features {
		if prototype, known := installationPrototypes[feature.Kind]; known && prototype.visualOnly {
			passive = append(passive, feature)
		}
	}
	return passive
}

func TestSteppedInstallationHasOutwardFaceWinding(t *testing.T) {
	mesh := steppedInstallation(installationRing{-0.5, 0.4, 0.4}, installationRing{0.5, 0.4, 0.4})
	if err := mesh.Validate(); err != nil {
		t.Fatalf("unit installation: %v", err)
	}
	want := []math3d.Vec3{{Z: -1}, {X: 1}, {Z: 1}, {X: -1}, {Y: -1}, {Y: 1}}
	if len(mesh.Topology.FaceNormals) != len(want) {
		t.Fatalf("face normals=%d, want %d", len(mesh.Topology.FaceNormals), len(want))
	}
	for index, normal := range mesh.Topology.FaceNormals {
		if normal.Dot(want[index]) < 0.99 {
			t.Errorf("face %d normal=%+v, want %+v", index, normal, want[index])
		}
	}
}

func TestDeathStarInstallationLayoutAndIDsSurviveTileRegeneration(t *testing.T) {
	coordinate := TileCoordinate{X: 1, Z: 3}
	first := DeathStarTrench().Tile(coordinate)
	second := DeathStarTrench().Tile(coordinate)
	if len(first.Features) != len(second.Features) {
		t.Fatalf("feature counts changed: %d vs %d", len(first.Features), len(second.Features))
	}
	for index, feature := range first.Features {
		other := second.Features[index]
		if feature.ID != other.ID || feature.Kind != other.Kind || feature.Pose != other.Pose || feature.Scale != other.Scale {
			t.Fatalf("tile regeneration changed installation %d: %+v vs %+v", index, feature, other)
		}
		if feature.Parts[0].Mesh.Topology != other.Parts[0].Mesh.Topology {
			t.Fatalf("installation %s rebuilt shared topology", feature.ID)
		}
	}
}

func TestDeathStarInstallationsLeaveTrenchRouteOpen(t *testing.T) {
	for z := trenchFirstTileZ; z <= trenchLastTileZ; z++ {
		tile := DeathStarTrench().Tile(TileCoordinate{X: trenchTileX, Z: z})
		for _, feature := range tile.Features {
			if feature.Kind == "exhaust-port" {
				continue
			}
			for _, box := range feature.Boxes {
				// The generated orientation is a quarter-turn; either horizontal
				// half-extent can project across the trench's X boundary.
				if math.Abs(box.Center.X)-math.Max(box.HalfExtents.X, box.HalfExtents.Z) <= 14 {
					t.Fatalf("%s blocks trench at X=%.2f with collider %+v", feature.ID, box.Center.X, box)
				}
			}
		}
	}
}

func TestDeathStarTrenchIsFiniteWithinOrdinarySurface(t *testing.T) {
	definition := DeathStarTrench()
	ordinary := definition.Tile(TileCoordinate{X: 2, Z: 0})
	if len(ordinary.Parts) != 1 || len(ordinary.Planes) != 1 || ordinary.Planes[0].FeatureID != "surface-deck" {
		t.Fatalf("ordinary surface parts=%d planes=%+v", len(ordinary.Parts), ordinary.Planes)
	}
	beyondEnd := definition.Tile(TileCoordinate{X: 0, Z: trenchLastTileZ + 1})
	if len(beyondEnd.Parts) != 1 || beyondEnd.Planes[0].FeatureID != "surface-deck" {
		t.Fatalf("surface beyond trench retained trench geometry: %+v", beyondEnd)
	}
}

func TestDeathStarTrenchPlacesPortBeforeTerminalEscapeLane(t *testing.T) {
	terminalTile := DeathStarTrench().Tile(TileCoordinate{X: trenchTileX, Z: trenchLastTileZ})
	foundEndWall, foundBulkhead := false, false
	for _, plane := range terminalTile.Planes {
		foundEndWall = foundEndWall || plane.FeatureID == "trench-end-wall"
	}
	for _, part := range terminalTile.Parts {
		foundBulkhead = foundBulkhead || part.Name == "trench terminal bulkhead" && part.Surface == deathStarTrenchTexture && len(part.Mesh.Faces) == 1
	}
	portTile := DeathStarTrench().Tile(TileCoordinate{X: trenchTileX, Z: trenchPortTileZ})
	foundPort := false
	for _, feature := range portTile.Features {
		foundPort = foundPort || feature.Kind == "exhaust-port" && feature.Targetable && !feature.Hittable
	}
	if !foundEndWall || !foundBulkhead || !foundPort {
		t.Fatalf("terminal end wall=%t bulkhead=%t exhaust port=%t", foundEndWall, foundBulkhead, foundPort)
	}
}

func TestDeathStarEntryBeginsAboveOrdinarySurface(t *testing.T) {
	definition := DeathStarTrench()
	entry := definition.Transitions[0].EntryPose.Position
	coordinate := TileCoordinate{
		X: int(math.Floor(entry.X/definition.TileSize + 0.5)),
		Z: int(math.Floor(entry.Z/definition.TileSize + 0.5)),
	}
	if coordinate.X == trenchTileX {
		t.Fatalf("entry %+v lies over trench tile %+v", entry, coordinate)
	}
}

func TestDeathStarExitVolumeAllowsClimbBackToSpace(t *testing.T) {
	definition := DeathStarTrench()
	if !definition.ExitVolume.Contains(math3d.Vec3{Y: 20}) {
		t.Fatal("surface flight height is outside the local environment")
	}
	if definition.ExitVolume.Contains(math3d.Vec3{Y: 60}) {
		t.Fatal("climb-away height remains inside the local environment")
	}
}

func TestDeathStarTrenchHasOpenFlyableTop(t *testing.T) {
	tile := DeathStarTrench().Tile(TileCoordinate{})
	trench := tile.Parts[1].Mesh
	for index, face := range trench.Faces {
		if index < 3 && !face.DoubleSided {
			t.Fatalf("trench face %d is not double-sided", index)
		}
	}
	for _, plane := range tile.Planes {
		if plane.FeatureID == "trench-floor" && plane.Center == (math3d.Vec3{Y: -14}) {
			return
		}
	}
	t.Fatal("trench floor missing")
}

func TestDeathStarEnvironmentBindsSeparateFramePerHost(t *testing.T) {
	objects := []scene.Object{
		catalog.DeathStar(10, kinematics.Pose{Orientation: math3d.IdentityQuaternion()}),
		catalog.DeathStar(20, kinematics.Pose{Orientation: math3d.IdentityQuaternion()}),
	}
	bound := Bind(DefaultRegistry(), objects)
	if len(bound) != 4 {
		t.Fatalf("bound environments = %d, want 4", len(bound))
	}
	seen := make(map[scene.FrameID]bool)
	for _, instance := range bound {
		if seen[instance.FrameID] {
			t.Fatalf("bound environments share frame %q", instance.FrameID)
		}
		seen[instance.FrameID] = true
	}
}

func TestDeathStarTrenchFrameIsTangentToNearSurface(t *testing.T) {
	pose := DeathStarTrench().LocalPose
	if got := DeathStarTrench().LevelUp; got != (math3d.Vec3{Y: 1}) {
		t.Fatalf("surface level reference=%+v, want local +Y", got)
	}
	if got := pose.Orientation.Rotate(math3d.Vec3{Y: 1}); got.Sub(math3d.Vec3{Z: -1}).Length() > 1e-9 {
		t.Fatalf("local up maps to %+v, want world -Z", got)
	}
	if got := pose.Orientation.Rotate(math3d.Vec3{Z: 1}); got.Sub(math3d.Vec3{Y: 1}).Length() > 1e-9 {
		t.Fatalf("local forward maps to %+v, want world +Y", got)
	}
}

func TestDeathStarMissionRegionsUsePhysicalTrenchDimensions(t *testing.T) {
	checkpoints := DeathStarTrenchCheckpoints()
	attackGate := checkpoints[len(checkpoints)-1]
	tests := []struct {
		name string
		at   math3d.Vec3
		want DeathStarMissionRegion
	}{
		{"deck", math3d.Vec3{X: 20, Y: 8}, DeathStarSurfaceRegion},
		{"above trench", math3d.Vec3{Y: 4}, DeathStarSurfaceRegion},
		{"trench entrance", math3d.Vec3{Y: -2, Z: -110}, DeathStarTrenchRegion},
		{"attack run", math3d.Vec3{Y: -8, Z: attackGate + 1}, DeathStarExhaustAttackRegion},
		{"through escape lane", math3d.Vec3{Y: -8, Z: 210}, DeathStarExhaustAttackRegion},
		{"past closed end", math3d.Vec3{Y: -8, Z: 290}, DeathStarSurfaceRegion},
	}
	for _, test := range tests {
		if got := DeathStarRegion(test.at); got != test.want {
			t.Errorf("%s region=%d, want %d", test.name, got, test.want)
		}
	}
}

func TestDeathStarMissionGuidePointsMatchAuthoredTrench(t *testing.T) {
	guide := DeathStarTrenchGuidePoint(math3d.Vec3{X: 50, Y: 10, Z: -60})
	if guide.X != 0 || guide.Y >= 0 || guide.Z != -60 || DeathStarRegion(guide) != DeathStarTrenchRegion {
		t.Fatalf("trench guide=%+v region=%d", guide, DeathStarRegion(guide))
	}
	port := DeathStarExhaustPortPoint()
	tile := DeathStarTrench().Tile(TileCoordinate{X: trenchTileX, Z: trenchPortTileZ})
	found := false
	for _, feature := range tile.Features {
		if feature.Kind == "exhaust-port" {
			found = feature.Pose.Position.Sub(port).Length() < 1e-9
		}
	}
	if !found || DeathStarRegion(port) != DeathStarExhaustAttackRegion {
		t.Fatalf("mission port=%+v is not the authored exhaust port", port)
	}
}

func TestRegistryProvidesRoomViewContextAndDefensivePortals(t *testing.T) {
	registry := NewRegistry()
	frame := scene.FrameID("test/falcon-cockpit")
	room := Room{
		Name:       "Falcon cockpit",
		Frame:      frame,
		Background: view.Background{Kind: view.BackgroundNone},
		Portals: []Portal{{
			Name: "windscreen", Destination: scene.ExteriorFrame,
			Boundary: []math3d.Vec3{{X: -1, Y: -1}, {X: 1, Y: -1}, {X: 1, Y: 1}, {X: -1, Y: 1}},
		}},
	}
	if err := registry.RegisterRoom(room); err != nil {
		t.Fatalf("register room: %v", err)
	}
	matrix := math3d.Translation(1, 2, 3)
	context, ok := registry.ViewContext(frame, matrix)
	if !ok || context.FrameID != frame || context.ViewMatrix != matrix || context.Background.Kind != view.BackgroundNone {
		t.Fatalf("room context=%+v, found=%v", context, ok)
	}
	if _, ok := registry.ViewContext(scene.ExteriorFrame, matrix); ok {
		t.Fatal("unregistered exterior frame unexpectedly resolved as a room")
	}

	copyRoom, ok := registry.Room(frame)
	if !ok {
		t.Fatal("registered room not found")
	}
	copyRoom.Portals[0].Boundary[0].X = 99
	again, _ := registry.Room(frame)
	if again.Portals[0].Boundary[0].X == 99 {
		t.Fatal("caller mutation changed registered portal boundary")
	}
}

func TestRoomRejectsInvalidAndDuplicatePortals(t *testing.T) {
	registry := NewRegistry()
	portal := Portal{Name: "door", Destination: scene.ExteriorFrame, Boundary: []math3d.Vec3{{}, {X: 1}, {Y: 1}}}
	room := Room{Name: "room", Frame: "test/room", Background: view.Background{Kind: view.BackgroundNone}, Portals: []Portal{portal, portal}}
	if err := registry.RegisterRoom(room); err == nil {
		t.Fatal("room with duplicate portal names passed validation")
	}
	room.Portals = []Portal{{Name: "door", Destination: scene.ExteriorFrame}}
	if err := registry.RegisterRoom(room); err == nil {
		t.Fatal("room with degenerate portal passed validation")
	}
	room.Portals = []Portal{{Name: "door", Destination: scene.ExteriorFrame, Boundary: []math3d.Vec3{
		{}, {X: 2}, {X: 1, Y: 0.5}, {X: 2, Y: 2}, {Y: 2},
	}}}
	if err := registry.RegisterRoom(room); err == nil {
		t.Fatal("room with concave portal passed validation")
	}
}
