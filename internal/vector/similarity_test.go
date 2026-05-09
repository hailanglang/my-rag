package vector

import "testing"

func TestCosine_parallel(t *testing.T) {
	a := []float32{1, 0}
	b := []float32{1, 0}
	if c := Cosine(a, b); c < 0.9999 {
		t.Fatalf("got %v", c)
	}
}

func TestCosine_orthogonal(t *testing.T) {
	a := []float32{1, 0}
	b := []float32{0, 1}
	if c := Cosine(a, b); c != 0 {
		t.Fatalf("got %v", c)
	}
}

func TestCosine_zeroNorm(t *testing.T) {
	if c := Cosine([]float32{0, 0}, []float32{1, 0}); c != 0 {
		t.Fatalf("got %v", c)
	}
}
