package chunk

// Chunk splits text into overlapping rune-based segments.
// Stride is ChunkSizeRunes - ChunkOverlapRunes so consecutive chunks
// share ChunkOverlapRunes runes from the source.
func Chunk(text string) []string {
	if text == "" {
		return nil
	}
	runes := []rune(text)
	if len(runes) <= ChunkSizeRunes {
		return []string{text}
	}
	stride := ChunkSizeRunes - ChunkOverlapRunes
	if stride < 1 {
		stride = 1
	}
	var out []string
	for start := 0; start < len(runes); {
		end := start + ChunkSizeRunes
		if end > len(runes) {
			end = len(runes)
		}
		out = append(out, string(runes[start:end]))
		if end == len(runes) {
			break
		}
		start += stride
	}
	return out
}
