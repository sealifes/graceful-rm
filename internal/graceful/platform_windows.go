//go:build windows

package graceful

func currentUID() int { return 1 }
func currentGID() int { return 1 }
