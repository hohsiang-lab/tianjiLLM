package handler

import "sync"

type stickyStrategyCandidate struct {
	ID            string
	Payload       any
	Metadata      any
	MetadataScore int64
}

type stickyStrategyEntry struct {
	SelectedID string
	Metadata   any
}

type stickyStrategyPolicy struct {
	CanReuse func(stickyStrategyEntry, stickyStrategyCandidate) bool
	Select   func([]stickyStrategyCandidate) stickyStrategyCandidate
}

type stickyStrategyStore struct {
	mu      sync.Mutex
	entries map[string]stickyStrategyEntry
}

func (s *stickyStrategyStore) Select(trackKey string, candidates []stickyStrategyCandidate, policy stickyStrategyPolicy) (stickyStrategyCandidate, stickyStrategyEntry, bool) {
	if len(candidates) == 0 || trackKey == "" {
		return stickyStrategyCandidate{}, stickyStrategyEntry{}, false
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if s.entries == nil {
		s.entries = make(map[string]stickyStrategyEntry)
	}

	if entry, ok := s.entries[trackKey]; ok && entry.SelectedID != "" {
		for _, candidate := range candidates {
			if candidate.ID != entry.SelectedID {
				continue
			}
			if policy.CanReuse == nil || policy.CanReuse(entry, candidate) {
				entry = stickyStrategyEntry{SelectedID: candidate.ID, Metadata: candidate.Metadata}
				s.entries[trackKey] = entry
				return candidate, entry, true
			}
			break
		}
	}

	selected := candidates[0]
	if policy.Select != nil {
		selected = policy.Select(candidates)
	}
	entry := stickyStrategyEntry{SelectedID: selected.ID, Metadata: selected.Metadata}
	s.entries[trackKey] = entry
	return selected, entry, true
}

func (s *stickyStrategyStore) SeedIfAbsent(trackKey string, entry stickyStrategyEntry) {
	if trackKey == "" || entry.SelectedID == "" {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.entries == nil {
		s.entries = make(map[string]stickyStrategyEntry)
	}
	if _, ok := s.entries[trackKey]; !ok {
		s.entries[trackKey] = entry
	}
}

func selectLowestStickyScore(candidates []stickyStrategyCandidate, score func(stickyStrategyCandidate) (int64, bool)) stickyStrategyCandidate {
	if len(candidates) == 0 {
		return stickyStrategyCandidate{}
	}
	selected := candidates[0]
	selectedSet := false
	var bestScore int64
	for _, candidate := range candidates {
		candidateScore, ok := score(candidate)
		if !ok {
			continue
		}
		if !selectedSet || candidateScore < bestScore {
			selected = candidate
			bestScore = candidateScore
			selectedSet = true
		}
	}
	return selected
}
