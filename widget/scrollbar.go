package widget

import (
	"github.com/ZeroGCDev/zerotui/buffer"
	"github.com/ZeroGCDev/zerotui/color"
	"github.com/ZeroGCDev/zerotui/geometry"
	"github.com/ZeroGCDev/zerotui/style"
)

// ScrollBar is a tiny terminal-native scrollbar shared by virtual views. It
// draws only the viewport track and thumb; it never allocates.
type ScrollBar struct {
	Total      int
	Offset     int
	Viewport   int
	Track      *color.Color // optional track foreground override
	Thumb      *color.Color // optional thumb foreground override
	Background *color.Color // optional track/thumb background
}

func (s ScrollBar) Draw(buf *buffer.Buffer, area geometry.Rect, theme *style.Theme) {
	if area.W < 1 || area.H < 1 || s.Total <= s.Viewport {
		return
	}
	track := bgOr(theme.TrackEmpty, s.Background)
	if s.Track != nil {
		track = track.WithFg(*s.Track)
	}
	thumbStyle := bgOr(theme.Info, s.Background)
	if s.Thumb != nil {
		thumbStyle = thumbStyle.WithFg(*s.Thumb)
	}
	buf.FillRect(area.X, area.Y, 1, area.H, '│', track)
	thumb := area.H * s.Viewport / s.Total
	if thumb < 1 {
		thumb = 1
	}
	maxOffset := s.Total - s.Viewport
	pos := 0
	if maxOffset > 0 {
		pos = (area.H - thumb) * s.Offset / maxOffset
	}
	buf.FillRect(area.X, area.Y+pos, 1, thumb, '█', thumbStyle)
}
