package benchmarks

import (
	"github.com/ZeroGCDev/zerotui/style"
	"testing"
)

func BenchmarkThemeStress_EachFactory(b *testing.B) {
	for _, o := range style.BuiltInEditorThemes() {
		o := o
		b.Run(o.Name, func(b *testing.B) {
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_ = o.Factory()
			}
		})
	}
}
