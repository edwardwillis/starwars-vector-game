package game

import "github.com/hajimehoshi/ebiten/v2"

// staticArtworkCache retains a complete, immutable screen-space artwork image.
// Its caller owns the comparable key and the build callback, keeping craft
// geometry and invalidation policy outside the reusable image lifecycle.
//
// The zero value is ready to use. It is intended for the Ebitengine draw
// thread: callers rebuild or release it only while no other goroutine is
// drawing with the same cache.
type staticArtworkCache[K comparable] struct {
	image         *ebiten.Image
	width, height int
	key           K
	valid         bool
}

// Draw rebuilds the cached image only when the target size or caller-owned key
// changes, then composites it at the screen origin. build receives a cleared
// image with the target's dimensions.
func (cache *staticArtworkCache[K]) Draw(screen *ebiten.Image, key K, build func(*ebiten.Image)) {
	width, height := screen.Bounds().Dx(), screen.Bounds().Dy()
	if width <= 0 || height <= 0 {
		cache.release()
		return
	}
	if !cache.valid || cache.width != width || cache.height != height || cache.key != key {
		cache.release()
		cache.image = ebiten.NewImage(width, height)
		cache.image.Clear()
		cache.width, cache.height = width, height
		cache.key = key
		build(cache.image)
		// A build panic leaves this false, ensuring that a later Draw never
		// treats partially rendered artwork as a valid cache entry.
		cache.valid = true
	}
	screen.DrawImage(cache.image, nil)
}

// Invalidate marks the artwork stale without disposing it mid-frame. The next
// Draw replaces and disposes it before the new artwork is built.
func (cache *staticArtworkCache[K]) Invalidate() {
	cache.valid = false
}

// Dispose explicitly releases the cached GPU resource. It is safe to call on
// an empty cache and leaves the zero-value cache ready for another Draw.
func (cache *staticArtworkCache[K]) Dispose() {
	cache.release()
}

func (cache *staticArtworkCache[K]) release() {
	if cache.image != nil {
		cache.image.Dispose()
		cache.image = nil
	}
	cache.width, cache.height = 0, 0
	cache.valid = false
}
