//go:build windows

package daemon

import (
	"os"
	"os/exec"
	"syscall"
)

func setDetached(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP}
}

func stopProcess(process *os.Process) error {
	return process.Kill()
}

func probeProcess(process *os.Process) error {
	// Windows does not support signal 0; treat as unknown
	return nil
}
