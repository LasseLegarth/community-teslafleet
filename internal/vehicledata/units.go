package vehicledata

import (
	"math"
	"strings"
)

const kmToMi = 0.621371

// RangeToMiles converts a distance/range value (in the configured input unit) to
// miles, which is what Tesla vehicle_data and the legacy stream always report.
func RangeToMiles(v float64, input string) float64 {
	if strings.EqualFold(input, "mi") {
		return v
	}
	return v * kmToMi
}

// SpeedToMph converts a speed value (in the configured input unit) to mph.
func SpeedToMph(v float64, input string) float64 {
	if strings.EqualFold(input, "mph") {
		return v
	}
	return v * kmToMi
}

const miToKm = 1.609344

// SpeedToKmh converts a speed (in the configured input unit) to km/h (for HA display).
func SpeedToKmh(v float64, input string) float64 {
	if strings.EqualFold(input, "mph") {
		return v * miToKm
	}
	return v
}

// RangeToKm converts a distance/range (in the configured input unit) to km (for HA display).
func RangeToKm(v float64, input string) float64 {
	if strings.EqualFold(input, "mi") {
		return v * miToKm
	}
	return v
}

func round1(v float64) float64 {
	return math.Round(v*10) / 10
}

// Display-unit system for Home Assistant output.
const (
	SystemMetric   = "metric"
	SystemImperial = "imperial"

	barToPsi = 14.503773773
)

// imperialSystem reports whether the configured system is imperial (metric otherwise).
func imperialSystem(system string) bool { return strings.EqualFold(system, SystemImperial) }

// DistanceUnit/SpeedUnit/TempUnit/PressureUnit return the HA unit label for a system.
func DistanceUnit(system string) string {
	if imperialSystem(system) {
		return "mi"
	}
	return "km"
}

func SpeedUnit(system string) string {
	if imperialSystem(system) {
		return "mph"
	}
	return "km/h"
}

func TempUnit(system string) string {
	if imperialSystem(system) {
		return "°F"
	}
	return "°C"
}

func PressureUnit(system string) string {
	if imperialSystem(system) {
		return "psi"
	}
	return "bar"
}

// DistanceForDisplay converts a distance in `input` unit (mi|km) to the system's unit.
func DistanceForDisplay(v float64, input, system string) float64 {
	miles := RangeToMiles(v, input)
	if imperialSystem(system) {
		return miles
	}
	return miles * miToKm
}

// SpeedForDisplay converts a speed in `input` unit (mph|kmh) to the system's unit.
func SpeedForDisplay(v float64, input, system string) float64 {
	mph := SpeedToMph(v, input)
	if imperialSystem(system) {
		return mph
	}
	return mph * miToKm
}

// TempForDisplay converts a temperature in °C (the stream's unit) to the system's unit.
func TempForDisplay(celsius float64, system string) float64 {
	if imperialSystem(system) {
		return celsius*9/5 + 32
	}
	return celsius
}

// PressureForDisplay converts a pressure in bar (the stream's unit) to the system's unit.
func PressureForDisplay(bar float64, system string) float64 {
	if imperialSystem(system) {
		return bar * barToPsi
	}
	return bar
}
