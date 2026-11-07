package helper

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

func elevate(args []string, logFile string) error {
	quoted := make([]string, len(args))
	for i, a := range args {
		quoted[i] = shellQuote(a)
	}
	shell := strings.Join(quoted, " ") + " >> " + shellQuote(logFile) + " 2>&1 &"
	if os.Geteuid() == 0 {
		return exec.Command("/bin/sh", "-c", shell).Run()
	}
	script := fmt.Sprintf(`do shell script %s with prompt "Veil нужно разрешение на создание VPN-туннеля." with administrator privileges`, appleScriptString(shell))
	out, err := exec.Command("osascript", "-e", script).CombinedOutput()
	if err != nil {
		return fmt.Errorf("запрос прав администратора: %v: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

func appleScriptString(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `"`, `\"`)
	return `"` + s + `"`
}
