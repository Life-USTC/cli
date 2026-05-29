package homework

import "testing"

func TestFilterHomeworkRows_UsesCompletionWhenIsCompletedMissing(t *testing.T) {
	rows := []map[string]any{
		{"id": "done", "completion": map[string]any{"completedAt": "2025-06-01T00:00:00Z"}},
		{"id": "pending", "completion": nil},
	}

	cases := []struct {
		name string
		opts myHomeworkListOpts
		want string
	}{
		{name: "done", opts: myHomeworkListOpts{done: true}, want: "done"},
		{name: "pending", opts: myHomeworkListOpts{pending: true}, want: "pending"},
	}

	for _, tc := range cases {
		got, err := filterHomeworkRows(rows, tc.opts)
		if err != nil {
			t.Fatalf("%s: unexpected error: %v", tc.name, err)
		}
		if len(got) != 1 || got[0]["id"] != tc.want {
			t.Fatalf("%s: got %#v, want only %q", tc.name, got, tc.want)
		}
	}
}
