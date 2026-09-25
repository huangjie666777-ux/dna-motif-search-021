package api

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/huangjie666777-ux/dna-motif-search-021/internal/dna"
)

const (
	MaxBodyBytes = 2 << 20 // 2 MiB
	MaxPageSize  = 500
	DefaultPages = 50
)

func NewRouter() http.Handler {
	r := chi.NewRouter()
	r.Get("/healthz", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	r.Post("/api/v1/search", searchHandler)
	return r
}

type errorResponse struct {
	Error  string      `json:"error"`
	Issues []dna.Issue `json:"issues,omitempty"`
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeIssues(w http.ResponseWriter, status int, msg string, issues []dna.Issue) {
	writeJSON(w, status, errorResponse{Error: msg, Issues: issues})
}

func searchHandler(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, MaxBodyBytes)
	raw, err := io.ReadAll(r.Body)
	if err != nil {
		status := http.StatusBadRequest
		msg := "could not read request body"
		var mbe *http.MaxBytesError
		if errors.As(err, &mbe) {
			status = http.StatusRequestEntityTooLarge
			msg = "request body too large (limit 2 MiB)"
		}
		writeIssues(w, status, msg, []dna.Issue{{Field: "body", Message: err.Error()}})
		return
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	var req dna.SearchRequest
	if err := dec.Decode(&req); err != nil {
		status := http.StatusBadRequest
		msg := "invalid JSON request body"
		writeIssues(w, status, msg, []dna.Issue{{
			Field:   "body",
			Message: err.Error(),
		}})
		return
	}

	var extraIssues []dna.Issue
	if req.PageSize > MaxPageSize {
		extraIssues = append(extraIssues, dna.Issue{
			Field:   "page_size",
			Message: pageSizeMsg(),
		})
	}
	if req.Page < 0 {
		extraIssues = append(extraIssues, dna.Issue{
			Field:   "page",
			Message: "page must be >= 1 (omit for first page)",
		})
	}

	records, hits, issues := dna.Search(req)
	issues = append(issues, extraIssues...)
	if len(issues) > 0 {
		writeIssues(w, http.StatusBadRequest, "request rejected: validation failed; no partial results returned", issues)
		return
	}
	_ = records

	page := req.Page
	if page == 0 {
		page = 1
	}
	pageSize := req.PageSize
	if pageSize == 0 {
		pageSize = DefaultPages
	}
	writeJSON(w, http.StatusOK, dna.Paginate(hits, page, pageSize))
}

func pageSizeMsg() string {
	return "page_size must be between 1 and 500"
}
