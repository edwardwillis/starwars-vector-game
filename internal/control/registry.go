package control

import "fmt"

const (
	StaticName  = "builtin/static"
	ManualName  = "builtin/manual"
	PursuitName = "builtin/pursuit"
)

// Factory constructs an independent controller instance. Configuration is
// intentionally opaque at this boundary so contributors can define strategy-
// specific schemas while the registry still provides stable names.
type Factory func(seed uint64, configuration any) (Strategy, error)

// Registry maps stable controller names to factories. A registry is normally
// assembled once at startup and then treated as immutable by the simulation.
type Registry struct {
	factories map[string]Factory
}

func NewRegistry() *Registry {
	return &Registry{factories: make(map[string]Factory)}
}

func (registry *Registry) Register(name string, factory Factory) error {
	if registry == nil {
		return fmt.Errorf("controller registry is nil")
	}
	if name == "" {
		return fmt.Errorf("controller name is required")
	}
	if factory == nil {
		return fmt.Errorf("controller %q has no factory", name)
	}
	if registry.factories == nil {
		registry.factories = make(map[string]Factory)
	}
	if _, exists := registry.factories[name]; exists {
		return fmt.Errorf("controller %q is already registered", name)
	}
	registry.factories[name] = factory
	return nil
}

func (registry *Registry) Create(name string, seed uint64, configuration any) (Strategy, error) {
	if registry == nil {
		return nil, fmt.Errorf("controller registry is nil")
	}
	factory, ok := registry.factories[name]
	if !ok {
		return nil, fmt.Errorf("controller %q is not registered", name)
	}
	strategy, err := factory(seed, configuration)
	if err != nil {
		return nil, fmt.Errorf("create controller %q: %w", name, err)
	}
	if strategy == nil {
		return nil, fmt.Errorf("controller %q factory returned nil", name)
	}
	return strategy, nil
}

// DefaultRegistry contains the built-in strategies used by local gameplay.
func DefaultRegistry() *Registry {
	registry := NewRegistry()
	_ = registry.Register(StaticName, func(_ uint64, _ any) (Strategy, error) {
		return Static{}, nil
	})
	_ = registry.Register(ManualName, func(_ uint64, configuration any) (Strategy, error) {
		if configuration == nil {
			return Manual{}, nil
		}
		intent, ok := configuration.(Intent)
		if !ok {
			return nil, fmt.Errorf("manual configuration must be control.Intent")
		}
		return Manual{Intent: intent}, nil
	})
	_ = registry.Register(PursuitName, func(seed uint64, configuration any) (Strategy, error) {
		config, ok := configuration.(PursuitConfig)
		if !ok {
			return nil, fmt.Errorf("pursuit configuration must be control.PursuitConfig")
		}
		return NewPursuit(seed, config), nil
	})
	return registry
}

// Static is a controller for objects that should not move or act.
type Static struct{}

func (Static) Decide(Context) Decision { return Decision{} }

// Manual adapts a prevalidated human intent to the controller contract. The
// local game currently reads input directly; networked clients will use this
// strategy through the same registry.
type Manual struct {
	Intent Intent
}

func (manual Manual) Decide(Context) Decision { return Decision{Flight: manual.Intent} }
