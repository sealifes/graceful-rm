//go:build !windows

package graceful

import "os"

func currentUID() int { return os.Getuid() }
func currentGID() int { return os.Getgid() }
