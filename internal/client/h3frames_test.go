package client

import (
	"testing"

	"veil/internal/config"
)

// Построитель кадров пресета Chrome не должен паниковать на пустых и коротких данных CRYPTO (probe-пакеты).
func TestFrameBuilderNoPanic(t *testing.T) {
	spec, err := chromeQUICSpec(config.Target{})
	if err != nil {
		t.Fatal(err)
	}
	b := spec.InitialPacketSpec.FrameBuilder
	if _, ok := b.(safeFrames); !ok {
		t.Fatalf("построитель кадров не обёрнут: %T", b)
	}
	for _, n := range []int{0, 1, 2, 3, 5, 40, 300, 1200} {
		for i := 0; i < 200; i++ {
			out, err := b.Build(make([]byte, n))
			if err != nil {
				t.Fatalf("n=%d: %v", n, err)
			}
			if len(out) == 0 {
				t.Fatalf("n=%d: пустой результат", n)
			}
		}
	}
}
