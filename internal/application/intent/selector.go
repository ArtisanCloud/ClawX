package intent

import "sort"

func SelectCandidate(candidates []Candidate) (Candidate, bool) {
	if len(candidates) == 0 {
		return Candidate{}, false
	}
	if len(candidates) == 1 {
		return candidates[0], true
	}

	ordered := append([]Candidate(nil), candidates...)
	sort.Slice(ordered, func(i, j int) bool {
		if ordered[i].Confidence != ordered[j].Confidence {
			return ordered[i].Confidence > ordered[j].Confidence
		}
		if ordered[i].Reason != ordered[j].Reason {
			return ordered[i].Reason < ordered[j].Reason
		}
		return ordered[i].SkillName < ordered[j].SkillName
	})
	return ordered[0], true
}
