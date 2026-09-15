package young_event

import (
	"testing"

	"github.com/spf13/cobra"
)

func TestYoungEventFlagsMapToCanonicalPageSize(t *testing.T) {
	var opts listOpts
	cmd := &cobra.Command{}
	addListFlags(cmd, &opts)
	if err := cmd.ParseFlags([]string{"--active", "true", "--category", "系列项目", "--search", "robotics", "--page", "2", "--limit", "20"}); err != nil {
		t.Fatal(err)
	}
	params, err := buildListParams(opts)
	if err != nil {
		t.Fatal(err)
	}
	for key, want := range map[string]string{
		"active": "true", "category": "系列项目", "search": "robotics", "page": "2", "pageSize": "20",
	} {
		if got := params.Get(key); got != want {
			t.Errorf("params[%s] = %q, want %q", key, got, want)
		}
	}
	if len(params) != 5 {
		t.Fatalf("params = %#v", params)
	}
}

func TestYoungEventFlagsMapNewFilters(t *testing.T) {
	params, err := buildListParams(listOpts{
		organizerID: "org-1",
		dateFrom:    "2026-09-01",
		dateTo:      "2026-09-30",
		timeBasis:   "registration",
	})
	if err != nil {
		t.Fatal(err)
	}
	for key, want := range map[string]string{
		"organizerId": "org-1", "dateFrom": "2026-09-01", "dateTo": "2026-09-30", "timeBasis": "registration",
	} {
		if got := params.Get(key); got != want {
			t.Errorf("params[%s] = %q, want %q", key, got, want)
		}
	}
}

func TestNormalizeActiveRejectsUnknownValue(t *testing.T) {
	if _, err := normalizeActive("yes"); err == nil {
		t.Fatal("normalizeActive accepted unknown value")
	}
}

func TestBuildListParamsRejectsNegativePagination(t *testing.T) {
	if _, err := buildListParams(listOpts{page: -1}); err == nil {
		t.Fatal("buildListParams accepted a negative page")
	}
	if _, err := buildListParams(listOpts{pageSize: -1}); err == nil {
		t.Fatal("buildListParams accepted a negative limit")
	}
}

func TestBuildListParamsRequiresDatePair(t *testing.T) {
	if _, err := buildListParams(listOpts{dateFrom: "2026-09-01"}); err == nil {
		t.Fatal("buildListParams accepted an unpaired date-from")
	}
}
