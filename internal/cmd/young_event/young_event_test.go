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
	if *params.Active != "true" || *params.Category != "系列项目" || *params.Search != "robotics" || *params.Page != 2 || *params.PageSize != 20 {
		t.Fatalf("params = %#v", params)
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
