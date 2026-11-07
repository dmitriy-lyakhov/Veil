//go:build windows

package netsys

import (
	"os/exec"
	"syscall"

	"golang.org/x/sys/windows"
)

const cpOEM = 1 // CP_OEMCP

func HideWindow(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: windows.CREATE_NO_WINDOW}
}

// decodeOutput переводит вывод консольных утилит из OEM-кодировки (cp866 и т.п.) в UTF-8.
func DecodeOutput(b []byte) string {
	if len(b) == 0 {
		return ""
	}
	n, err := windows.MultiByteToWideChar(cpOEM, 0, &b[0], int32(len(b)), nil, 0)
	if err != nil || n == 0 {
		return string(b)
	}
	w := make([]uint16, n)
	if _, err := windows.MultiByteToWideChar(cpOEM, 0, &b[0], int32(len(b)), &w[0], n); err != nil {
		return string(b)
	}
	return windows.UTF16ToString(w)
}
