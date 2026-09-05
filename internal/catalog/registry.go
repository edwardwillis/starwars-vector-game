package catalog

import (
	"fmt"

	"github.com/edwardwillis/starwars-vector-game/internal/kinematics"
	"github.com/edwardwillis/starwars-vector-game/internal/scene"
)

// Definition describes every lifecycle representation of a catalog object.
type Definition struct {
	Name           string
	Create         func(scene.ObjectID, kinematics.Pose) scene.Object
	CreateFragment func(scene.ObjectID, int, kinematics.Pose) scene.Object
	PolygonCount   func(int) int
	CreatePolygon  func(scene.ObjectID, int, int, kinematics.Pose) scene.Object
}

type Registry struct{ definitions map[string]Definition }

func NewRegistry() *Registry { return &Registry{definitions: make(map[string]Definition)} }

func (r *Registry) Register(def Definition) error {
	if r == nil {
		return fmt.Errorf("catalog registry is nil")
	}
	if def.Name == "" || def.Create == nil {
		return fmt.Errorf("catalog definition requires name and create function")
	}
	if r.definitions == nil {
		r.definitions = make(map[string]Definition)
	}
	if _, exists := r.definitions[def.Name]; exists {
		return fmt.Errorf("catalog definition %q already registered", def.Name)
	}
	r.definitions[def.Name] = def
	return nil
}

func (r *Registry) Lookup(name string) (Definition, error) {
	if r == nil {
		return Definition{}, fmt.Errorf("catalog registry is nil")
	}
	def, ok := r.definitions[name]
	if !ok {
		return Definition{}, fmt.Errorf("catalog definition %q is not registered", name)
	}
	return def, nil
}

func (r *Registry) Create(name string, id scene.ObjectID, pose kinematics.Pose) (scene.Object, error) {
	def, err := r.Lookup(name)
	if err != nil {
		return scene.Object{}, err
	}
	object := def.Create(id, pose)
	object.Definition = name
	if err := object.Validate(); err != nil {
		return scene.Object{}, fmt.Errorf("catalog definition %q: %w", name, err)
	}
	return object, nil
}

func (r *Registry) CreateFragment(name string, id scene.ObjectID, component int, pose kinematics.Pose) (scene.Object, error) {
	def, err := r.Lookup(name)
	if err != nil {
		return scene.Object{}, err
	}
	if def.CreateFragment == nil {
		return scene.Object{}, fmt.Errorf("catalog definition %q has no fragment factory", name)
	}
	object := def.CreateFragment(id, component, pose)
	object.Definition = name
	return object, object.Validate()
}

func (r *Registry) PolygonCount(name string, component int) (int, error) {
	def, err := r.Lookup(name)
	if err != nil {
		return 0, err
	}
	if def.PolygonCount == nil {
		return 0, fmt.Errorf("catalog definition %q has no polygon shards", name)
	}
	return def.PolygonCount(component), nil
}

func (r *Registry) CreatePolygon(name string, id scene.ObjectID, component, polygon int, pose kinematics.Pose) (scene.Object, error) {
	def, err := r.Lookup(name)
	if err != nil {
		return scene.Object{}, err
	}
	if def.CreatePolygon == nil {
		return scene.Object{}, fmt.Errorf("catalog definition %q has no polygon factory", name)
	}
	object := def.CreatePolygon(id, component, polygon, pose)
	object.Definition = name
	return object, object.Validate()
}

func DefaultRegistry() *Registry {
	r := NewRegistry()
	_ = r.Register(Definition{Name: TIEFighterName, Create: TIEFighter, CreateFragment: TIEFighterFragment, PolygonCount: TIEFighterPolygonCount, CreatePolygon: TIEFighterPolygon})
	_ = r.Register(Definition{Name: XWingName, Create: XWing, CreateFragment: XWingFragment, PolygonCount: XWingPolygonCount, CreatePolygon: XWingPolygon})
	_ = r.Register(Definition{Name: LaserBoltName, Create: LaserBolt})
	_ = r.Register(Definition{Name: DeathStarName, Create: DeathStar})
	return r
}
