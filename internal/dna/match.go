package dna

import (
	"sort"
)

// Segment 是沿输入正链向右排列的一段命中坐标，0 开始、左闭右开。
type Segment struct {
	Start int `json:"start"`
	End   int `json:"end"`
}

// Hit 是一条模式命中。
type Hit struct {
	RecordID    string    `json:"record_id"`
	PatternName string    `json:"pattern_name"`
	Strand      string    `json:"strand"` // forward | reverse | both
	Start       int       `json:"start"`  // 命中起点（正链坐标，从 0 开始）
	End         int       `json:"end"`    // 非跨原点时的右开终点
	Length      int       `json:"length"`
	CrossOrigin bool      `json:"cross_origin"`
	Segments    []Segment `json:"segments"` // 沿正链向右排列的坐标段

	patternOrder int
	recordOrder  int
}

func baseMatch(pat, base byte) bool {
	return pat == 'N' || pat == base
}

func matchAt(seq, pat string, start int) bool {
	n := len(seq)
	for i := 0; i < len(pat); i++ {
		if !baseMatch(pat[i], seq[(start+i)%n]) {
			return false
		}
	}
	return true
}

// SearchOptions 控制一次检索。
type SearchOptions struct {
	Records  []Record
	Patterns []Pattern
	Circular map[string]bool
	Strand   string // forward | reverse | both
}

// Pattern 是带唯一名称的检索模式（已大写、已校验）。
type Pattern struct {
	Name     string
	Seq      string
	opposite string
}

// NewPattern 构造已标准化的模式（序列需先通过校验）。
func NewPattern(name, seqUpper string) Pattern {
	return Pattern{Name: name, Seq: seqUpper, opposite: ReverseComplement(seqUpper)}
}

type rawHit struct {
	recordOrder int
	recordIdx   int
	start       int
	patternIdx  int
	reverse     bool
}

// Search 执行检索并返回稳定排序、双链合并后的全部命中。
func Search(opts SearchOptions) []Hit {
	var raws []rawHit
	for recIdx, rec := range opts.Records {
		circular := opts.Circular[rec.ID]
		n := len(rec.Seq)
		for pi, pat := range opts.Patterns {
			if len(pat.Seq) > n {
				continue // 比记录长的模式不产生命中
			}
			lastStart := n - 1 // 环状：每个起点一圈
			if !circular {
				lastStart = n - len(pat.Seq) // 线性：不允许首尾拼接
			}
			if opts.Strand == "forward" || opts.Strand == "both" {
				for s := 0; s <= lastStart; s++ {
					if matchAt(rec.Seq, pat.Seq, s) {
						raws = append(raws, rawHit{rec.Order, recIdx, s, pi, false})
					}
				}
			}
			if opts.Strand == "reverse" || opts.Strand == "both" {
				for s := 0; s <= lastStart; s++ {
					if matchAt(rec.Seq, pat.opposite, s) {
						raws = append(raws, rawHit{rec.Order, recIdx, s, pi, true})
					}
				}
			}
		}
	}

	sort.SliceStable(raws, func(i, j int) bool {
		a, b := raws[i], raws[j]
		if a.recordOrder != b.recordOrder {
			return a.recordOrder < b.recordOrder
		}
		if a.start != b.start {
			return a.start < b.start
		}
		if a.patternIdx != b.patternIdx {
			return a.patternIdx < b.patternIdx
		}
		return !a.reverse && b.reverse // forward 在 reverse 前，便于合并
	})

	hits := make([]Hit, 0, len(raws))
	for i := 0; i < len(raws); i++ {
		r := raws[i]
		rec := opts.Records[r.recordIdx]
		pat := opts.Patterns[r.patternIdx]
		strand := "forward"
		if r.reverse {
			strand = "reverse"
		}
		if opts.Strand == "both" && i+1 < len(raws) {
			n := raws[i+1]
			if n.recordOrder == r.recordOrder && n.patternIdx == r.patternIdx && n.start == r.start && n.reverse != r.reverse {
				strand = "both"
				i++
			}
		}
		hits = append(hits, buildHit(rec, pat, r.start, strand, r.patternIdx))
	}
	return hits
}

func buildHit(rec Record, pat Pattern, start int, strand string, patternOrder int) Hit {
	n := len(rec.Seq)
	length := len(pat.Seq)
	h := Hit{
		RecordID:     rec.ID,
		PatternName:  pat.Name,
		Strand:       strand,
		Start:        start,
		Length:       length,
		patternOrder: patternOrder,
		recordOrder:  rec.Order,
	}
	if start+length <= n {
		h.End = start + length
		h.Segments = []Segment{{Start: start, End: h.End}}
	} else {
		h.CrossOrigin = true
		h.End = -1
		h.Segments = []Segment{
			{Start: start, End: n},
			{Start: 0, End: (start + length) % n},
		}
	}
	return h
}
