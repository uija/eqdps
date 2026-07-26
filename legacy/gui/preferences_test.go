package main

import "testing"

func TestPreferenceSliderMappingRoundTrip(t *testing.T) {
	for _, value := range []float32{.75, 1, 1.25, 1.5} {
		slider := settingToSlider(value, .75, 1.5)
		got := sliderToSetting(slider, .75, 1.5)
		if difference := got - value; difference < -.00001 || difference > .00001 {
			t.Fatalf("round trip for %v produced %v", value, got)
		}
	}
}
