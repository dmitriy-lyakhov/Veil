//go:build windows

package platform

import "unsafe"

func unsafePointer(p *uint16) unsafe.Pointer { return unsafe.Pointer(p) }
