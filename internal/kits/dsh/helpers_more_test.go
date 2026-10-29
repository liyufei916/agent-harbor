package dsh

import "testing"

func TestDSHValidationHelpers(t *testing.T) {
	for _, raw := range []string{"http://127.0.0.1:1", "http://localhost:1", "https://127.0.0.1:1", "http://127.0.0.1:01"} {
		if gateURL(raw) != (raw == "http://127.0.0.1:1") {
			t.Errorf("gateURL(%q)", raw)
		}
	}
	if _, err := New("", "", ""); err == nil {
		t.Fatal("empty model accepted")
	}
	if _, err := New("", "", "model"); err != nil {
		t.Fatal(err)
	}
}
