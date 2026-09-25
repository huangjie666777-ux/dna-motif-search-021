package dna

import (
	"fmt"
	"strings"
	"unicode/utf8"
)

// Record is one FASTA entry. Seq is upper-cased ACGTN bytes.
type Record struct {
	ID  string
	Seq []byte
}

// Issue describes one validation problem. Locations are 1-based for human
// display; Field names a request field when no source position applies.
type Issue struct {
	Field   string `json:"field,omitempty"`
	Line    int    `json:"line,omitempty"`
	Column  int    `json:"column,omitempty"`
	Message string `json:"message"`
}

func (i Issue) String() string {
	switch {
	case i.Line > 0 && i.Column > 0:
		return fmt.Sprintf("line %d column %d: %s", i.Line, i.Column, i.Message)
	case i.Line > 0:
		return fmt.Sprintf("line %d: %s", i.Line, i.Message)
	case i.Field != "":
		return fmt.Sprintf("%s: %s", i.Field, i.Message)
	default:
		return i.Message
	}
}

// ParseFASTA parses FASTA text. Multiline sequences, blank lines and mixed
// case are supported. The first whitespace-delimited token of a header is
// the record ID. Every issue carries a position or a field name.
func ParseFASTA(text string) ([]Record, []Issue) {
	var records []Record
	var issues []Issue

	cur := -1
	var seq []byte
	headerSeen := false
	lines := splitLines(text)

	flush := func() {
		if cur >= 0 {
			records[cur].Seq = seq
		}
	}

	for idx, line := range lines {
		lineNo := idx + 1
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		if strings.HasPrefix(line, ">") {
			headerSeen = true
			flush()
			id := strings.Fields(strings.TrimPrefix(line, ">"))
			if len(id) == 0 || id[0] == "" {
				issues = append(issues, Issue{Line: lineNo, Message: "FASTA header has no record ID"})
				id = []string{fmt.Sprintf("__invalid_%d", lineNo)}
			}
			for k, r := range records {
				if r.ID == id[0] {
					issues = append(issues, Issue{
						Line:    lineNo,
						Message: fmt.Sprintf("duplicate record ID %q (first seen at record #%d)", id[0], k+1),
					})
				}
			}
			records = append(records, Record{ID: id[0]})
			cur = len(records) - 1
			seq = nil
			continue
		}
		if !headerSeen {
			issues = append(issues, Issue{Line: lineNo, Message: "sequence data appears before any FASTA header (headers begin with '>')"})
			continue
		}
		for col, ch := range line {
			if ch == ' ' || ch == '\t' || ch == '\r' {
				continue
			}
			u := ch
			if u >= 'a' && u <= 'z' {
				u -= 'a' - 'A'
			}
			switch u {
			case 'A', 'C', 'G', 'T', 'N':
				seq = append(seq, byte(u))
			default:
				column := col + 1
				if !utf8.ValidRune(ch) {
					column = col + 1
				}
				issues = append(issues, Issue{
					Line:    lineNo,
					Column:  column,
					Message: fmt.Sprintf("illegal character %q in sequence; only ACGTN (case insensitive) are allowed", ch),
				})
			}
		}
	}
	flush()

	if !headerSeen && len(issues) == 0 {
		issues = append(issues, Issue{Field: "fasta", Message: "empty FASTA input: at least one record is required"})
	}
	for i := range records {
		if len(records[i].Seq) == 0 {
			issues = append(issues, Issue{
				Field:   fmt.Sprintf("fasta record %q", records[i].ID),
				Message: "empty sequence",
			})
		}
	}
	return records, issues
}

func splitLines(s string) []string {
	if s == "" {
		return nil
	}
	return strings.Split(s, "\n")
}
