package profile

import "testing"

func TestBuiltinProfilesAreValidAndResolved(t *testing.T) {
	profiles := []GameProfile{Cadet(), Pilot(), Ace(), Nightmare()}
	for _, profile := range profiles {
		if err := profile.Validate(); err != nil {
			t.Fatalf("profile %q is invalid: %v", profile.Name, err)
		}
		if profile.Name != profile.Difficulty.Name {
			t.Fatalf("profile %q has difficulty %q", profile.Name, profile.Difficulty.Name)
		}
		if len(profile.Swarm.InitialPositions) != profile.Swarm.Count {
			t.Fatalf("profile %q has %d positions for %d fighters", profile.Name, len(profile.Swarm.InitialPositions), profile.Swarm.Count)
		}
	}
}

func TestPilotShowsControlsForTenSeconds(t *testing.T) {
	if duration := Pilot().Display.ControlsDisplayDuration; duration != 10 {
		t.Fatalf("controls display duration=%v, want 10", duration)
	}
}

func TestBuiltinResolvesStableNames(t *testing.T) {
	for _, name := range []string{CadetName, PilotName, AceName, NightmareName} {
		profile, err := Builtin(name)
		if err != nil {
			t.Fatalf("Builtin(%q) returned an error: %v", name, err)
		}
		if profile.Name != name {
			t.Fatalf("Builtin(%q) returned %q", name, profile.Name)
		}
	}
	if _, err := Builtin("unknown"); err == nil {
		t.Fatal("Builtin accepted an unknown profile")
	}
	if resolved, err := Builtin("cadet"); err != nil || resolved.Name != CadetName {
		t.Fatalf("short profile alias did not resolve: profile=%q err=%v", resolved.Name, err)
	}
}

func TestDifficultyProfilesIncreasePressure(t *testing.T) {
	cadet, pilot, ace, nightmare := Cadet(), Pilot(), Ace(), Nightmare()
	if !(cadet.Swarm.Count < pilot.Swarm.Count && pilot.Swarm.Count < nightmare.Swarm.Count) {
		t.Fatal("swarm sizes do not increase across difficulty profiles")
	}
	if !(cadet.Swarm.AimError > pilot.Swarm.AimError && pilot.Swarm.AimError > ace.Swarm.AimError && ace.Swarm.AimError > nightmare.Swarm.AimError) {
		t.Fatal("aim error does not tighten across difficulty profiles")
	}
	if !(cadet.Swarm.Pursuit.MaxSpeed < pilot.Swarm.Pursuit.MaxSpeed && pilot.Swarm.Pursuit.MaxSpeed < ace.Swarm.Pursuit.MaxSpeed && ace.Swarm.Pursuit.MaxSpeed < nightmare.Swarm.Pursuit.MaxSpeed) {
		t.Fatal("maximum swarm speed does not increase across difficulty profiles")
	}
}

func TestCloneOwnsSwarmPositions(t *testing.T) {
	original := Pilot()
	clone := original.Clone()
	clone.Swarm.InitialPositions[0].X = 999
	if original.Swarm.InitialPositions[0].X == 999 {
		t.Fatal("Clone retained caller-owned swarm position storage")
	}
	clone.World.Objects[0].Definition = "changed"
	if original.World.Objects[0].Definition == "changed" {
		t.Fatal("Clone retained caller-owned world placement storage")
	}
}

func TestValidateRejectsInvalidConfiguration(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*GameProfile)
	}{
		{name: "missing name", mutate: func(profile *GameProfile) { profile.Name = "" }},
		{name: "zero tick", mutate: func(profile *GameProfile) { profile.Simulation.TickSeconds = 0 }},
		{name: "invalid laser", mutate: func(profile *GameProfile) { profile.Combat.Laser.Speed = 0 }},
		{name: "missing rendering profile", mutate: func(profile *GameProfile) { profile.Display.RenderingProfile = "" }},
		{name: "swarm position mismatch", mutate: func(profile *GameProfile) { profile.Swarm.InitialPositions = nil }},
		{name: "invalid shield", mutate: func(profile *GameProfile) { profile.Player.Shield.Maximum = 0 }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			profile := Pilot()
			test.mutate(&profile)
			if err := profile.Validate(); err == nil {
				t.Fatal("Validate accepted invalid configuration")
			}
		})
	}
}
