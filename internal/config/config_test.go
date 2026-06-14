package config

import "testing"

func TestApplyEnv_VINsParsing(t *testing.T) {
	t.Setenv("TGW_VINS", " VIN1 , VIN2 ,, VIN3 ")
	t.Setenv("TGW_TEMPLATE_DIR", "/tmpl/")

	c := Defaults()
	applyEnv(&c)

	if len(c.Vehicles) != 3 {
		t.Fatalf("got %d vehicles, want 3: %+v", len(c.Vehicles), c.Vehicles)
	}
	wantVINs := []string{"VIN1", "VIN2", "VIN3"}
	for i, w := range wantVINs {
		if c.Vehicles[i].VIN != w {
			t.Errorf("vehicle[%d].VIN = %q, want %q", i, c.Vehicles[i].VIN, w)
		}
		if want := "/tmpl/" + w + ".json"; c.Vehicles[i].Template != want {
			t.Errorf("vehicle[%d].Template = %q, want %q", i, c.Vehicles[i].Template, want)
		}
	}
}

func TestDeriveID_Stable(t *testing.T) {
	vin := "5YJ3TESTVIN000001"
	a := deriveID(vin)
	b := deriveID(vin)
	if a != b {
		t.Errorf("deriveID not stable: %d != %d", a, b)
	}
	if a <= 0 {
		t.Errorf("deriveID = %d, want positive", a)
	}
	if other := deriveID("DIFFERENTVIN00001"); other == a {
		t.Errorf("deriveID collision for distinct VINs: %d", a)
	}
}

func TestVehicle_IDString(t *testing.T) {
	v := Vehicle{ID: 123456}
	if got := v.IDString(); got != "123456" {
		t.Errorf("IDString = %q, want 123456", got)
	}
}

func TestVehiclesByKey(t *testing.T) {
	cfg := &Config{Vehicles: []Vehicle{{VIN: "VINA", ID: 11, VehicleID: 22}}}
	m := VehiclesByKey(cfg)
	for _, key := range []string{"VINA", "11", "22"} {
		if _, ok := m[key]; !ok {
			t.Errorf("byKey missing key %q", key)
		}
	}
}
