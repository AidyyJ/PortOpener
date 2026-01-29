package daemon

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	"github.com/AidyyJ/PortOpener/cli/internal/config"
)

func PIDPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return "portopener.pid"
	}
	return filepath.Join(home, ".portopener", "daemon.pid")
}

func Start(configPath string) (int, error) {
	exe, err := os.Executable()
	if err != nil {
		return 0, err
	}
	if strings.TrimSpace(configPath) == "" {
		configPath = config.DefaultPath()
	}
	cmd := exec.Command(exe, "start", "--config", configPath)
	cmd.Stdout = nil
	cmd.Stderr = nil
	setDetached(cmd)
	if err := cmd.Start(); err != nil {
		return 0, err
	}
	if err := writePID(cmd.Process.Pid); err != nil {
		return cmd.Process.Pid, err
	}
	return cmd.Process.Pid, nil
}

func Stop() error {
	pid, err := readPID()
	if err != nil {
		return err
	}
	process, err := os.FindProcess(pid)
	if err != nil {
		return err
	}
	if err := stopProcess(process); err != nil {
		return err
	}
	_ = os.Remove(PIDPath())
	return nil
}

func Status() (bool, string, error) {
	pid, err := readPID()
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return false, "not running", nil
		}
		return false, "", err
	}
	process, err := os.FindProcess(pid)
	if err != nil {
		return false, "not running", nil
	}
	if runtime.GOOS == "windows" {
		return true, fmt.Sprintf("pid %d (unverified)", pid), nil
	}
	if err := probeProcess(process); err != nil {
		return false, "not running", nil
	}
	return true, fmt.Sprintf("pid %d", pid), nil
}

func writePID(pid int) error {
	path := PIDPath()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(strconv.Itoa(pid)), 0o600)
}

func readPID() (int, error) {
	contents, err := os.ReadFile(PIDPath())
	if err != nil {
		return 0, err
	}
	trimmed := strings.TrimSpace(string(contents))
	if trimmed == "" {
		return 0, fmt.Errorf("pid file empty")
	}
	return strconv.Atoi(trimmed)
}
