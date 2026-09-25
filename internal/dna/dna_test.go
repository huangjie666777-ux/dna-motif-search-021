package dna

import (
	"strings"
	"testing"
)

func TestParseFASTAOK(t *testing.T) {
	fasta := ">rec1 first record\nACgt\n  nT \n\n>rec2\nAAA\n"
	recs, issues := ParseFASTA(fasta)
	if len(issues) != 0 {
		t.Fatalf("unexpected issues: %v", issues)
	}
	if len(recs) != 2 {
		t.Fatalf("got %d records", len(recs))
	}
	if recs[0].ID != "rec1" || string(recs[0].Seq) != "ACGTNT" {
		t.Fatalf("bad first record: %+v %q", recs[0], recs[0].Seq)
	}
	if recs[1].ID != "rec2" || string(recs[1].Seq) != "AAA" {
		t.Fatalf("bad second record: %+v", recs[1])
	}
}

func TestParseFASTAErrors(t *testing.T) {
	cases := []struct {
		name  string
		fasta string
		check string
	}{
		{"empty", "", "empty FASTA"},
		{"no header", "ACGT\n", "before any FASTA header"},
		{"duplicate id", ">a\nA\n>a\nC\n", "duplicate record ID"},
		{"empty record", ">a\n>b\nC\n", "empty sequence"},
		{"illegal char", ">a\nACXGT\n", "illegal character"},
		{"blank header", ">\nACGT\n", "no record ID"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, issues := ParseFASTA(tc.fasta)
			if len(issues) == 0 {
				t.Fatalf("expected issue containing %q", tc.check)
			}
			found := false
			for _, is := range issues {
				if strings.Contains(is.String(), tc.check) {
					found = true
				}
			}
			if !found {
				t.Fatalf("issue %q not found in %v", tc.check, issues)
			}
		})
	}
}

func TestIllegalCharPosition(t *testing.T) {
	_, issues := ParseFASTA(">r1\nAC\nZ\n")
	if len(issues) != 1 || issues[0].Line != 3 || issues[0].Column != 1 {
		t.Fatalf("got %+v", issues)
	}
}

func TestReverseComplement(t *testing.T) {
	got := string(reverseComplement([]byte("ACGTN")))
	want := "NACGT"
	if got != want {
		t.Fatalf("reverseComplement = %q, want %q", got, want)
	}
}

func TestSearchForwardWithN(t *testing.T) {
	req := SearchRequest{
		FASTA:  ">r1\nACGTACGT\n",
		Motifs: []Motif{{Name: "p1", Seq: "CG"}},
		Strand: StrandForward,
	}
	_, hits, issues := Search(req)
	if len(issues) != 0 {
		t.Fatalf("issues: %v", issues)
	}
	if len(hits) != 2 {
		t.Fatalf("got %d hits: %+v", len(hits), hits)
	}
	for i, start := range []int{1, 5} {
		if hits[i].Start != start || hits[i].End != start+2 || hits[i].Strand != StrandForward {
			t.Fatalf("hit %d = %+v", i, hits[i])
		}
	}
}

func TestOverlappingHitsKept(t *testing.T) {
	req := SearchRequest{
		FASTA:  ">r1\nAAAA\n",
		Motifs: []Motif{{Name: "aa", Seq: "AA"}},
		Strand: StrandForward,
	}
	_, hits, _ := Search(req)
	if len(hits) != 3 {
		t.Fatalf("overlaps must be retained, got %d", len(hits))
	}
}

func TestReverseStrandCoordinatesMappedToForward(t *testing.T) {
	// ACGT; motif AC on reverse means searching GT on forward coordinates.
	req := SearchRequest{
		FASTA:  ">r1\nACGT\n",
		Motifs: []Motif{{Name: "m", Seq: "AC"}},
		Strand: StrandReverse,
	}
	_, hits, _ := Search(req)
	if len(hits) != 1 || hits[0].Start != 2 || hits[0].End != 4 || hits[0].Strand != StrandReverse {
		t.Fatalf("got %+v", hits)
	}
}

func TestBothStrandsMergeSameInterval(t *testing.T) {
	req := SearchRequest{
		FASTA:  ">r1\nACGT\n",
		Motifs: []Motif{{Name: "pal", Seq: "ACGT"}},
		Strand: StrandBoth,
	}
	// ACGT revcomp is itself, so same interval matches both strands.
	_, hits, _ := Search(req)
	if len(hits) != 1 || hits[0].Strand != StrandBoth {
		t.Fatalf("expected one merged both-strand hit, got %+v", hits)
	}
}

