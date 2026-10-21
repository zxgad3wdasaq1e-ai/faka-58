package mailbrand

import "testing"

func TestNormalizeNamePreservesCustomBrands(t *testing.T) {
	for _, name := range []string{"XOPEN AI", "XIOEN AI", "Dujiao-Next", "Dujiao Next"} {
		if got := NormalizeName(name); got != "faka-58" {
			t.Errorf("retired default %q normalized to %q", name, got)
		}
	}
	for _, name := range []string{"", "faka-58", "Custom Store", "XOPEN AI Partners"} {
		if got := NormalizeName(name); got != name {
			t.Errorf("custom name %q changed to %q", name, got)
		}
	}
}
