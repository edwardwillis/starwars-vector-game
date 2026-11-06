package audio

import (
	"bytes"
	"encoding/binary"
	"io"
	"testing"

	ebitenaudio "github.com/hajimehoshi/ebiten/v2/audio"
	"github.com/hajimehoshi/ebiten/v2/audio/mp3"
	"github.com/hajimehoshi/ebiten/v2/audio/wav"
)

func TestBundledEffectsDecode(t *testing.T) {
	mp3Effects := map[string][]byte{
		"X-Wing laser":             xWingLaserMP3,
		"TIE flyby one":            tieFlybyOneMP3,
		"TIE flyby two":            tieFlybyTwoMP3,
		"Imperial-craft explosion": imperialCraftExplosionMP3,
		"Jabba laugh":              jabbaLaughMP3,
		"R2-D2":                    r2d2MP3,
	}
	for name, encoded := range mp3Effects {
		t.Run(name, func(t *testing.T) {
			stream, err := mp3.DecodeWithSampleRate(SampleRate, bytes.NewReader(encoded))
			if err != nil {
				t.Fatalf("decode: %v", err)
			}
			pcm, err := io.ReadAll(stream)
			if err != nil || len(pcm) == 0 {
				t.Fatalf("decoded PCM bytes=%d err=%v", len(pcm), err)
			}
		})
	}

	stream, err := wav.DecodeWithSampleRate(SampleRate, bytes.NewReader(shieldImpactWAV))
	if err != nil {
		t.Fatalf("decode shield impact: %v", err)
	}
	pcm, err := io.ReadAll(stream)
	if err != nil || len(pcm) == 0 {
		t.Fatalf("decoded shield-impact PCM bytes=%d err=%v", len(pcm), err)
	}
}

func TestJabbaLaughHasAudibleSignalNearStart(t *testing.T) {
	stream, err := mp3.DecodeWithSampleRate(SampleRate, bytes.NewReader(jabbaLaughMP3))
	if err != nil {
		t.Fatal(err)
	}
	pcm, err := io.ReadAll(stream)
	if err != nil {
		t.Fatal(err)
	}
	first := -1
	for index := 0; index+1 < len(pcm); index += 2 {
		sample := int16(binary.LittleEndian.Uint16(pcm[index:]))
		if sample > 500 || sample < -500 {
			first = index / 4 // Ebitengine PCM is stereo, 16-bit frames.
			break
		}
	}
	if first < 0 {
		t.Fatal("Jabba laugh has no audible PCM signal")
	}
	delay := float64(first) / SampleRate
	t.Logf("Jabba laugh first audible signal at %.3fs", delay)
	if delay > .5 {
		t.Fatalf("Jabba laugh has %.3fs of leading silence", delay)
	}
}

func TestSoundboardRegistersJabbaLaugh(t *testing.T) {
	board, err := New(ebitenaudio.NewContext(SampleRate))
	if err != nil {
		t.Fatalf("new soundboard: %v", err)
	}
	variants := board.samples[CueJabbaLaugh]
	if len(variants) != 1 || len(variants[0].pcm) == 0 {
		t.Fatalf("Jabba laugh samples=%d, PCM bytes=%d", len(variants), len(variants[0].pcm))
	}
}
