//go:build windows

package gui

import "unsafe"

func unsafePointer(p *uint16) unsafe.Pointer { return unsafe.Pointer(p) }
