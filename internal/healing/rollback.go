package healing

import (
	"fmt"
	"sort"
)

func SelectLatestHealthyRevision(records []RevisionRecord) (RevisionRecord, error) {
	if len(records) == 0 {
		return RevisionRecord{}, fmt.Errorf("no revisions found")
	}
	sorted := append([]RevisionRecord(nil), records...)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].UnixTime > sorted[j].UnixTime
	})
	for _, rec := range sorted {
		if rec.Healthy {
			return rec, nil
		}
	}
	return RevisionRecord{}, fmt.Errorf("no healthy revision available")
}
