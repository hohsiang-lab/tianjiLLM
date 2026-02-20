package auto

import "math"

// CosineSimilarity computes cosine similarity between two float32 vectors.
func CosineSimilarity(a, b []float32) float64 {
	if len(a) != len(b) || len(a) == 0 {
		return 0
	}

	var dot, normA, normB float64
	for i := range a {
		dot += float64(a[i]) * float64(b[i])
		normA += float64(a[i]) * float64(a[i])
		normB += float64(b[i]) * float64(b[i])
	}

	denom := math.Sqrt(normA) * math.Sqrt(normB)
	if denom == 0 {
		return 0
	}
	return dot / denom
}

// BestMatch finds the best matching candidate vector for a query.
// Returns the index and similarity score. Returns -1 if no candidate exceeds threshold.
func BestMatch(query []float32, candidates [][]float32, threshold float64) (int, float64) {
	bestIdx := -1
	bestScore := threshold

	for i, c := range candidates {
		score := CosineSimilarity(query, c)
		if score > bestScore {
			bestScore = score
			bestIdx = i
		}
	}

	return bestIdx, bestScore
}
