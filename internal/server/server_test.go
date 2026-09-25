package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func doSearch(t *testing.T, body string) (int, map[string]any) {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/search", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	NewRouter().ServeHTTP(rec, req)
	var out map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &out)
	return rec.Code, out
}

const basicFasta = ">linear\\nATATCG\\n>circ\\nAAAT\\n"

func TestSearchDoubleStrandCrossOriginAndPaging(t *testing.T) {
	body := `{
		"fasta": "` + basicFasta + `",
		"patterns": [
			{"name": "ata", "pattern": "ata"},
			{"name": "wraptaa", "pattern": "ATAA"}
		],
		"circular_record_ids": ["circ"],
		"strand": "both"
	}`
	status, out := doSearch(t, body)
	if status != http.StatusOK {
		t.Fatalf("status %d body %v", status, out)
	}
	total := int(out["total"].(float64))
	if total < 1 {
		t.Fatal("expected hits")
	}
	hits := out["hits"].([]any)
	var cross map[string]any
	for _, h := range hits {
		m := h.(map[string]any)
		if m["record_id"] == "circ" && m["pattern_name"] == "wraptaa" {
			cross = m
		}
	}
	if cross == nil || cross["cross_origin"] != true {
		t.Fatalf("cross-origin hit missing: %v", out)
	}
	segs := cross["segments"].([]any)
	first := segs[0].(map[string]any)
	second := segs[1].(map[string]any)
	if first["start"].(float64) != 2 || first["end"].(float64) != 4 ||
		second["start"].(float64) != 0 || second["end"].(float64) != 2 {
		t.Fatalf("bad segments: %v", segs)
	}

	// 双链合并：ATA 反向互补为 TAT，线性 ATATCG 上起点 0(ATA)、1(TAT) 不合并；
	// 验证存在 both 链合并命中（环状 circ 上 ATAT 型回文场景另测）。
	// 此处验证分页稳定：page_size=1 翻页不重不漏。
	seen := map[string]bool{}
	for page := 1; page <= total; page++ {
		paged := strings.Replace(body, `"strand": "both"`, fmtPages(page), 1)
		st, po := doSearch(t, paged)
		if st != 200 {
			t.Fatalf("page %d status %d", page, st)
		}
		ph := po["hits"].([]any)
		if len(ph) != 1 {
			t.Fatalf("page %d expected 1 hit, got %d", page, len(ph))
		}
		key := jsonKey(ph[0])
		if seen[key] {
			t.Fatalf("duplicate hit across pages: %s", key)
		}
		seen[key] = true
	}
	if len(seen) != total {
		t.Fatalf("paging lost hits: %d vs %d", len(seen), total)
	}
}

func fmtPages(page int) string {
	return `"strand": "both", "page": ` + itoa(page) + `, "page_size": 1`
}

func itoa(i int) string {
	b, _ := json.Marshal(i)
	return string(b)
}

func jsonKey(v any) string {
	b, _ := json.Marshal(v)
	return string(b)
}

func TestSearchBothMergeExample(t *testing.T) {
	body := `{
		"fasta": ">r\nAT\n",
		"patterns": [{"name": "p", "pattern": "AT"}],
		"strand": "both"
	}`
	status, out := doSearch(t, body)
	if status != 200 {
		t.Fatalf("%v", out)
	}
	hits := out["hits"].([]any)
	if len(hits) != 1 || hits[0].(map[string]any)["strand"] != "both" {
		t.Fatalf("expected single merged both hit: %v", hits)
	}
}

func TestSearchValidationErrors(t *testing.T) {
	cases := []struct {
		name string
		body string
	}{
		{"illegal char", `{"fasta": ">r\nACXG\n", "patterns": [{"name":"p","pattern":"A"}]}`},
		{"duplicate record", `{"fasta": ">r\nAC\n>r\nGT\n", "patterns": [{"name":"p","pattern":"A"}]}`},
		{"empty record", `{"fasta": ">r\n", "patterns": [{"name":"p","pattern":"A"}]}`},
		{"unknown circular id", `{"fasta": ">r\nACGT\n", "patterns": [{"name":"p","pattern":"A"}], "circular_record_ids": ["zzz"]}`},
		{"dup pattern name", `{"fasta": ">r\nACGT\n", "patterns": [{"name":"p","pattern":"A"},{"name":"p","pattern":"C"}]}`},
		{"illegal pattern char", `{"fasta": ">r\nACGT\n", "patterns": [{"name":"p","pattern":"X"}]}`},
		{"bad strand", `{"fasta": ">r\nACGT\n", "patterns": [{"name":"p","pattern":"A"}], "strand":"sideways"}`},
		{"no patterns", `{"fasta": ">r\nACGT\n", "patterns": []}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			status, out := doSearch(t, tc.body)
			if status != http.StatusBadRequest {
				t.Fatalf("expected 400, got %d %v", status, out)
			}
			errs, ok := out["errors"].([]any)
			if !ok || len(errs) == 0 {
				t.Fatalf("expected error details, got %v", out)
			}
		})
	}
}

func TestBodyTooLarge(t *testing.T) {
	big := strings.Repeat("A", MaxBodyBytes+100)
	body := `{"fasta": ">r\n` + big + `\n", "patterns": [{"name":"p","pattern":"A"}]}`
	req := httptest.NewRequest(http.MethodPost, "/search", bytes.NewBufferString(body))
	rec := httptest.NewRecorder()
	NewRouter().ServeHTTP(rec, req)
	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("expected 413, got %d", rec.Code)
	}
}

func TestHealth(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()
	NewRouter().ServeHTTP(rec, req)
	if rec.Code != 200 {
		t.Fatalf("healthz status %d", rec.Code)
	}
}
