package gui

import (
	"fmt"
	"time"
)

func timeNow() time.Time { return time.Now() }

func fmtMB(v float64) string { return fmt.Sprintf("%.1f", v) }
