package pages

// filterAvailableKeys returns keys from allKeys that are not already in members.
func filterAvailableKeys(allKeys []KeyMemberOption, members []KeyMemberOption) []KeyMemberOption {
	memberSet := make(map[string]struct{}, len(members))
	for _, m := range members {
		memberSet[m.Token] = struct{}{}
	}
	var result []KeyMemberOption
	for _, k := range allKeys {
		if _, ok := memberSet[k.Token]; !ok {
			result = append(result, k)
		}
	}
	return result
}
