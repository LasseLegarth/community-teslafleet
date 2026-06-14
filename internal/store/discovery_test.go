package store

import "testing"

// With no configured VINs (empty allow-list) the store auto-registers any VIN it
// sees on the stream — telemetry is the source of truth for which cars exist.
func TestStore_AutoRegister(t *testing.T) {
	s := New() // zero-config

	s.SetField("VIN_A", "Soc", 80.0)
	s.SetConnectivity("VIN_B", "online")

	if _, ok := s.Snapshot("VIN_A"); !ok {
		t.Errorf("VIN_A should be auto-registered after SetField")
	}
	if _, ok := s.Snapshot("VIN_B"); !ok {
		t.Errorf("VIN_B should be auto-registered after SetConnectivity")
	}
	if got := len(s.VINs()); got != 2 {
		t.Errorf("VINs() = %d, want 2", got)
	}
	// Empty VIN is never registered.
	s.SetField("", "Soc", 1.0)
	if got := len(s.VINs()); got != 2 {
		t.Errorf("empty VIN must not register: VINs() = %d, want 2", got)
	}
}

// A non-empty allow-list restricts the store to the listed VINs; everything else
// on the stream is ignored.
func TestStore_AllowListFilters(t *testing.T) {
	s := New("VIN_A")

	s.SetField("VIN_A", "Soc", 80.0)
	s.SetField("VIN_X", "Soc", 50.0) // not allowed → dropped

	if _, ok := s.Snapshot("VIN_A"); !ok {
		t.Errorf("allow-listed VIN_A should be present")
	}
	if snap, ok := s.Snapshot("VIN_X"); ok {
		t.Errorf("non-allowed VIN_X should be ignored, got %+v", snap)
	}
	if got := len(s.VINs()); got != 1 {
		t.Errorf("VINs() = %d, want 1 (only allow-listed)", got)
	}
}
