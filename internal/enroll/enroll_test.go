package enroll

import "testing"

func TestFieldsLoaded(t *testing.T) {
	if len(Fields) < 190 {
		t.Fatalf("expected ~198 fields, got %d", len(Fields))
	}
}

func TestGenerate_Profiles(t *testing.T) {
	b := Generate("balanced", "telemetry.example.com", 8443)
	if b.Hostname != "telemetry.example.com" || b.Port != 8443 || !b.PreferTyped {
		t.Errorf("ftc header wrong: %+v", b)
	}
	// Drive fields are realtime (1s) in balanced.
	if loc, ok := b.Fields["Location"]; !ok || loc["interval_seconds"] != 1 {
		t.Errorf("Location should be 1s in balanced, got %v", b.Fields["Location"])
	}
	// energy responsive → resend present.
	if soc, ok := b.Fields["Soc"]; !ok || soc["interval_seconds"] != 15 || soc["resend_interval_seconds"] == 0 {
		t.Errorf("Soc should be responsive w/ resend, got %v", b.Fields["Soc"])
	}
	// eco drops diagnostic + media.
	eco := Generate("eco", "x", 443)
	if _, ok := eco.Fields["DiStatorTempF"]; ok {
		t.Errorf("eco should not enroll diagnostic DiStatorTempF")
	}
	if len(eco.Fields) >= len(b.Fields) {
		t.Errorf("eco (%d) should enroll fewer than balanced (%d)", len(eco.Fields), len(b.Fields))
	}
	// live enrolls the most.
	live := Generate("live", "x", 443)
	if len(live.Fields) < len(b.Fields) {
		t.Errorf("live (%d) should enroll >= balanced (%d)", len(live.Fields), len(b.Fields))
	}
}

func TestEstimateCost(t *testing.T) {
	e := EstimateCost(Generate("balanced", "x", 443))
	if e.EnrolledFields == 0 || e.SignalsHigh < e.SignalsLow || e.USDHigh <= 0 {
		t.Errorf("implausible estimate: %+v", e)
	}
}
