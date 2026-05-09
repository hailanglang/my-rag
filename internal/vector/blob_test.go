package vector

import (
	"reflect"
	"testing"
)

func TestBlob_roundTrip(t *testing.T) {
	in := []float32{0.25, -1.5, 3.25}
	b := float32SliceToBlob(in)
	out, err := blobToFloat32Slice(b)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(in, out) {
		t.Fatalf("got %#v", out)
	}
}

func TestBlob_badLength(t *testing.T) {
	_, err := blobToFloat32Slice([]byte{1, 2, 3})
	if err == nil {
		t.Fatal("expected error")
	}
}
