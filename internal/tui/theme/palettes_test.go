package theme

import (
	"math"
	"strconv"
	"testing"
)

func TestPaletteTextContrast(t *testing.T) {
	for _, name := range []string{"owncode", "midnight", "ember", "grove"} {
		t.Run(name, func(t *testing.T) {
			p := GetTheme(name)
			for _, light := range []bool{false, true} {
				color := func(c AdaptiveColor) string {
					if light {
						return c.Light
					}
					return c.Dark
				}
				for _, bg := range []AdaptiveColor{p.Background(), p.BackgroundSecondary(), p.BackgroundDarker()} {
					for _, fg := range []AdaptiveColor{p.Text(), p.TextMuted(), p.Primary(), p.Secondary(), p.Accent(), p.Success(), p.Warning(), p.Error()} {
						a, b := luminance(t, color(fg)), luminance(t, color(bg))
						ratio := (max(a, b) + 0.05) / (min(a, b) + 0.05)
						if ratio < 4.5 {
							t.Errorf("light=%v %s on %s: contrast %.2f < 4.5", light, color(fg), color(bg), ratio)
						}
					}
				}
			}
		})
	}
}

func luminance(t *testing.T, hex string) float64 {
	t.Helper()
	n, err := strconv.ParseUint(hex[1:], 16, 32)
	if err != nil {
		t.Fatal(err)
	}
	linear := func(n uint64) float64 {
		c := float64(n) / 255
		if c <= 0.04045 {
			return c / 12.92
		}
		return math.Pow((c+0.055)/1.055, 2.4)
	}
	return 0.2126*linear((n>>16)&255) + 0.7152*linear((n>>8)&255) + 0.0722*linear(n&255)
}
