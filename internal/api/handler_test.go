package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func postJSON(t *testing.T, body string) (*httptest.ResponseRecorder, map[string]any) {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/search", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	NewRouter().ServeHTTP(rec, req)
	var parsed map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &parsed)
	return rec, parsed
}

func TestSearchEndpointBothStrands(t *testing.T) {
	body := `{
		"fasta": ">r1\nACGT\n",
		"motifs": [{"name": "pal", "sequence": "ACGT"}],
		"strand": "both"
	}`
	rec, parsed := postJSON(t, body)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	hits := parsed["hits"].([]any)
	if len(hits) != 1 {
		t.Fatalf("got %v", parsed)
	}
	h := hits[0].(map[string]any)
	if h["strand"] != "both" || h["cross_origin"] != false {
		t.Fatalf("hit=%v", h)
	}
}

func TestSearchEndpointValidationRejected(t *testing.T) {
	body := `{
		"fasta": ">r1\nACXG\n",
		"motifs": [{"name": "m", "sequence": "ACGT"}],
		"circular_ids": ["ghost"],
		"strand": "sideways"
	}`
	rec, parsed := postJSON(t, body)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d", rec.Code)
	}
	issues := parsed["issues"].([]any)
	if len(issues) < 3 {
		t.Fatalf("expected all issues, got %v", parsed)
	}
}

func TestSearchEndpointPagination(t *testing.T) {
	body := `{
		"fasta": ">r1\nAAAA\n",
		"motifs": [{"name": "aa", "sequence": "AA"}],
		"strand": "forward",
		"page": 2,
		"page_size": 2
	}`
	rec, parsed := postJSON(t, body)
	if rec.Code != 200 {
		t.Fatalf("body=%s", rec.Body.String())
	}
	if int(parsed["total"].(float64)) != 3 || len(parsed["hits"].([]any)) != 1 {
		t.Fatalf("got %v", parsed)
	}
}

func TestSearchEndpointBodyTooLarge(t *testing.T) {
	big := bytes.Repeat([]byte("A"), MaxBodyBytes+1)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/search", bytes.NewReader(big))
	rec := httptest.NewRecorder()
	NewRouter().ServeHTTP(rec, req)
	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("status=%d", rec.Code)
	}
}

func TestHealthz(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()
	NewRouter().ServeHTTP(rec, req)
	if rec.Code != 200 {
		t.Fatalf("status=%d", rec.Code)
	}
}
