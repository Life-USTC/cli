package youngutil

import (
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/Life-USTC/CLI/internal/api"
)

func TestDateRangeUsesShanghaiCalendarBoundaries(t *testing.T) {
	now := time.Date(2026, time.September, 16, 23, 30, 0, 0, time.FixedZone("UTC", 0))
	tests := []struct {
		view     DateView
		from, to string
	}{
		{DateDay, "2026-09-17", "2026-09-17"},
		{DateWeek, "2026-09-14", "2026-09-20"},
		{DateMonth, "2026-09-01", "2026-09-30"},
	}
	for _, tt := range tests {
		from, to, err := DateRange(tt.view, "", now)
		if err != nil {
			t.Fatalf("DateRange(%q): %v", tt.view, err)
		}
		if from != tt.from || to != tt.to {
			t.Errorf("DateRange(%q) = %s..%s, want %s..%s", tt.view, from, to, tt.from, tt.to)
		}
	}
}

func TestDateRangeRejectsInvalidAnchor(t *testing.T) {
	if _, _, err := DateRange(DateDay, "2026-02-30", time.Now()); err == nil {
		t.Fatal("DateRange accepted an invalid anchor")
	}
}

func TestFetchAllPagesTraversesCompleteResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("pageSize") != "2" {
			t.Errorf("pageSize = %q", r.URL.Query().Get("pageSize"))
		}
		page := r.URL.Query().Get("page")
		w.Header().Set("Content-Type", "application/json")
		if page == "1" {
			_, _ = io.WriteString(w, `{"data":[{"youngId":"a"},{"youngId":"b"}],"pagination":{"page":1,"pageSize":2,"total":3,"totalPages":2}}`)
			return
		}
		if page == "2" {
			_, _ = io.WriteString(w, `{"data":[{"youngId":"c"}],"pagination":{"page":2,"pageSize":2,"total":3,"totalPages":2}}`)
			return
		}
		http.Error(w, "unexpected page", http.StatusBadRequest)
	}))
	defer server.Close()
	client, err := api.NewClient(server.URL, false)
	if err != nil {
		t.Fatal(err)
	}
	data, err := FetchAllPages(t.Context(), client, "/events", url.Values{"dateFrom": []string{"2026-09-01"}}, "data", 2, "youngId")
	if err != nil {
		t.Fatal(err)
	}
	rows := data.(map[string]any)["data"].([]any)
	if len(rows) != 3 {
		t.Fatalf("rows = %#v", rows)
	}
	pagination := data.(map[string]any)["pagination"].(map[string]any)
	if pagination["totalPages"] != 1 {
		t.Fatalf("pagination = %#v", pagination)
	}
}

func TestFetchAllPagesRejectsIncompleteResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"data":[{"youngId":"a"}],"pagination":{"page":1,"pageSize":2,"total":3,"totalPages":2}}`)
	}))
	defer server.Close()
	client, err := api.NewClient(server.URL, false)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := FetchAllPages(t.Context(), client, "/events", nil, "data", 2, "youngId"); err == nil {
		t.Fatal("FetchAllPages accepted an incomplete response")
	}
}
