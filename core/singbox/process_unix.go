//go:build linux || darwin

package singbox

import (
	"os/exec"
	"syscall"
)

func setSysProcAttr(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setpgid: true,
	}
}

func sendSigterm(pid int) error {
	return syscall.Kill(-pid, syscall.SIGTERM)
}

func sendSigkill(pid int) error {
	return syscall.Kill(-pid, syscall.SIGKILL)
}

func sendSighup(pid int) error {
	return syscall.Kill(pid, syscall.SIGHUP)
}
