package vector

import (
	"encoding/binary"
	"errors"
	"math"
)

var errBadBlobLen = errors.New("vector: embedding blob length must be a multiple of 4")

func float32SliceToBlob(v []float32) []byte {
	if len(v) == 0 {
		return nil
	}
	b := make([]byte, 4*len(v))
	for i, f := range v {
		binary.LittleEndian.PutUint32(b[i*4:], math.Float32bits(f))
	}
	return b
}

func blobToFloat32Slice(b []byte) ([]float32, error) {
	if len(b)%4 != 0 {
		return nil, errBadBlobLen
	}
	if len(b) == 0 {
		return nil, nil
	}
	out := make([]float32, len(b)/4)
	for i := range out {
		u := binary.LittleEndian.Uint32(b[i*4:])
		out[i] = math.Float32frombits(u)
	}
	return out, nil
}
