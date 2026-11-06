package game

import (
	gameaudio "github.com/edwardwillis/starwars-vector-game/internal/audio"
	"github.com/edwardwillis/starwars-vector-game/internal/overlay"
	"github.com/hajimehoshi/ebiten/v2"
)

// ShowCharacterOverlay requests a configured character portrait. The
// character's definition, rather than its gameplay caller, owns the effect,
// timings, audio, and other presentation settings.
func (g *Game) ShowCharacterOverlay(characterID overlay.ID) bool {
	if g.characterOverlay == nil || !g.characterOverlay.ShowConfigured(characterID) {
		return false
	}
	g.cancelCharacterOverlayAudio()
	g.triggerCharacterOverlayAudio()
	return true
}

// showCharacterOverlayOverride is intentionally limited to development
// previews and tests that compare transitions. Normal gameplay calls the
// character-owned API above.
func (g *Game) showCharacterOverlayOverride(characterID overlay.ID, presentation overlay.Presentation, edge overlay.Edge) bool {
	return g.showCharacterOverlayFromEdge(characterID, presentation, edge)
}

func (g *Game) showCharacterOverlayFromEdge(characterID overlay.ID, presentation overlay.Presentation, edge overlay.Edge) bool {
	if g.characterOverlay == nil {
		return false
	}
	if !g.characterOverlay.ShowFromEdge(characterID, presentation, edge) {
		return false
	}
	g.cancelCharacterOverlayAudio()
	g.triggerCharacterOverlayAudio()
	return true
}

func (g *Game) updateCharacterOverlay(seconds float64) {
	if g.characterOverlay == nil || g.paused || (g.flow != flowPlaying && g.flow != flowOutcome) {
		return
	}
	g.characterOverlay.Update(seconds)
	g.triggerCharacterOverlayAudio()
}

func (g *Game) cancelCharacterOverlay() {
	g.cancelCharacterOverlayAudio()
	if g.characterOverlay != nil {
		g.characterOverlay.Cancel()
	}
}

func (g *Game) drawCharacterOverlay(screen *ebiten.Image) {
	if g.characterOverlay != nil {
		g.characterOverlay.Draw(screen)
	}
}

func (g *Game) triggerCharacterOverlayAudio() {
	if g.characterOverlay == nil {
		return
	}
	for {
		event, pending := g.characterOverlay.ConsumeAudioEvent()
		if !pending {
			return
		}
		if g.soundPlayer == nil {
			continue
		}
		cue, found := characterOverlayCue(event.Asset)
		if !found {
			continue
		}
		if configurable, supported := g.soundPlayer.(gameaudio.ConfigurablePlayer); supported {
			playback := configurable.TriggerWithVolume(cue, event.Volume)
			if event.Policy == overlay.StopOnCancel {
				g.characterOverlayPlayback = playback
				g.characterOverlayPolicy = event.Policy
			}
			continue
		}
		g.soundPlayer.Trigger(cue)
	}
}

func (g *Game) cancelCharacterOverlayAudio() {
	if g.characterOverlayPolicy == overlay.StopOnCancel && g.characterOverlayPlayback != nil {
		g.characterOverlayPlayback.Stop()
	}
	g.characterOverlayPlayback = nil
	g.characterOverlayPolicy = overlay.FinishNaturally
}

func characterOverlayCue(asset string) (gameaudio.Cue, bool) {
	switch asset {
	case "jabba-laugh":
		return gameaudio.CueJabbaLaugh, true
	case "r2d2-1":
		return gameaudio.CueR2D2, true
	default:
		return "", false
	}
}
