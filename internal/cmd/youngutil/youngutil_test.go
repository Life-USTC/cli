package youngutil

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"strings"
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

func TestRequireIDRejectsWhitespace(t *testing.T) {
	if _, err := RequireID("  \t", "<young-id>"); err == nil {
		t.Fatal("RequireID accepted whitespace-only input")
	}
	if got, err := RequireID(" event-1 ", "<young-id>"); err != nil || got != "event-1" {
		t.Fatalf("RequireID = %q, %v; want event-1", got, err)
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
	data, err := FetchAllPages(t.Context(), func(ctx context.Context, params url.Values) (any, error) {
		resp, err := client.DoRaw(ctx, http.MethodGet, "/events", params, nil, "", nil)
		body, ct, err := api.ReadResponse(resp, err)
		if err != nil {
			return nil, err
		}
		return api.DecodeResponseBody(body, ct, false)
	}, "/events", url.Values{"dateFrom": []string{"2026-09-01"}}, "data", 2, "youngId")
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
	if _, err := FetchAllPages(t.Context(), func(ctx context.Context, params url.Values) (any, error) {
		resp, err := client.DoRaw(ctx, http.MethodGet, "/events", params, nil, "", nil)
		body, ct, err := api.ReadResponse(resp, err)
		if err != nil {
			return nil, err
		}
		return api.DecodeResponseBody(body, ct, false)
	}, "/events", nil, "data", 2, "youngId"); err == nil {
		t.Fatal("FetchAllPages accepted an incomplete response")
	}
}

func TestFetchAllPagesPreservesPaginationPolicies(t *testing.T) {
	page := func(total, pages int, ids ...string) any {
		rows := make([]any, len(ids))
		for i, id := range ids {
			rows[i] = map[string]any{"youngId": id}
		}
		return map[string]any{"data": rows, "pagination": map[string]any{"total": total, "totalPages": pages}}
	}
	tests := []struct {
		name      string
		responses []any
		wantPages []string
		wantCount int
		wantError string
	}{
		{"empty zero pages", []any{page(0, 0)}, []string{"1"}, 0, ""},
		{"empty one page", []any{page(0, 1)}, []string{"1"}, 0, ""},
		{"empty middle page", []any{page(3, 3, "a"), page(3, 3), page(3, 3, "b", "c")}, []string{"1", "2", "3"}, 3, ""},
		{"empty total with rows", []any{page(0, 1, "a")}, []string{"1"}, 0, "inconsistent empty pagination"},
		{"empty total with more pages", []any{page(0, 2)}, []string{"1"}, 0, "inconsistent empty pagination"},
		{"nonempty total without pages", []any{page(1, 0)}, []string{"1"}, 0, "returned no pages"},
		{"page count limit", []any{page(1, 10001)}, []string{"1"}, 0, "invalid page count"},
		{"changed total", []any{page(3, 2, "a"), page(4, 2, "b")}, []string{"1", "2"}, 0, "changed pagination"},
		{"changed page count", []any{page(3, 2, "a"), page(3, 3, "b")}, []string{"1", "2"}, 0, "changed pagination"},
		{"duplicate first page", []any{page(3, 2, "a", "a")}, []string{"1"}, 0, "duplicate youngId"},
		{"duplicate later page", []any{page(3, 2, "a"), page(3, 2, "a", "b")}, []string{"1", "2"}, 0, "duplicate youngId"},
		{"empty ID", []any{page(1, 1, "")}, []string{"1"}, 0, "row without youngId"},
		{"oversized later page", []any{page(4, 2, "a"), page(4, 2, "b", "c", "d")}, []string{"1", "2"}, 0, "oversized page"},
		{"incomplete total", []any{page(3, 2, "a"), page(3, 2, "b")}, []string{"1", "2"}, 0, "returned 2 of 3 results"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var requested []string
			params := url.Values{"module": {"智"}, "active": {"false"}}
			data, err := FetchAllPages(t.Context(), func(_ context.Context, query url.Values) (any, error) {
				requested = append(requested, query.Get("page"))
				if query.Get("pageSize") != "2" || query.Get("module") != "智" || query.Get("active") != "false" {
					t.Fatalf("query = %v", query)
				}
				if len(requested) > len(tt.responses) {
					t.Fatalf("unexpected extra page: %v", requested)
				}
				return tt.responses[len(requested)-1], nil
			}, "/events", params, "data", 2, "youngId")
			if !reflect.DeepEqual(requested, tt.wantPages) {
				t.Fatalf("requested = %v, want %v", requested, tt.wantPages)
			}
			if params.Has("page") || params.Has("pageSize") {
				t.Fatalf("caller parameters changed: %v", params)
			}
			if tt.wantError != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantError) || data != nil {
					t.Fatalf("data = %#v, err = %v; want no partial data and %q", data, err, tt.wantError)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			result := data.(map[string]any)
			if len(result["data"].([]any)) != tt.wantCount {
				t.Fatalf("data = %#v", result)
			}
			pagination := result["pagination"].(map[string]any)
			if pagination["page"] != 1 || pagination["totalPages"] != 1 || pagination["total"] != tt.wantCount {
				t.Fatalf("pagination = %#v", pagination)
			}
		})
	}
}

func TestFetchAllIfUnpagedKeepsExplicitSinglePage(t *testing.T) {
	for _, params := range []url.Values{{"page": {"2"}}, {"pageSize": {"1"}}} {
		calls := 0
		payload := map[string]any{"data": []any{}, "pagination": map[string]any{"page": 2, "total": 9, "totalPages": 9}}
		data, err := FetchAllIfUnpaged(t.Context(), func(_ context.Context, query url.Values) (any, error) {
			calls++
			if !reflect.DeepEqual(query, params) {
				t.Fatalf("query = %v, want %v", query, params)
			}
			return payload, nil
		}, "/events", params, "data", 2, "youngId")
		if err != nil || calls != 1 || !reflect.DeepEqual(data, payload) {
			t.Fatalf("calls = %d, data = %#v, err = %v", calls, data, err)
		}
	}
}
