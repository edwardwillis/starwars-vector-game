package render

import "fmt"

// Texture is immutable CPU-side RGBA data. Model and scene packages carry only
// IDs; Ebitengine images are created lazily on the render thread.
type Texture struct {
	Width, Height int
	Pixels        []byte
	Sampling      TextureSampling
}

// TextureSampling controls how a GPU source image is sampled. Linear mipmaps
// are appropriate for dense bitmap materials seen at a grazing angle; the
// zero value retains the crisp nearest sampling used by vector-era textures.
type TextureSampling uint8

const (
	TextureSamplingNearest TextureSampling = iota
	TextureSamplingLinearMipmap
)

type TextureRegistry struct{ textures map[string]Texture }

func NewTextureRegistry() *TextureRegistry {
	return &TextureRegistry{textures: make(map[string]Texture)}
}

func (registry *TextureRegistry) Register(id string, texture Texture) error {
	if registry == nil || id == "" || texture.Width <= 0 || texture.Height <= 0 ||
		texture.Width > 4096 || texture.Height > 4096 ||
		len(texture.Pixels) != texture.Width*texture.Height*4 || texture.Sampling > TextureSamplingLinearMipmap {
		return fmt.Errorf("invalid opaque texture %q", id)
	}
	if registry.textures == nil {
		registry.textures = make(map[string]Texture)
	}
	if _, exists := registry.textures[id]; exists {
		return fmt.Errorf("duplicate texture %q", id)
	}
	for offset := 3; offset < len(texture.Pixels); offset += 4 {
		if texture.Pixels[offset] != 255 {
			return fmt.Errorf("texture %q contains non-opaque texels", id)
		}
	}
	texture.Pixels = append([]byte(nil), texture.Pixels...)
	registry.textures[id] = texture
	return nil
}

func (registry *TextureRegistry) Lookup(id string) (Texture, bool) {
	if registry == nil {
		return Texture{}, false
	}
	texture, ok := registry.textures[id]
	if ok {
		texture.Pixels = append([]byte(nil), texture.Pixels...)
	}
	return texture, ok
}

// Sampling returns a registered texture's immutable sampling mode without
// copying its pixel payload. Unknown IDs retain the nearest-sampling default
// used by the flat-white fallback source.
func (registry *TextureRegistry) Sampling(id string) TextureSampling {
	if registry == nil {
		return TextureSamplingNearest
	}
	return registry.textures[id].Sampling
}

const (
	DeathStarDeckTextureID         = "builtin/death-star-deck"
	DeathStarHorizonDeckTextureID  = "builtin/death-star-horizon-deck"
	DeathStarInstallationTextureID = "builtin/death-star-installation"
	DeathStarTrenchTextureID       = "builtin/death-star-trench"
)

func DefaultTextureRegistry() *TextureRegistry {
	registry := NewTextureRegistry()
	if err := registry.Register(DeathStarDeckTextureID, deathStarDeckTexture()); err != nil {
		panic(err)
	}
	if err := registry.Register(DeathStarHorizonDeckTextureID, deathStarHorizonDeckTexture()); err != nil {
		panic(err)
	}
	if err := registry.Register(DeathStarInstallationTextureID, deathStarInstallationTexture()); err != nil {
		panic(err)
	}
	if err := registry.Register(DeathStarTrenchTextureID, deathStarTrenchTexture()); err != nil {
		panic(err)
	}
	return registry
}

// deathStarDeckTexture is deliberately quiet. The floor supplies scale through
// its structural vector grid; actual equipment is authored as opaque 3D
// installations rather than baked into a grazing-angle ground texture.
func deathStarDeckTexture() Texture {
	const shade = 162 // 68 * 162 / 255 ≈ the historic 42 deck fill.
	return Texture{Width: 1, Height: 1, Pixels: []byte{shade, shade, shade, 255}}
}

// deathStarHorizonDeckTexture preserves the distant deck grid without asking
// the CPU vector pipeline to transform and clip every line on every streamed
// horizon tile. Its dark texels multiply the deck-line material colour down to
// the same deck grey used by the old opaque fill, while the bright grid texels
// retain the familiar structural lines. The texture repeats once per 80-unit
// horizon tile through the tile face's existing UVs.
func deathStarHorizonDeckTexture() Texture {
	const size, divisions = 64, 4
	pixels := make([]byte, size*size*4)
	for y := range size {
		for x := range size {
			shade := byte(128) // 84 * 128 / 255 ≈ the existing 42 deck fill.
			if x%(size/divisions) == 0 || y%(size/divisions) == 0 {
				shade = 192
			} else if x%(size/divisions) == 8 && y%(size/divisions) == 8 {
				shade = 144
			}
			offset := (y*size + x) * 4
			pixels[offset], pixels[offset+1], pixels[offset+2], pixels[offset+3] = shade, shade, shade, 255
		}
	}
	return Texture{Width: size, Height: size, Pixels: pixels}
}

// deathStarTrenchTexture is a dark structural lining: repeating reinforced
// panels with occasional inset channels. Its seams are only modestly lighter
// than the base, so the wall reads as recessed armour rather than a field of
// pale outlined rectangles.
func deathStarTrenchTexture() Texture {
	const size, module = 64, 32
	pixels := make([]byte, size*size*4)
	for y := range size {
		for x := range size {
			localX, localY := x%module, y%module
			shade := byte(154)
			if localX == 0 || localY == 0 {
				shade = 188
			} else if localX == 2 || localY == 2 {
				shade = 174
			} else if localX >= 12 && localX <= 19 {
				shade = 100
			}
			offset := (y*size + x) * 4
			pixels[offset], pixels[offset+1], pixels[offset+2], pixels[offset+3] = shade, shade, shade, 255
		}
	}
	return Texture{Width: size, Height: size, Pixels: pixels}
}

// deathStarInstallationTexture provides restrained vertical panel seams for
// installation bodies. Horizontal channel bands are deliberately absent: on
// vertical faces they read as a repeated interference pattern rather than
// deliberate architectural detail at surface-flight range.
func deathStarInstallationTexture() Texture {
	const size, module = 64, 32
	pixels := make([]byte, size*size*4)
	for y := range size {
		for x := range size {
			localX := x % module
			shade := byte(210)
			if localX == 0 {
				shade = 142 // recessed primary panel seam
			} else if localX == 3 {
				shade = 170 // restrained inset bevel
			}
			offset := (y*size + x) * 4
			pixels[offset], pixels[offset+1], pixels[offset+2], pixels[offset+3] = shade, shade, shade, 255
		}
	}
	return Texture{Width: size, Height: size, Pixels: pixels}
}
