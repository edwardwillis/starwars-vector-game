package game

import (
	"image/color"
	"testing"

	"github.com/hajimehoshi/ebiten/v2"
)

func TestStaticArtworkCacheLifecycle(t *testing.T) {
	var cache staticArtworkCache[string]
	builds := 0
	dynamicValue := 1
	build := func(image *ebiten.Image) {
		builds++
		image.Fill(color.RGBA{R: uint8(30 * builds), G: 20, B: 10, A: 255})
	}
	screen := ebiten.NewImage(64, 48)

	cache.Draw(screen, "fixed-artwork", build)
	first := cache.image
	if first == nil || !cache.valid || builds != 1 {
		t.Fatalf("initial cache state image=%v valid=%t builds=%d", first, cache.valid, builds)
	}

	cache.Draw(screen, "fixed-artwork", build)
	if cache.image != first || builds != 1 {
		t.Fatalf("unchanged cache rebuilt image=%t builds=%d", cache.image != first, builds)
	}

	// This value stands in for a live instrument update. It is intentionally
	// absent from the static-artwork key and must not trigger a rebuild.
	dynamicValue++
	cache.Draw(screen, "fixed-artwork", build)
	if dynamicValue != 2 || cache.image != first || builds != 1 {
		t.Fatalf("dynamic state rebuilt static image=%t builds=%d", cache.image != first, builds)
	}

	cache.Invalidate()
	cache.Draw(screen, "fixed-artwork", build)
	second := cache.image
	if second == first || builds != 2 {
		t.Fatalf("explicit invalidation did not replace image=%t builds=%d", second != first, builds)
	}
	assertDisposedImage(t, first)

	resizedScreen := ebiten.NewImage(65, 48)
	cache.Draw(resizedScreen, "fixed-artwork", build)
	third := cache.image
	if third == second || cache.width != 65 || cache.height != 48 || builds != 3 {
		t.Fatalf("size change did not rebuild size=%dx%d replacement=%t builds=%d", cache.width, cache.height, third != second, builds)
	}
	assertDisposedImage(t, second)

	cache.Dispose()
	if cache.image != nil || cache.valid {
		t.Fatalf("dispose left cache image=%v valid=%t", cache.image, cache.valid)
	}
	assertDisposedImage(t, third)
	cache.Dispose() // Disposal is intentionally idempotent.

	cache.Draw(screen, "fixed-artwork", build)
	if cache.image == nil || !cache.valid || builds != 4 {
		t.Fatalf("disposed cache did not rebuild image=%v valid=%t builds=%d", cache.image, cache.valid, builds)
	}
}

func TestStaticArtworkCachesRemainIndependent(t *testing.T) {
	var port, starboard staticArtworkCache[string]
	screen := ebiten.NewImage(32, 24)
	portBuilds, starboardBuilds := 0, 0
	port.Draw(screen, "same-key", func(image *ebiten.Image) {
		portBuilds++
		image.Fill(color.RGBA{R: 90, A: 255})
	})
	starboard.Draw(screen, "same-key", func(image *ebiten.Image) {
		starboardBuilds++
		image.Fill(color.RGBA{B: 90, A: 255})
	})
	if port.image == starboard.image || portBuilds != 1 || starboardBuilds != 1 {
		t.Fatalf("independent caches shared image=%t builds=%d/%d", port.image == starboard.image, portBuilds, starboardBuilds)
	}

	port.Invalidate()
	port.Draw(screen, "same-key", func(image *ebiten.Image) {
		portBuilds++
		image.Fill(color.RGBA{R: 120, A: 255})
	})
	if portBuilds != 2 || starboardBuilds != 1 || !starboard.valid {
		t.Fatalf("port invalidation affected starboard builds=%d/%d valid=%t", portBuilds, starboardBuilds, starboard.valid)
	}
}

func TestStaticArtworkCacheBuildFailureRemainsInvalid(t *testing.T) {
	var cache staticArtworkCache[string]
	screen := ebiten.NewImage(32, 24)
	func() {
		defer func() {
			if recover() == nil {
				t.Fatal("expected artwork build to panic")
			}
		}()
		cache.Draw(screen, "fixed-artwork", func(*ebiten.Image) { panic("test build failure") })
	}()
	partial := cache.image
	if partial == nil || cache.valid {
		t.Fatalf("failed build published image=%v valid=%t", partial, cache.valid)
	}

	builds := 0
	cache.Draw(screen, "fixed-artwork", func(image *ebiten.Image) {
		builds++
		image.Fill(color.RGBA{G: 90, A: 255})
	})
	if cache.image == partial || !cache.valid || builds != 1 {
		t.Fatalf("failed cache did not rebuild image=%t valid=%t builds=%d", cache.image != partial, cache.valid, builds)
	}
	assertDisposedImage(t, partial)
}

func assertDisposedImage(t *testing.T, image *ebiten.Image) {
	t.Helper()
	defer func() {
		if recover() == nil {
			t.Fatal("expected disposed image to reject Bounds")
		}
	}()
	_ = image.Bounds()
}
