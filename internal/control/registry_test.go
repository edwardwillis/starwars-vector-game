package control

import "testing"

type testStrategy struct{}

func (testStrategy) Decide(Context) Decision {
	return Decision{Flight: Intent{Throttle: 1}, Fire: true}
}

func TestDefaultRegistryCreatesBuiltIns(t *testing.T) {
	registry := DefaultRegistry()
	for _, name := range []string{StaticName, ManualName, PursuitName} {
		var configuration any
		if name == PursuitName {
			configuration = DefaultPursuitConfig()
		}
		strategy, err := registry.Create(name, 7, configuration)
		if err != nil {
			t.Fatalf("Create(%q) returned an error: %v", name, err)
		}
		if strategy == nil {
			t.Fatalf("Create(%q) returned nil", name)
		}
	}
}

func TestRegistrySupportsContributorStrategy(t *testing.T) {
	registry := NewRegistry()
	if err := registry.Register("example/attack", func(uint64, any) (Strategy, error) {
		return testStrategy{}, nil
	}); err != nil {
		t.Fatalf("Register returned an error: %v", err)
	}
	if err := registry.Register("example/attack", func(uint64, any) (Strategy, error) {
		return testStrategy{}, nil
	}); err == nil {
		t.Fatal("Register accepted a duplicate name")
	}
	strategy, err := registry.Create("example/attack", 1, nil)
	if err != nil {
		t.Fatalf("Create returned an error: %v", err)
	}
	decision := strategy.Decide(Context{})
	if !decision.Fire || decision.Flight.Throttle != 1 {
		t.Fatalf("registered strategy returned unexpected decision: %+v", decision)
	}
}

func TestRegistryReportsUnknownAndInvalidConfigurations(t *testing.T) {
	registry := DefaultRegistry()
	if _, err := registry.Create("missing", 1, nil); err == nil {
		t.Fatal("Create accepted an unknown strategy")
	}
	if _, err := registry.Create(PursuitName, 1, nil); err == nil {
		t.Fatal("Create accepted invalid pursuit configuration")
	}
}
