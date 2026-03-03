package healing

import "testing"

func TestSelectLatestHealthyRevision(t *testing.T) {
	recs := []RevisionRecord{
		{Revision: "1", UnixTime: 100, Healthy: true},
		{Revision: "2", UnixTime: 200, Healthy: false},
		{Revision: "3", UnixTime: 300, Healthy: true},
	}
	r, err := SelectLatestHealthyRevision(recs)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if r.Revision != "3" {
		t.Fatalf("want latest healthy revision 3, got %s", r.Revision)
	}
}

func TestSelectLatestHealthyRevisionNoHealthy(t *testing.T) {
	recs := []RevisionRecord{{Revision: "1", UnixTime: 1, Healthy: false}}
	if _, err := SelectLatestHealthyRevision(recs); err == nil {
		t.Fatalf("expected error")
	}
}
