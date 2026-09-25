package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/huangjie666777-ux/dna-motif-search-021/internal/dna"
)

const (
	MaxBodyBytes     = 1 << 20 // 1 MiB
	MaxPatterns      = 64
	MaxPatternLength = 10_000
	DefaultPageSize  = 50
	MaxPageSize      = 1000
)

type patternInput struct {
	Name    string `json:"name"`
	Pattern string `json:"pattern"`
}

type searchRequest struct {
	FASTA    string         `json:"fasta"`
	Patterns []patternInput `json:"patterns"`
	Circular []string       `json:"circular_record_ids"`
	Strand   string         `json:"strand"` // forward | reverse | both
	Page     int            `json:"page"`
	PageSize int            `json:"page_size"`
}

// NewRouter 构建服务路由。
func NewRouter() http.Handler {
	r := chi.NewRouter()
	r.Get("/healthz", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	r.Post("/search", handleSearch)
	return r
}

func handleSearch(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, MaxBodyBytes)
	raw, err := io.ReadAll(r.Body)
	if err != nil {
		var maxErr *http.MaxBytesError
		if errors.As(err, &maxErr) || strings.Contains(err.Error(), "request body too large") {
			writeError(w, http.StatusRequestEntityTooLarge, dna.Errors{{Field: "body", Message: fmt.Sprintf("请求体超过 %d 字节上限", MaxBodyBytes)}})
			return
		}
		writeError(w, http.StatusBadRequest, dna.Errors{{Field: "body", Message: "读取请求体失败: " + err.Error()}})
		return
	}

	var req searchRequest
	dec := json.NewDecoder(strings.NewReader(string(raw)))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, dna.Errors{{Field: "body", Message: "JSON 解析失败: " + err.Error()}})
		return
	}

	hits, status, verrs := runSearch(&req)
	if verrs != nil {
		writeError(w, status, verrs)
		return
	}

	total := len(hits)
	page, size := req.Page, req.PageSize
	if page <= 0 {
		page = 1
	}
	if size <= 0 {
		size = DefaultPageSize
	}
	if size > MaxPageSize {
		size = MaxPageSize
	}
	start := (page - 1) * size
	pageHits := []dna.Hit{}
	if start < total {
		end := start + size
		if end > total {
			end = total
		}
		pageHits = hits[start:end]
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"total":     total,
		"page":      page,
		"page_size": size,
		"hits":      pageHits,
	})
}

func runSearch(req *searchRequest) ([]dna.Hit, int, dna.Errors) {
	var errs dna.Errors

	var records []dna.Record
	if strings.TrimSpace(req.FASTA) == "" {
		errs = append(errs, dna.ParseError{Field: "fasta", Message: "fasta 字段为空"})
	} else {
		recs, ferrs := dna.ParseFASTA(req.FASTA)
		if ferrs != nil {
			errs = append(errs, ferrs...)
		} else {
			records = recs
		}
	}

	if len(req.Patterns) == 0 {
		errs = append(errs, dna.ParseError{Field: "patterns", Message: "patterns 为空，至少需要一个模式"})
	}
	if len(req.Patterns) > MaxPatterns {
		errs = append(errs, dna.ParseError{Field: "patterns", Message: fmt.Sprintf("模式数量 %d 超过上限 %d", len(req.Patterns), MaxPatterns)})
	}

	patternNames := map[string]int{}
	patterns := make([]dna.Pattern, 0, len(req.Patterns))
	for i, p := range req.Patterns {
		if i >= MaxPatterns {
			break
		}
		fname := fmt.Sprintf("patterns[%d].name", i)
		fpatt := fmt.Sprintf("patterns[%d].pattern", i)
		if strings.TrimSpace(p.Name) == "" {
			errs = append(errs, dna.ParseError{Field: fname, Message: "模式名称为空"})
		} else if prev, ok := patternNames[p.Name]; ok {
			errs = append(errs, dna.ParseError{Field: fname, Message: fmt.Sprintf("模式名称 %q 重复，首次位于 patterns[%d]", p.Name, prev)})
		} else {
			patternNames[p.Name] = i
		}
		if p.Pattern == "" {
			errs = append(errs, dna.ParseError{Field: fpatt, Message: "模式序列为空"})
		} else if len(p.Pattern) > MaxPatternLength {
			errs = append(errs, dna.ParseError{Field: fpatt, Message: fmt.Sprintf("模式长度 %d 超过上限 %d", len(p.Pattern), MaxPatternLength)})
		} else {
			upper := strings.ToUpper(p.Pattern)
			valid := true
			for j := 0; j < len(upper); j++ {
				switch upper[j] {
				case 'A', 'C', 'G', 'T', 'N':
				default:
					errs = append(errs, dna.ParseError{
						Field:   fpatt,
						Column:  j + 1,
						Message: fmt.Sprintf("非法字符 %q，仅允许 ACGTN", string(p.Pattern[j])),
					})
					valid = false
				}
			}
			if valid {
				patterns = append(patterns, dna.NewPattern(p.Name, upper))
			}
		}
	}

	strand := req.Strand
	if strand == "" {
		strand = "both"
	}
	if strand != "forward" && strand != "reverse" && strand != "both" {
		errs = append(errs, dna.ParseError{Field: "strand", Message: "strand 仅允许 forward、reverse、both"})
	}

	circular := map[string]bool{}
	recordIDs := map[string]bool{}
	for _, rec := range records {
		recordIDs[rec.ID] = true
	}
	for i, id := range req.Circular {
		if id == "" || !recordIDs[id] {
			errs = append(errs, dna.ParseError{
				Field:   "circular_record_ids",
				Column:  i + 1,
				Message: fmt.Sprintf("未知环状记录 ID %q", id),
			})
		}
		circular[id] = true
	}

	if len(errs) > 0 {
		return nil, http.StatusBadRequest, errs
	}

	hits := dna.Search(dna.SearchOptions{
		Records:  records,
		Patterns: patterns,
		Circular: circular,
		Strand:   strand,
	})
	return hits, http.StatusOK, nil
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

type errorBody struct {
	Error  string           `json:"error"`
	Errors []dna.ParseError `json:"errors"`
}

func writeError(w http.ResponseWriter, status int, errs dna.Errors) {
	details := make([]dna.ParseError, len(errs))
	copy(details, errs)
	writeJSON(w, status, errorBody{Error: "请求校验失败，整次请求已拒绝", Errors: details})
}
