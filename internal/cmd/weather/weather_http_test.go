package weather

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Life-USTC/CLI/internal/api"
	openapi "github.com/Life-USTC/CLI/internal/openapi"
)

func TestFetchWeatherUsesCanonicalLocationKey(t *testing.T) {
	t.Setenv("LIFE_USTC_CONFIG_DIR", t.TempDir())
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Query().Get("locationKey"); got != "ustc-gaoxin" {
			t.Errorf("locationKey = %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"location":{"key":"ustc-gaoxin","name":"Gaoxin"},"fetchedAt":"2026-01-01T00:00:00Z","providers":[],"current":{"temperature":1},"daily":[],"hourly":[],"alerts":[],"extensions":{}}`)
	}))
	defer server.Close()
	client, err := api.NewTypedClient(server.URL, false)
	if err != nil {
		t.Fatal(err)
	}
	key := openapi.CatalogWeatherGetParamsLocationKey("ustc-gaoxin")
	data, err := fetchWeather(client, &openapi.CatalogWeatherGetParams{LocationKey: &key})
	if err != nil {
		t.Fatal(err)
	}
	if got := data.(map[string]any)["location"].(map[string]any)["key"]; got != "ustc-gaoxin" {
		t.Fatalf("location key = %#v", got)
	}
}
