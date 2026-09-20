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
}

func TestDefaultDeckTextureIsRegistered(t *testing.T) {
	texture, ok := DefaultTextureRegistry().Lookup(DeathStarDeckTextureID)
	if !ok || texture.Width != 16 || texture.Height != 16 {
		t.Fatalf("missing built-in deck texture: %+v", texture)
	}
}
