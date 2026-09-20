package render

import "fmt"

// Texture is immutable CPU-side RGBA data. Model and scene packages carry only
// IDs; Ebitengine images are created lazily on the render thread.
type Texture struct {
	Width, Height int
	Pixels        []byte
}

type TextureRegistry struct{ textures map[string]Texture }

func NewTextureRegistry() *TextureRegistry {
	return &TextureRegistry{textures: make(map[string]Texture)}
}

func (registry *TextureRegistry) Register(id string, texture Texture) error {
	if registry == nil || id == "" || texture.Width <= 0 || texture.Height <= 0 ||
		texture.Width > 4096 || texture.Height > 4096 ||
		len(texture.Pixels) != texture.Width*texture.Height*4 {
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

const DeathStarDeckTextureID = "builtin/death-star-deck"

func DefaultTextureRegistry() *TextureRegistry {
	registry := NewTextureRegistry()
	const size = 16
	pixels := make([]byte, size*size*4)
	for y := range size {
		for x := range size {
			// Low-contrast modular plating leaves the green vector grid as the
			// primary visual cue. The authored tile UVs repeat this small motif.
			shade := byte(238)
			if x == 0 || y == 0 {
				shade = 250
			} else if (x == 4 || x == 11) && (y == 4 || y == 11) {
				shade = 255
			}
			offset := (y*size + x) * 4
			pixels[offset], pixels[offset+1], pixels[offset+2], pixels[offset+3] = shade, shade, shade, 255
		}
	}
	if err := registry.Register(DeathStarDeckTextureID, Texture{Width: size, Height: size, Pixels: pixels}); err != nil {
		panic(err)
	}
	return registry
}
