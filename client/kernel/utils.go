package kernel

import "time"

func wait(ackch chan error, execFunc func() error) bool {
	ServiceConfInitTimeout := 5 * time.Second
	go func() {
		ackch <- execFunc()
	}()
	select {
	case <-time.After(ServiceConfInitTimeout):
		return false
	case err := <-ackch:
		if err != nil {
			return false
		}
		return true
	}
}

var (
	limit  int32 = 1e8
	maxGap int32 = 1e7
)

func abs(a int32) int32 {
	if a < 0 {
		return -a
	}
	return a
}

func SmallerThan(a, b int32) bool {
	if abs(a-b) > maxGap {
		return a > b
	}
	return a < b
}

func Max(a, b int32) int32 {
	if abs(a-b) > maxGap {
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

func Inc(a int32) int32 {
	return (a + 1) % limit
}
