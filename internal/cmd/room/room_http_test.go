package room

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Life-USTC/CLI/internal/api"
)

func TestFetchRoomMapUsesRoomCodePath(t *testing.T) {
	t.Setenv("LIFE_USTC_CONFIG_DIR", t.TempDir())
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/api/catalog/rooms/B001/map" {
			t.Fatalf("request = %s %s", r.Method, r.URL)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"code":"B001","status":"highlighted","building":"Main","floor":"1","imageUrl":"https://example.test/map.png"}`)
	}))
	defer server.Close()
	client, err := api.NewTypedClient(server.URL, false)
	if err != nil {
		t.Fatal(err)
	}
	data, err := fetchRoomMap(client, "B001")
	if err != nil {
		t.Fatal(err)
	}
	if got := data.(map[string]any)["code"]; got != "B001" {
		t.Fatalf("room code = %#v", got)
	}
}
