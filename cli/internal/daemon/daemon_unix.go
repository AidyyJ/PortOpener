//go:build !windows

package daemon

import (
	"os"
	"os/exec"
	"syscall"
)

func setDetached(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
}

func stopProcess(process *os.Process) error {
	return process.Signal(syscall.SIGTERM)
}

func probeProcess(process *os.Process) error {
	return process.Signal(syscall.Signal(0))
}
