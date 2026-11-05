// Package audio owns presentation-only game sound effects.
//
// Simulation publishes no audio state: callers trigger named cues after an
// authoritative gameplay action has succeeded. That keeps deterministic
// replays, mission rules, and future network simulation independent of the
// local device's audio timing and availability.
package audio

import (
	"bytes"
	_ "embed"
	"fmt"
	"io"
	"math/rand/v2"

	ebitenaudio "github.com/hajimehoshi/ebiten/v2/audio"
	"github.com/hajimehoshi/ebiten/v2/audio/mp3"
	"github.com/hajimehoshi/ebiten/v2/audio/wav"
)

// Cue identifies a presentation sound. New gameplay effects should add a cue
// and register its sample here instead of coupling playback to game logic.
type Cue string

const (
	// CueXWingLaser is one paired player laser volley, not each individual bolt.
	CueXWingLaser Cue = "x-wing-laser"
	// CueTIEFlyby is played for a close Imperial fighter pass. Its registered
	// variants are selected locally at random.
	CueTIEFlyby Cue = "tie-flyby"
	// CueShieldImpact is emitted after the player has accepted shield damage.
	CueShieldImpact Cue = "shield-impact"
	// CueImperialFighterExplosion is an Alliance-fired destruction of an
	// Imperial fighter.
	CueImperialFighterExplosion Cue = "imperial-fighter-explosion"
)

// Player is the presentation boundary consumed by the game package. A nil
// Player means audio is unavailable or intentionally disabled.
type Player interface {
	Trigger(Cue)
	Update()
}

const (
	// SampleRate is shared by every decoded effect in the application context.
	SampleRate          = 48000
	maximumActiveVoices = 8
)

//go:embed assets/sfx/X-Wing_fire.mp3
var xWingLaserMP3 []byte

//go:embed assets/sfx/TIE_fighter_flyby_1.mp3
var tieFlybyOneMP3 []byte

//go:embed assets/sfx/TIE_fighter_flyby_2.mp3
var tieFlybyTwoMP3 []byte

//go:embed assets/sfx/shield-impact.wav
var shieldImpactWAV []byte

//go:embed assets/sfx/imperial-craft-explosion.mp3
var imperialCraftExplosionMP3 []byte

type sample struct {
	pcm    []byte
	volume float64
}

// Soundboard mixes short, one-shot effects. It is deliberately separate from
// simulation; it may safely drop a cue when the local output is unavailable.
type Soundboard struct {
	context *ebitenaudio.Context
	samples map[Cue][]sample
	active  []*ebitenaudio.Player
}

// New creates the initial soundboard and decodes the bundled effects once.
func New(context *ebitenaudio.Context) (*Soundboard, error) {
	if context == nil {
		return nil, fmt.Errorf("audio context is nil")
	}
	laser, err := decodeMP3(context, xWingLaserMP3)
	if err != nil {
		return nil, fmt.Errorf("decode X-Wing laser effect: %w", err)
	}
	flybyOne, err := decodeMP3(context, tieFlybyOneMP3)
	if err != nil {
		return nil, fmt.Errorf("decode first TIE flyby effect: %w", err)
	}
	flybyTwo, err := decodeMP3(context, tieFlybyTwoMP3)
	if err != nil {
		return nil, fmt.Errorf("decode second TIE flyby effect: %w", err)
	}
	shieldImpact, err := decodeWAV(context, shieldImpactWAV)
	if err != nil {
		return nil, fmt.Errorf("decode shield-impact effect: %w", err)
	}
	imperialCraftExplosion, err := decodeMP3(context, imperialCraftExplosionMP3)
	if err != nil {
		return nil, fmt.Errorf("decode Imperial-craft explosion effect: %w", err)
	}
	return &Soundboard{
		context: context,
		samples: map[Cue][]sample{
			CueXWingLaser: {{pcm: laser, volume: 0.55}},
			CueTIEFlyby: {
				{pcm: flybyOne, volume: 0.48},
				{pcm: flybyTwo, volume: 0.48},
			},
			CueShieldImpact:             {{pcm: shieldImpact, volume: 0.52}},
			CueImperialFighterExplosion: {{pcm: imperialCraftExplosion, volume: 0.58}},
		},
	}, nil
}

func decodeMP3(context *ebitenaudio.Context, encoded []byte) ([]byte, error) {
	stream, err := mp3.Decode(context, bytes.NewReader(encoded))
	if err != nil {
		return nil, err
	}
	return io.ReadAll(stream)
}

func decodeWAV(context *ebitenaudio.Context, encoded []byte) ([]byte, error) {
	stream, err := wav.Decode(context, bytes.NewReader(encoded))
	if err != nil {
		return nil, err
	}
	return io.ReadAll(stream)
}

// Trigger starts one local instance of a registered cue. Unknown cues are
// intentionally ignored so an optional future asset cannot break gameplay.
func (board *Soundboard) Trigger(cue Cue) {
	if board == nil {
		return
	}
	board.Update()
	variants, found := board.samples[cue]
	if !found || len(variants) == 0 {
		return
	}
	asset := variants[rand.IntN(len(variants))]
	if len(board.active) >= maximumActiveVoices {
		_ = board.active[0].Close()
		board.active = board.active[1:]
	}
	player := board.context.NewPlayerFromBytes(asset.pcm)
	player.SetVolume(asset.volume)
	player.Play()
	board.active = append(board.active, player)
}

// Update releases completed players. It belongs in the application frame loop,
// not the authoritative fixed-tick simulation.
func (board *Soundboard) Update() {
	if board == nil || len(board.active) == 0 {
		return
	}
	active := board.active[:0]
	for _, player := range board.active {
		if player.IsPlaying() {
			active = append(active, player)
			continue
		}
		_ = player.Close()
	}
	board.active = active
}
