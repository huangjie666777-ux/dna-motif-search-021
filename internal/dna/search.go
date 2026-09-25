package dna

import (
	"fmt"
	"sort"
)

const (
	StrandForward  = "forward"
	StrandReverse  = "reverse"
	StrandBoth     = "both"
	MaxMotifCount  = 100
	MaxMotifLength = 256
)

type Motif struct {
	Name string `json:"name"`
	Seq  string `json:"sequence"`
}

// Segment is one half of a hit on the forward (input) strand.
type Segment struct {
	Start int `json:"start"`
	End   int `json:"end"`
}

// Hit always uses forward-strand coordinates, 0-based and half-open.
// A cross-origin hit has two segments: [start, recordLength) and [0, end).
type Hit struct {
	RecordID    string    `json:"record_id"`
	MotifName   string    `json:"motif_name"`
	Strand      string    `json:"strand"` // forward, reverse or both
	Start       int       `json:"start"`
	End         int       `json:"end"`
	CrossOrigin bool      `json:"cross_origin"`
	Segments    []Segment `json:"segments"`
	Circular    bool      `json:"circular"`
	MotifIndex  int       `json:"-"`
}

type SearchRequest struct {
	FASTA    string   `json:"fasta"`
	Motifs   []Motif  `json:"motifs"`
	Circular []string `json:"circular_ids"`
	Strand   string   `json:"strand"`
	Page     int      `json:"page"`
	PageSize int      `json:"page_size"`
}

type SearchResult struct {
	Total    int   `json:"total"`
	Page     int   `json:"page"`
	PageSize int   `json:"page_size"`
	Hits     []Hit `json:"hits"`
}

// ValidateMotifs normalizes motif sequences to upper case and returns issues.
func ValidateMotifs(motifs []Motif) ([]Motif, []Issue) {
	var issues []Issue
	normalized := make([]Motif, len(motifs))
	seen := map[string]int{}
	if len(motifs) == 0 {
		issues = append(issues, Issue{Field: "motifs", Message: "at least one pattern is required"})
	}
	if len(motifs) > MaxMotifCount {
		issues = append(issues, Issue{
			Field:   "motifs",
			Message: fmt.Sprintf("too many patterns: %d > limit %d", len(motifs), MaxMotifCount),
		})
	}
	for i, m := range motifs {
		field := fmt.Sprintf("motifs[%d].name", i)
		if m.Name == "" {
			issues = append(issues, Issue{Field: field, Message: "pattern name must not be empty"})
		} else if prev, ok := seen[m.Name]; ok {
			issues = append(issues, Issue{Field: field, Message: fmt.Sprintf("duplicate pattern name %q (first used at motifs[%d])", m.Name, prev)})
		} else {
			seen[m.Name] = i
		}

		sfield := fmt.Sprintf("motifs[%d].sequence", i)
		if len(m.Seq) == 0 {
			issues = append(issues, Issue{Field: sfield, Message: "pattern sequence must not be empty"})
			continue
		}
		if len(m.Seq) > MaxMotifLength {
			issues = append(issues, Issue{Field: sfield, Message: fmt.Sprintf("pattern too long: %d > limit %d", len(m.Seq), MaxMotifLength)})
		}
		buf := make([]byte, 0, len(m.Seq))
		for col, ch := range m.Seq {
			u := byte(ch)
			if u >= 'a' && u <= 'z' {
				u -= 'a' - 'A'
			}
			switch u {
			case 'A', 'C', 'G', 'T', 'N':
				buf = append(buf, u)
			default:
				issues = append(issues, Issue{
					Field:   sfield,
					Line:    0,
					Column:  col + 1,
					Message: fmt.Sprintf("illegal character %q in pattern; only ACGTN (case insensitive) are allowed", ch),
				})
			}
		}
		normalized[i] = Motif{Name: m.Name, Seq: string(buf)}
	}
	return normalized, issues
}

func reverseComplement(seq []byte) []byte {
	out := make([]byte, len(seq))
	comp := map[byte]byte{'A': 'T', 'T': 'A', 'C': 'G', 'G': 'C', 'N': 'N'}
	for i, b := range seq {
		out[len(seq)-1-i] = comp[b]
	}
	return out
}

// baseMatch treats N on either side as matching any single base.
func baseMatch(a, b byte) bool {
	return a == 'N' || b == 'N' || a == b
}

func matchAt(seq, motif []byte, start, length int, circular bool) bool {
	if length > len(seq) {
		return false
	}
	if !circular && start+length > len(seq) {
		return false
	}
	for j := 0; j < length; j++ {
		pos := start + j
		if circular {
			pos %= len(seq)
		}
		if !baseMatch(seq[pos], motif[j]) {
			return false
		}
	}
	return true
}

