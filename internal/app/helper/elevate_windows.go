//go:build windows

package helper

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"syscall"

	"golang.org/x/sys/windows"
)

func isAdmin() bool { return windows.GetCurrentProcessToken().IsElevated() }

func elevate(args []string, logFile string) error {
	if isAdmin() {
		f, err := os.OpenFile(logFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
		if err != nil {
			return err
		}
		defer f.Close()
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Stdout, cmd.Stderr = f, f
		cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: windows.CREATE_NO_WINDOW}
		if err := cmd.Start(); err != nil {
			return err
		}
		go cmd.Wait()
		return nil
	}

	// Запрос UAC. Вывод помощника пишем в журнал через cmd: ShellExecute не умеет
	// перенаправлять потоки, а у процесса без консоли своего stderr нет.
	quoted := make([]string, len(args))
	for i, a := range args {
		quoted[i] = `"` + a + `"`
	}
	params := fmt.Sprintf(`/c "%s >> "%s" 2>&1"`, strings.Join(quoted, " "), logFile)
	verb, _ := windows.UTF16PtrFromString("runas")
	file, _ := windows.UTF16PtrFromString("cmd.exe")
	p, _ := windows.UTF16PtrFromString(params)
	if err := windows.ShellExecute(0, verb, file, p, nil, windows.SW_HIDE); err != nil {
		return fmt.Errorf("запрос прав администратора отклонён: %w", err)
	}
	return nil
}
