package desk

import "testing"

func TestMaxPlacementFillsTheWorkArea(t *testing.T) {
	monitor := rect{Left: 0, Top: 0, Right: 1920, Bottom: 1080}
	work := rect{Left: 0, Top: 0, Right: 1920, Bottom: 1040}
	const frame int32 = 8
	pos, size := maxPlacement(work, monitor, frame, false)
	if top, height := pos.Y+frame, size.Y-frame*2; top != work.Top || top+height != work.Bottom {
		t.Fatalf("vertical client = %d+%d, work is %d..%d", top, height, work.Top, work.Bottom)
	}
	if left, width := pos.X+frame, size.X-frame*2; left != work.Left || left+width != work.Right {
		t.Fatalf("horizontal client = %d+%d, work is %d..%d", left, width, work.Left, work.Right)
	}
}

func TestFullscreenCoversTheMonitor(t *testing.T) {
	monitor := rect{Left: 1920, Top: 0, Right: 3840, Bottom: 1080}
	work := rect{Left: 1920, Top: 0, Right: 3840, Bottom: 1040}
	const frame int32 = 8
	pos, size := maxPlacement(work, monitor, frame, true)
	if pos.X+frame != 0 || pos.Y+frame != 0 {
		t.Fatalf("fullscreen origin %d,%d", pos.X, pos.Y)
	}
	if pos.Y+size.Y-frame != monitor.Bottom-monitor.Top {
		t.Fatalf("fullscreen bottom %d", pos.Y+size.Y-frame)
	}
}