func buildHit(rec Record, motif Motif, motifIndex, start int, strand string, circular bool) Hit {
	ml := len(motif.Seq)
	hit := Hit{
		RecordID:   rec.ID,
		MotifName:  motif.Name,
		Strand:     strand,
		Start:      start,
		MotifIndex: motifIndex,
		Circular:   circular,
	}
	end := start + ml
	if circular && end > len(rec.Seq) {
		hit.CrossOrigin = true
		hit.End = end - len(rec.Seq)
		hit.Segments = []Segment{
			{Start: start, End: len(rec.Seq)},
			{Start: 0, End: hit.End},
		}
	} else {
		hit.End = end
		hit.Segments = []Segment{{Start: start, End: end}}
	}
	return hit
}

// Search validates the full request and either rejects it (no partial
// results) or returns every hit deterministically ordered.
func Search(req SearchRequest) ([]Record, []Hit, []Issue) {
	var allIssues []Issue

	records, fastaIssues := ParseFASTA(req.FASTA)
	allIssues = append(allIssues, fastaIssues...)

	normMotifs, motifIssues := ValidateMotifs(req.Motifs)
	allIssues = append(allIssues, motifIssues...)

	if req.Strand != StrandForward && req.Strand != StrandReverse && req.Strand != StrandBoth {
		allIssues = append(allIssues, Issue{
			Field:   "strand",
			Message: fmt.Sprintf("strand must be one of %q, %q, %q", StrandForward, StrandReverse, StrandBoth),
		})
	}

	recordSet := map[string]bool{}
	for _, r := range records {
		recordSet[r.ID] = true
	}
	circularSet := map[string]bool{}
	for i, id := range req.Circular {
		field := fmt.Sprintf("circular_ids[%d]", i)
		if id == "" {
			allIssues = append(allIssues, Issue{Field: field, Message: "circular record ID must not be empty"})
			continue
		}
		if !recordSet[id] {
			allIssues = append(allIssues, Issue{Field: field, Message: fmt.Sprintf("unknown circular record ID %q", id)})
		}
		if circularSet[id] {
			allIssues = append(allIssues, Issue{Field: field, Message: fmt.Sprintf("circular record ID %q listed more than once", id)})
		}
		circularSet[id] = true
	}

	if len(allIssues) > 0 {
		return nil, nil, allIssues
	}

	wantFwd := req.Strand == StrandForward || req.Strand == StrandBoth
	wantRev := req.Strand == StrandReverse || req.Strand == StrandBoth

	var hits []Hit
	for _, rec := range records {
		circular := circularSet[rec.ID]
		for mi, motif := range normMotifs {
			fwdPat := []byte(motif.Seq)
			revPat := reverseComplement(fwdPat)
			ml := len(fwdPat)
			if ml > len(rec.Seq) {
				continue
			}
			limit := len(rec.Seq)
			if !circular {
				limit = len(rec.Seq) - ml + 1
			}
			for start := 0; start < limit; start++ {
				onFwd := wantFwd && matchAt(rec.Seq, fwdPat, start, ml, circular)
				onRev := wantRev && matchAt(rec.Seq, revPat, start, ml, circular)
				strand := ""
				switch {
				case onFwd && onRev:
					strand = StrandBoth
				case onFwd:
					strand = StrandForward
				case onRev:
					strand = StrandReverse
				default:
					continue
				}
				hits = append(hits, buildHit(rec, motif, mi, start, strand, circular))
			}
		}
	}

	sort.SliceStable(hits, func(i, j int) bool {
		if hits[i].RecordID != hits[j].RecordID {
			return recordOrder(records, hits[i].RecordID) < recordOrder(records, hits[j].RecordID)
		}
		if hits[i].Start != hits[j].Start {
			return hits[i].Start < hits[j].Start
		}
		return hits[i].MotifIndex < hits[j].MotifIndex
	})

	return records, hits, nil
}

func recordOrder(records []Record, id string) int {
	for i, r := range records {
		if r.ID == id {
			return i
		}
	}
	return len(records)
}

// Paginate returns a stable page; page numbers are 1-based.
func Paginate(hits []Hit, page, pageSize int) SearchResult {
	if pageSize <= 0 {
		pageSize = 50
	}
	if page <= 0 {
		page = 1
	}
	res := SearchResult{Total: len(hits), Page: page, PageSize: pageSize, Hits: []Hit{}}
	start := (page - 1) * pageSize
	if start >= len(hits) {
		return res
	}
	end := start + pageSize
	if end > len(hits) {
		end = len(hits)
	}
	res.Hits = hits[start:end]
	return res
}
