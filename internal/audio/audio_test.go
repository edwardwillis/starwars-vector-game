package audio

import (
	"bytes"
	"io"
	"testing"

	"github.com/hajimehoshi/ebiten/v2/audio/mp3"
	"github.com/hajimehoshi/ebiten/v2/audio/wav"
)

func TestBundledEffectsDecode(t *testing.T) {
	mp3Effects := map[string][]byte{
		"X-Wing laser":             xWingLaserMP3,
		"TIE flyby one":            tieFlybyOneMP3,
		"TIE flyby two":            tieFlybyTwoMP3,
		"Imperial-craft explosion": imperialCraftExplosionMP3,
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
