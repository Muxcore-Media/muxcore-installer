package password

import "testing"

func TestGenerateLength(t *testing.T) {
	p := Generate(16)
	if len(p) != 16 {
		t.Fatalf("len = %d", len(p))
	}
}

func TestGenerateUnique(t *testing.T) {
	a, b := Generate(16), Generate(16)
	if a == b {
		t.Fatal("two generated passwords should differ")
	}
}
