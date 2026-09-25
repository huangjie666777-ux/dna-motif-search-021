package dna

import (
	"testing"
)

func TestParseFASTAOK(t *testing.T) {
	fasta := ">r1 first record\nACgt\n\nna\n>r2\nTTTT\n"
	recs, errs := ParseFASTA(fasta)
	if errs != nil {
		t.Fatalf("unexpected errors: %v", errs)
	}
	if len(recs) != 2 {
		t.Fatalf("got %d records", len(recs))
	}
	if recs[0].ID != "r1" || recs[0].Seq != "ACGTNA" {
		t.Fatalf("bad record 1: %+v", recs[0])
	}
	if recs[0].Order != 0 || recs[1].Order != 1 {
		t.Fatal("record order not kept")
	}
}

func TestParseFASTAErrors(t *testing.T) {
	cases := []struct {
		name  string
		fasta string
		check func(*testing.T, Errors)
	}{
		{
			name:  "empty",
			fasta: "   \n\n",
			check: func(t *testing.T, es Errors) {
				if len(es) != 1 || es[0].Field != "fasta" {
					t.Fatalf("got %+v", es)
				}
			},
		},
		{
			name:  "empty record",
			fasta: ">x\n\n>y\nACGT\n",
			check: func(t *testing.T, es Errors) {
				found := false
				for _, e := range es {
					if e.RecordID == "x" && e.Line == 1 {
						found = true
					}
				}
				if !found {
					t.Fatalf("missing empty-record error: %+v", es)
				}
			},
		},
		{
			name:  "duplicate id",
			fasta: ">x\nACGT\n>x\nACGT\n",
			check: func(t *testing.T, es Errors) {
				if len(es) != 1 || es[0].Line != 3 {
					t.Fatalf("got %+v", es)
				}
			},
		},
		{
			name:  "illegal char with position",
			fasta: ">x\nACXG\n",
			check: func(t *testing.T, es Errors) {
				if len(es) != 1 || es[0].Line != 2 || es[0].Column != 3 {
					t.Fatalf("got %+v", es)
				}
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, errs := ParseFASTA(tc.fasta)
			if errs == nil {
				t.Fatal("expected errors, got nil")
			}
			tc.check(t, errs)
		})
	}
}

func TestReverseComplement(t *testing.T) {
	if got := ReverseComplement("ACGTN"); got != "NACGT" {
		t.Fatalf("got %s", got)
	}
}

func rec(id, seq string) Record { return Record{ID: id, Seq: seq} }
func pats(names ...string) []Pattern {
	ps := make([]Pattern, len(names))
	for i, n := range names {
		ps[i] = NewPattern(n, n)
	}
	return ps
}

func TestLinearOverlapsAndOrder(t *testing.T) {
	r := rec("r", "AAA")
	hits := Search(SearchOptions{
		Records:  []Record{r},
		Patterns: pats("AA", "A"),
		Strand:   "forward",
	})
	if len(hits) != 5 { // AA@0, A@0, AA@1, A@1, A@2
		t.Fatalf("got %d hits: %+v", len(hits), hits)
	}
	want := []struct {
		start int
		name  string
	}{
		{0, "AA"}, {0, "A"}, {1, "AA"}, {1, "A"}, {2, "A"},
	}
	for i, w := range want {
		if hits[i].Start != w.start || hits[i].PatternName != w.name {
			t.Fatalf("hit %d = %+v, want start=%d name=%s", i, hits[i], w.start, w.name)
		}
	}
	if hits[0].Segments[0] != (Segment{0, 2}) || hits[0].CrossOrigin {
		t.Fatalf("bad segments: %+v", hits[0])
	}
}

func TestLinearNoWrap(t *testing.T) {
	hits := Search(SearchOptions{
		Records:  []Record{rec("r", "AAAT")},
		Patterns: pats("ATAA"),
		Strand:   "forward",
	})
	if len(hits) != 0 {
		t.Fatalf("linear record must not wrap, got %+v", hits)
	}
}

func TestCircularCrossOrigin(t *testing.T) {
	hits := Search(SearchOptions{
		Records:  []Record{rec("r", "AAAT")},
		Patterns: pats("ATAA"),
		Circular: map[string]bool{"r": true},
		Strand:   "forward",
	})
	if len(hits) != 1 {
		t.Fatalf("got %+v", hits)
	}
	h := hits[0]
	if !h.CrossOrigin || h.Start != 2 || h.End != -1 || h.Length != 4 {
		t.Fatalf("bad cross-origin hit: %+v", h)
	}
	if len(h.Segments) != 2 || h.Segments[0] != (Segment{2, 4}) || h.Segments[1] != (Segment{0, 2}) {
		t.Fatalf("bad segments: %+v", h.Segments)
	}
}

func TestPatternLongerThanRecord(t *testing.T) {
	hits := Search(SearchOptions{
		Records:  []Record{rec("r", "AC")},
		Patterns: pats("ACGT"),
		Circular: map[string]bool{"r": true},
		Strand:   "forward",
	})
	if len(hits) != 0 {
		t.Fatalf("long pattern must not hit: %+v", hits)
	}
}

func TestReverseStrandCoordinates(t *testing.T) {
	// 模式 AA 在反链等价于搜 TT；正链 "TTT" 上起点 0、1 命中，strand=reverse。
	hits := Search(SearchOptions{
		Records:  []Record{rec("r", "TTT")},
		Patterns: pats("AA"),
		Strand:   "reverse",
	})
	if len(hits) != 2 {
		t.Fatalf("got %+v", hits)
	}
	if hits[0].Strand != "reverse" || hits[0].Start != 0 {
		t.Fatalf("bad reverse hit: %+v", hits[0])
	}
}

func TestBothStrandsMerge(t *testing.T) {
	// AT 的反向互补仍是 AT：同模式同区间双链同时命中，合并为 both。
	hits := Search(SearchOptions{
		Records:  []Record{rec("r", "ATAT")},
		Patterns: pats("AT"),
		Strand:   "both",
	})
	if len(hits) != 2 {
		t.Fatalf("merged hits should be 2, got %+v", hits)
	}
	for _, h := range hits {
		if h.Strand != "both" {
			t.Fatalf("expected merged both hit, got %+v", h)
		}
	}

	// 不同名称的模式即使同区间也不合并。
	hits2 := Search(SearchOptions{
		Records:  []Record{rec("r", "AT")},
		Patterns: []Pattern{NewPattern("x", "AT"), NewPattern("y", "AT")},
		Strand:   "both",
	})
	if len(hits2) != 2 {
		t.Fatalf("different names must not merge: %+v", hits2)
	}
}

func TestNWildcardEnds(t *testing.T) {
	hits := Search(SearchOptions{
		Records:  []Record{rec("r", "ACGT")},
		Patterns: []Pattern{NewPattern("p", "NCNT")},
		Circular: map[string]bool{},
		Strand:   "forward",
	})
	if len(hits) != 1 || hits[0].Start != 0 {
		t.Fatalf("N wildcard match failed: %+v", hits)
	}
}

func TestRecordOrderSorting(t *testing.T) {
	hits := Search(SearchOptions{
		Records:  []Record{{ID: "b", Seq: "AA", Order: 1}, {ID: "a", Seq: "AA", Order: 0}},
		Patterns: pats("A"),
		Strand:   "forward",
	})
	if len(hits) != 4 || hits[0].RecordID != "a" {
		t.Fatalf("bad record ordering: %+v", hits)
	}
}