func TestCircularCrossOrigin(t *testing.T) {
	// length 6, motif ACACG at start 4 spans [4,6)+[0,3).
	req := SearchRequest{
		FASTA:    ">r1\nACGTAC\n",
		Motifs:   []Motif{{Name: "cross", Seq: "ACACG"}},
		Circular: []string{"r1"},
		Strand:   StrandForward,
	}
	_, hits, _ := Search(req)
	var cross *Hit
	for i := range hits {
		if hits[i].CrossOrigin {
			cross = &hits[i]
		}
	}
	if cross == nil {
		t.Fatalf("expected cross-origin hit, got %+v", hits)
	}
	if cross.Start != 4 || cross.End != 3 {
		t.Fatalf("bad coords: %+v", cross)
	}
	if len(cross.Segments) != 2 || cross.Segments[0] != (Segment{4, 6}) || cross.Segments[1] != (Segment{0, 3}) {
		t.Fatalf("bad segments: %+v", cross.Segments)
	}
}

func TestCircularStartsCheckedOnceAndLongMotifSkipped(t *testing.T) {
	req := SearchRequest{
		FASTA:    ">r1\nAAAA\n",
		Motifs:   []Motif{{Name: "aa", Seq: "AA"}, {Name: "long", Seq: "AAAAA"}},
		Circular: []string{"r1"},
		Strand:   StrandForward,
	}
	_, hits, _ := Search(req)
	if len(hits) != 4 {
		t.Fatalf("each start checked once around the circle; got %d hits: %+v", len(hits), hits)
	}
}

func TestLinearNeverWraps(t *testing.T) {
	req := SearchRequest{
		FASTA:  ">r1\nACGTA\n",
		Motifs: []Motif{{Name: "w", Seq: "AAC"}},
		Strand: StrandForward,
	}
	_, hits, _ := Search(req)
	if len(hits) != 0 {
		t.Fatalf("linear records must not wrap, got %+v", hits)
	}
}

func TestUnknownCircularIDAndBadMotifReject(t *testing.T) {
	req := SearchRequest{
		FASTA:    ">r1\nACGT\n",
		Motifs:   []Motif{{Name: "", Seq: "X"}},
		Circular: []string{"nope"},
		Strand:   StrandForward,
	}
	_, hits, issues := Search(req)
	if hits != nil || len(issues) < 2 {
		t.Fatalf("request must be fully rejected, hits=%v issues=%v", hits, issues)
	}
}

func TestStableOrderAndPagination(t *testing.T) {
	req := SearchRequest{
		FASTA: ">r1\nAAAA\n>r2\nAAAA\n",
		Motifs: []Motif{
			{Name: "first", Seq: "AA"},
			{Name: "second", Seq: "AA"},
		},
		Strand: StrandForward,
	}
	_, hits, _ := Search(req)
	// 3 starts * 2 motifs * 2 records = 12
	if len(hits) != 12 {
		t.Fatalf("got %d hits", len(hits))
	}
	if hits[0].RecordID != "r1" || hits[0].Start != 0 || hits[0].MotifName != "first" {
		t.Fatalf("bad first hit: %+v", hits[0])
	}
	if hits[1].MotifName != "second" {
		t.Fatalf("same start must order by motif input order: %+v", hits[1])
	}
	if hits[6].RecordID != "r2" {
		t.Fatalf("record order wrong: %+v", hits[6])
	}
	p1 := Paginate(hits, 1, 5)
	p2 := Paginate(hits, 2, 5)
	p3 := Paginate(hits, 3, 5)
	if p1.Total != 12 || len(p1.Hits) != 5 || len(p2.Hits) != 5 || len(p3.Hits) != 2 {
		t.Fatalf("pagination sizes wrong: %d %d %d", len(p1.Hits), len(p2.Hits), len(p3.Hits))
	}
	if sameHit(p1.Hits[4], p2.Hits[0]) || sameHit(p2.Hits[4], p3.Hits[0]) {
		t.Fatal("pages overlap or duplicate")
	}
}

func sameHit(a, b Hit) bool {
	return a.RecordID == b.RecordID && a.MotifName == b.MotifName &&
		a.Start == b.Start && a.Strand == b.Strand
}

func TestNMatchingBothEnds(t *testing.T) {
	req := SearchRequest{
		FASTA:  ">r1\nACGT\n",
		Motifs: []Motif{{Name: "n", Seq: "NGN"}},
		Strand: StrandForward,
	}
	_, hits, _ := Search(req)
	if len(hits) != 1 || hits[0].Start != 1 {
		t.Fatalf("N at both ends must match, got %+v", hits)
	}
}
