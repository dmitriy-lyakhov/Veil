package helper

import (
	"errors"
	"os"
	"os/exec"
)

func elevate(args []string, logFile string) error {
	f, err := os.OpenFile(logFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return err
	}
	defer f.Close()
	var cmd *exec.Cmd
	if os.Geteuid() == 0 {
		cmd = exec.Command(args[0], args[1:]...)
	} else {
		if _, err := exec.LookPath("pkexec"); err != nil {
			return errors.New("не найден pkexec — запустите «sudo veil connect <профиль>» вручную")
		}
		cmd = exec.Command("pkexec", args...)
	}
	cmd.Stdout, cmd.Stderr = f, f
	if err := cmd.Start(); err != nil {
		return err
	}
	go cmd.Wait()
	return nil
}
