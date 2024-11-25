package utils

var limit int64 = 1e18

func abs(a int64) int64 {
	if a < 0 {
		return -a
	}
	return a
}

func SmallerThan(a, b int64) bool {
	if abs(a-b) > 1e10 {
		return a > b
	}
	return a < b
}

func Max(a, b int64) int64 {
	if abs(a-b) > 1e10 {
		if a > b {
			return b
		}
		return a
	}
	if a > b {
		return a
	}
	return b
}

func Inc(a int64) int64 {
	return (a + 1) % limit
}
