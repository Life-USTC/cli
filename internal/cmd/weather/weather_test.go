package weather

import "testing"

func TestValidateLocationKeyUsesCanonicalValues(t *testing.T) {
	for _, value := range []string{"ustc-main", "ustc-gaoxin"} {
		if err := validateLocationKey(value); err != nil {
			t.Errorf("validateLocationKey(%q) = %v", value, err)
		}
	}
	if err := validateLocationKey("main"); err == nil {
		t.Fatal("validateLocationKey accepted non-canonical location")
	}
}
