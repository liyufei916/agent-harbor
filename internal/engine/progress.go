package engine

// Progress is a UI-safe install/start progress snapshot (0–100).
type Progress struct {
	Percent int    `json:"percent"`
	Label   string `json:"label"`
}

// ProgressFunc receives coarse progress during Ensure/Start/EnsureImage.
type ProgressFunc func(Progress)

func clampPercent(p int) int {
	if p < 0 {
		return 0
	}
	if p > 100 {
		return 100
	}
	return p
}
