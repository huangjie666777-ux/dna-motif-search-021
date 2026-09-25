package dna

import (
	"fmt"
	"strings"
	"unicode"
)

// Record 是一条 FASTA 记录，Seq 为大写后的 ACGTN 序列。
type Record struct {
	ID        string // 标题行首个词
	HeaderIdx int    // 标题所在行号（从 1 开始），用于错误定位
	Order     int    // 记录输入顺序（从 0 开始）
	Seq       string
}

func isSpaceByte(b byte) bool {
	return b == ' ' || b == '\t' || b == '\r' || unicode.IsSpace(rune(b))
}

// ParseError 描述一处输入错误及其位置。
type ParseError struct {
	Field    string `json:"field,omitempty"`
	Record   int    `json:"record_index,omitempty"` // 从 0 开始
	RecordID string `json:"record_id,omitempty"`
	Line     int    `json:"line,omitempty"`   // 从 1 开始
	Column   int    `json:"column,omitempty"` // 从 1 开始
	Message  string `json:"message"`
}

func (e ParseError) Error() string {
	switch {
	case e.Line > 0 && e.Column > 0:
		return fmt.Sprintf("line %d column %d: %s", e.Line, e.Column, e.Message)
	case e.Line > 0:
		return fmt.Sprintf("line %d: %s", e.Line, e.Message)
	case e.RecordID != "":
		return fmt.Sprintf("record %q: %s", e.RecordID, e.Message)
	default:
		return e.Message
	}
}

// Errors 聚合同一次请求中的全部错误，整批校验失败时返回。
type Errors []ParseError

func (es Errors) Error() string {
	parts := make([]string, len(es))
	for i, e := range es {
		parts[i] = e.Error()
	}
	return strings.Join(parts, "; ")
}

func validBaseUpper(b byte) (byte, bool) {
	switch b {
	case 'A', 'a':
		return 'A', true
	case 'C', 'c':
		return 'C', true
	case 'G', 'g':
		return 'G', true
	case 'T', 't':
		return 'T', true
	case 'N', 'n':
		return 'N', true
	default:
		return 0, false
	}
}

// ParseFASTA 解析 FASTA 文本，返回记录或全部带位置的错误。
func ParseFASTA(text string) ([]Record, Errors) {
	var errs Errors
	lines := strings.Split(text, "\n")

	var records []Record
	var cur *Record
	var seqBuf strings.Builder
	seen := map[string]int{}

	flush := func() {
		if cur == nil {
			return
		}
		cur.Seq = seqBuf.String()
		records = append(records, *cur)
		cur = nil
		seqBuf.Reset()
	}

	hasContent := false
	for i, raw := range lines {
		lineNo := i + 1
		line := strings.TrimSuffix(raw, "\r")
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		hasContent = true
		if trimmed[0] == '>' {
			flush()
			rest := strings.TrimSpace(line[1:])
			id := firstField(rest)
			if id == "" {
				errs = append(errs, ParseError{Line: lineNo, Message: "FASTA 标题行缺少记录 ID（标题首个词为空）"})
				id = fmt.Sprintf("__invalid_%d", lineNo)
			}
			if prev, ok := seen[id]; ok {
				errs = append(errs, ParseError{Record: len(records), RecordID: id, Line: lineNo, Message: fmt.Sprintf("记录 ID 重复，首次出现在第 %d 行", prev)})
			} else {
				seen[id] = lineNo
			}
			cur = &Record{ID: id, HeaderIdx: lineNo, Order: len(records)}
			continue
		}
		if cur == nil {
			errs = append(errs, ParseError{Line: lineNo, Message: "序列出现在任何 FASTA 标题（>）之前"})
			continue
		}
		for col := 0; col < len(line); col++ {
			b := line[col]
			if isSpaceByte(b) {
				continue
			}
			u, ok := validBaseUpper(b)
			if !ok {
				errs = append(errs, ParseError{
					Record: cur.Order, RecordID: cur.ID,
					Line: lineNo, Column: col + 1,
					Message: fmt.Sprintf("非法碱基字符 %q，仅允许 ACGTN（不区分大小写）", string(b)),
				})
				continue
			}
			seqBuf.WriteByte(u)
		}
	}
	flush()

	if len(errs) > 0 {
		return nil, errs
	}
	if !hasContent || len(records) == 0 {
		return nil, Errors{{Field: "fasta", Message: "FASTA 为空或不含任何记录"}}
	}
	for i, rec := range records {
		if rec.Seq == "" {
			errs = append(errs, ParseError{Record: i, RecordID: rec.ID, Line: rec.HeaderIdx, Message: "记录序列为空"})
		}
	}
	if len(errs) > 0 {
		return nil, errs
	}
	return records, nil
}

func firstField(s string) string {
	s = strings.TrimLeft(s, " \t")
	for i := 0; i < len(s); i++ {
		if s[i] == ' ' || s[i] == '\t' {
			return s[:i]
		}
	}
	return s
}

// ReverseComplement 返回模式的反向互补：A<->T、C<->G、N 不变。
func ReverseComplement(p string) string {
	b := make([]byte, len(p))
	for i := 0; i < len(p); i++ {
		var c byte
		switch p[len(p)-1-i] {
		case 'A':
			c = 'T'
		case 'T':
			c = 'A'
		case 'C':
			c = 'G'
		case 'G':
			c = 'C'
		default: // N
			c = 'N'
		}
		b[i] = c
	}
	return string(b)
}
