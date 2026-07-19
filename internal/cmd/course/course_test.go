package course

import "testing"

func TestCourseListUsesViewIdentifier(t *testing.T) {
	columns := courseListColumns()
	last := columns[len(columns)-1]
	if last.Header != "JW ID" || last.Key != "jwId" {
		t.Fatalf("last column = %#v, want JW ID backed by jwId", last)
	}
}
