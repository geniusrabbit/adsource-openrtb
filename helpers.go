package adsourceopenrtb

func b2i(b bool) int {
	if b {
		return 1
	}
	return 0
}

func intRef(v int) *int {
	return &v
}
