//go:build windows

package singbox

import (
	"os"
	"os/exec"
)

func setSysProcAttr(cmd *exec.Cmd) {
}

func sendSigterm(pid int) error {
	proc, err := os.FindProcess(pid)
	if err != nil {
		return err
	}
	return proc.Signal(os.Interrupt)
}

func sendSigkill(pid int) error {
	proc, err := os.FindProcess(pid)
	if err != nil {
		return err
	}
	return proc.Kill()
}
