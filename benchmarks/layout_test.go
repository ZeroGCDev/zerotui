package benchmarks

import (
	"testing"

	"github.com/ZeroGCDev/zerotui/geometry"
	"github.com/ZeroGCDev/zerotui/layout"
	"github.com/ZeroGCDev/zerotui/widget"
)

func layoutLeaves(n int) []layout.Node {
	out := make([]layout.Node, n)
	for i := range out {
		out[i] = layout.Wrap(widget.NewLabel("item"))
	}
	return out
}
func BenchmarkLayoutFlexHorizontal(b *testing.B) {
	nodes := layoutLeaves(16)
	items := make([]layout.Item, 0, len(nodes))
	for _, n := range nodes {
		items = append(items, layout.Flex1(n))
	}
	root := layout.NewFlex(layout.Horizontal, items...)
	area := geometry.Rect{W: 160, H: 48}
	dst := make([]layout.Placement, 0, 32)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		dst = layout.ComputeInto(root, area, dst)
	}
	sinkInt = len(dst)
}
func BenchmarkLayoutFlexVertical(b *testing.B) {
	nodes := layoutLeaves(16)
	items := make([]layout.Item, 0, len(nodes))
	for _, n := range nodes {
		items = append(items, layout.Fix(n, 3))
	}
	root := layout.NewFlex(layout.Vertical, items...)
	dst := make([]layout.Placement, 0, 32)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		dst = layout.ComputeInto(root, geometry.Rect{W: 160, H: 48}, dst)
	}
	sinkInt = len(dst)
}
func BenchmarkLayoutSplit(b *testing.B) {
	leaf := layout.Wrap(widget.NewLabel("split"))
	root := layout.NewSplit(layout.Horizontal, leaf, leaf, .5)
	dst := make([]layout.Placement, 0, 8)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		dst = layout.ComputeInto(root, geometry.Rect{W: 160, H: 48}, dst)
	}
	sinkInt = len(dst)
}
func BenchmarkLayoutGrid(b *testing.B) {
	root := layout.NewGrid(4, 4, layoutLeaves(16)...)
	dst := make([]layout.Placement, 0, 32)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		dst = layout.ComputeInto(root, geometry.Rect{W: 160, H: 48}, dst)
	}
	sinkInt = len(dst)
}
func BenchmarkLayoutStack(b *testing.B) {
	root := layout.NewStack(layoutLeaves(16)...)
	dst := make([]layout.Placement, 0, 32)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		dst = layout.ComputeInto(root, geometry.Rect{W: 160, H: 48}, dst)
	}
	sinkInt = len(dst)
}
func BenchmarkLayoutCenter(b *testing.B) {
	root := layout.Center(layout.Wrap(widget.NewLabel("center")), 0.5, 0.5)
	dst := make([]layout.Placement, 0, 4)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		dst = layout.ComputeInto(root, geometry.Rect{W: 160, H: 48}, dst)
	}
	sinkInt = len(dst)
}
func BenchmarkLayoutResponsive(b *testing.B) {
	compact := layout.NewFlex(layout.Vertical, layout.Flex1(layout.Wrap(widget.NewLabel("a"))), layout.Flex1(layout.Wrap(widget.NewLabel("b"))))
	expanded := layout.NewFlex(layout.Horizontal, layout.Flex1(layout.Wrap(widget.NewLabel("a"))), layout.Flex1(layout.Wrap(widget.NewLabel("b"))))
	root := layout.Responsive(100, compact, expanded)
	dst := make([]layout.Placement, 0, 8)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		dst = layout.ComputeInto(root, geometry.Rect{W: 80 + i%100, H: 48}, dst)
	}
	sinkInt = len(dst)
}
func BenchmarkLayoutOverlay(b *testing.B) {
	visible := true
	root := layout.NewOverlay(func() bool { return visible }, layout.Wrap(widget.NewLabel("overlay")))
	dst := make([]layout.Placement, 0, 4)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		visible = i&1 == 0
		dst = layout.ComputeInto(root, geometry.Rect{W: 160, H: 48}, dst)
	}
	sinkInt = len(dst)
}
func BenchmarkLayoutNested(b *testing.B) {
	leaf := layout.Wrap(widget.NewLabel("nested"))
	root := layout.BorderedRounded("ROOT",
		layout.NewFlex(layout.Horizontal,
			layout.Flex1(layout.Bordered("A", layout.NewGrid(2, 2, leaf, leaf, leaf, leaf), nil)),
			layout.Flex1(layout.Center(layout.NewStack(leaf, leaf, leaf), .8, .8)),
		), nil)
	dst := make([]layout.Placement, 0, 32)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		dst = layout.ComputeInto(root, geometry.Rect{W: 160, H: 48}, dst)
	}
	sinkInt = len(dst)
}
func BenchmarkLayoutLargeTree(b *testing.B) {
	nodes := layoutLeaves(256)
	root := layout.NewGrid(16, 16, nodes...)
	dst := make([]layout.Placement, 0, 300)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		dst = layout.ComputeInto(root, geometry.Rect{W: 160, H: 80}, dst)
	}
	sinkInt = len(dst)
}
func BenchmarkLayoutResizeReflow(b *testing.B) {
	leaf := layout.Wrap(widget.NewLabel("resize"))
	root := layout.NewFlex(layout.Horizontal,
		layout.Flex1(leaf), layout.Flex1(leaf), layout.Flex1(leaf), layout.Flex1(leaf),
	)
	dst := make([]layout.Placement, 0, 8)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		dst = layout.ComputeInto(root, geometry.Rect{W: 80 + i%81, H: 24 + i%25}, dst)
	}
	sinkInt = len(dst)
}

func BenchmarkLayoutBordered(b *testing.B) {
	root := layout.BorderedRounded("PANEL", layout.Wrap(widget.NewLabel("content")), nil)
	dst := make([]layout.Placement, 0, 4)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		dst = layout.ComputeInto(root, geometry.Rect{W: 160, H: 48}, dst)
	}
	sinkInt = len(dst)
}
func BenchmarkLayoutClosable(b *testing.B) {
	root := layout.ClosableRounded("CLOSE", layout.Wrap(widget.NewLabel("content")), nil, nil)
	dst := make([]layout.Placement, 0, 4)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		dst = layout.ComputeInto(root, geometry.Rect{W: 160, H: 48}, dst)
	}
	sinkInt = len(dst)
}
