package render

import "testing"

func TestTextureRegistryValidatesAndCopiesOpaquePixels(t *testing.T) {
	registry := NewTextureRegistry()
	pixels := []byte{10, 20, 30, 255}
	if err := registry.Register("test/texture", Texture{Width: 1, Height: 1, Pixels: pixels}); err != nil {
		t.Fatal(err)
	}
	pixels[0] = 99
	texture, ok := registry.Lookup("test/texture")
	if !ok || texture.Pixels[0] != 10 {
		t.Fatalf("registered texture was mutated by caller: %+v", texture)
	}
	if err := registry.Register("test/texture", Texture{Width: 1, Height: 1, Pixels: []byte{0, 0, 0, 255}}); err == nil {
		t.Fatal("accepted duplicate texture ID")
	}
	if err := registry.Register("test/transparent", Texture{Width: 1, Height: 1, Pixels: []byte{0, 0, 0, 128}}); err == nil {
		t.Fatal("accepted translucent texel in opaque registry")
	}
	if err := registry.Register("test/invalid-sampling", Texture{Width: 1, Height: 1, Pixels: []byte{0, 0, 0, 255}, Sampling: TextureSamplingLinearMipmap + 1}); err == nil {
		t.Fatal("accepted an unknown texture sampling mode")
	}
	if got := registry.Sampling("missing"); got != TextureSamplingNearest {
		t.Fatalf("missing texture sampling=%d, want nearest", got)
	}
}

func TestDefaultDeckTextureIsRegistered(t *testing.T) {
	texture, ok := DefaultTextureRegistry().Lookup(DeathStarDeckTextureID)
	if !ok || texture.Width != 1 || texture.Height != 1 {
		t.Fatalf("missing built-in deck texture: %+v", texture)
	}
	if texture.Sampling != TextureSamplingNearest {
		t.Fatalf("quiet deck sampling=%d, want nearest", texture.Sampling)
	}
	if len(texture.Pixels) != 4 || texture.Pixels[0] != 162 || texture.Pixels[1] != 162 || texture.Pixels[2] != 162 || texture.Pixels[3] != 255 {
		t.Fatalf("quiet deck texel=%v, want [162 162 162 255]", texture.Pixels)
	}
}

func TestDefaultHorizonDeckTextureIsRegistered(t *testing.T) {
	texture, ok := DefaultTextureRegistry().Lookup(DeathStarHorizonDeckTextureID)
	if !ok || texture.Width != 64 || texture.Height != 64 {
		t.Fatalf("missing built-in horizon deck texture: %+v", texture)
	}
	if texture.Pixels[0] <= texture.Pixels[(texture.Width+1)*4] {
		t.Fatal("horizon deck texture lost its brighter grid line")
	}
	if texture.Pixels[(16*texture.Width+16)*4] != texture.Pixels[0] {
		t.Fatal("horizon deck texture lost its four-division grid")
	}
}

func TestDefaultInstallationTextureIsRegistered(t *testing.T) {
	texture, ok := DefaultTextureRegistry().Lookup(DeathStarInstallationTextureID)
	if !ok || texture.Width != 64 || texture.Height != 64 {
		t.Fatalf("missing built-in installation texture: %+v", texture)
	}
	if texture.Pixels[0] >= texture.Pixels[(texture.Width+1)*4] {
		t.Fatal("installation texture lost its dark panel seam")
	}
	if texture.Pixels[(12*texture.Width+13)*4] != texture.Pixels[(texture.Width+13)*4] {
		t.Fatal("installation texture retained a horizontal channel band")
	}
	if texture.Pixels[(12*texture.Width)*4] >= texture.Pixels[(12*texture.Width+13)*4] {
		t.Fatal("installation texture lost its vertical panel seam")
	}
}

func TestDefaultTrenchTextureIsRegistered(t *testing.T) {
	texture, ok := DefaultTextureRegistry().Lookup(DeathStarTrenchTextureID)
	if !ok || texture.Width != 64 || texture.Height != 64 {
		t.Fatalf("missing built-in trench texture: %+v", texture)
	}
	if texture.Pixels[0] <= texture.Pixels[(texture.Width+1)*4] {
		t.Fatal("trench texture lost its bright structural seam")
	}
	if texture.Pixels[0] >= 208 {
		t.Fatalf("trench seam=%d is too bright for recessed armour", texture.Pixels[0])
	}
	if texture.Pixels[(13*texture.Width+13)*4] >= texture.Pixels[(texture.Width+1)*4] {
		t.Fatal("trench texture lost its inset channel")
	}
}
