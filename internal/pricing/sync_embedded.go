package pricing

// selectEmbeddedToInsert returns model names from the embedded map that are
// NOT present in existingDB. This implements the HO-83 insert-only fallback:
// models already in DB are never overwritten.
func selectEmbeddedToInsert(embedded map[string]ModelInfo, existingDB map[string]struct{}) []string {
	result := make([]string, 0, len(embedded))
	for name := range embedded {
		if _, exists := existingDB[name]; !exists {
			result = append(result, name)
		}
	}
	return result
}
