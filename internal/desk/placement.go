package desk

// rect is a window rectangle in pixels. Right and Bottom are exclusive.
type rect struct {
	Left, Top, Right, Bottom int32
}

type point struct {
	X, Y int32
}

// maxPlacement is where a thick-frame window should sit when it is maximized.
// The invisible resize border hangs outside the area, so the page itself fills
// either the work area (taskbar still visible) or the whole monitor.
func maxPlacement(work, monitor rect, frame int32, fullscreen bool) (point, point) {
	area := work
	if fullscreen {
		area = monitor
	}
	pos := point{
		X: area.Left - monitor.Left - frame,
		Y: area.Top - monitor.Top - frame,
	}
	size := point{
		X: area.Right - area.Left + frame*2,
		Y: area.Bottom - area.Top + frame*2,
	}
	return pos, size
}
