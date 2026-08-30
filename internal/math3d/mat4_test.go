package math3d

import (
	"math"
	"testing"
)

func TestTransformPoint(t *testing.T) {
	point := Vec3{X: 1, Y: 2, Z: 3}

	assertVec3(t, Identity().TransformPoint(point), point)
	assertVec3(t, Translation(4, -2, 1).TransformPoint(point), Vec3{X: 5, Y: 0, Z: 4})
	assertVec3(t, Scaling(2, 3, 4).TransformPoint(point), Vec3{X: 2, Y: 6, Z: 12})
}

func TestTransformDirectionIgnoresTranslation(t *testing.T) {
	direction := Vec3{X: 1, Y: 2, Z: 3}
	assertVec3(t, Translation(10, 20, 30).TransformDirection(direction), direction)
}

func TestRotations(t *testing.T) {
	quarterTurn := math.Pi / 2
	assertVec3(t, RotationX(quarterTurn).TransformPoint(Vec3{Y: 1}), Vec3{Z: 1})
	assertVec3(t, RotationY(quarterTurn).TransformPoint(Vec3{Z: 1}), Vec3{X: 1})
	assertVec3(t, RotationZ(quarterTurn).TransformPoint(Vec3{X: 1}), Vec3{Y: 1})
}

func TestMatrixCompositionAppliesRightmostFirst(t *testing.T) {
	transform := Translation(10, 0, 0).Mul(Scaling(2, 2, 2))
	assertVec3(t, transform.TransformPoint(Vec3{X: 1}), Vec3{X: 12})
}

func TestPerspectiveProjection(t *testing.T) {
	projection := Perspective(math.Pi/2, 1, 1, 10)

	assertVec3(t, projection.TransformPoint(Vec3{X: 1, Y: 1, Z: -2}), Vec3{X: 0.5, Y: 0.5, Z: 1.0 / 9.0})
	assertClose(t, projection.TransformPoint(Vec3{Z: -1}).Z, -1)
	assertClose(t, projection.TransformPoint(Vec3{Z: -10}).Z, 1)
}

func TestPerspectiveRejectsInvalidParameters(t *testing.T) {
	tests := []struct {
		name      string
		fov       float64
		aspect    float64
		near, far float64
	}{
		{name: "zero field of view", aspect: 1, near: 1, far: 10},
		{name: "wide field of view", fov: math.Pi, aspect: 1, near: 1, far: 10},
		{name: "zero aspect", fov: 1, near: 1, far: 10},
		{name: "zero near plane", fov: 1, aspect: 1, far: 10},
		{name: "reversed planes", fov: 1, aspect: 1, near: 10, far: 1},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			defer func() {
				if recover() == nil {
					t.Fatal("Perspective did not panic")
				}
			}()
			Perspective(test.fov, test.aspect, test.near, test.far)
		})
	}
}
