package control

import (
	"sync"
	"time"

	"github.com/retrozoid/control/protocol"
	"github.com/retrozoid/control/protocol/input"
)

const (
	MouseNone    input.MouseButton = "none"
	MouseLeft    input.MouseButton = "left"
	MouseRight   input.MouseButton = "right"
	MouseMiddle  input.MouseButton = "middle"
	MouseBack    input.MouseButton = "back"
	MouseForward input.MouseButton = "forward"
)

func NewMouse(caller protocol.Caller) Mouse {
	return Mouse{
		caller: caller,
		mutex:  &sync.Mutex{},
	}
}

type Mouse struct {
	caller protocol.Caller
	mutex  *sync.Mutex
}

func (m Mouse) Move(button input.MouseButton, point Point) error {
	return input.DispatchMouseEvent(m.caller, input.DispatchMouseEventArgs{
		X:          point.X,
		Y:          point.Y,
		Type:       "mouseMoved",
		Button:     button,
		ClickCount: 1,
	})
}

func (m Mouse) Press(button input.MouseButton, point Point) error {
	return input.DispatchMouseEvent(m.caller, input.DispatchMouseEventArgs{
		X:          point.X,
		Y:          point.Y,
		Type:       "mousePressed",
		Button:     button,
		ClickCount: 1,
	})
}

func (m Mouse) Release(button input.MouseButton, point Point) error {
	return input.DispatchMouseEvent(m.caller, input.DispatchMouseEventArgs{
		X:          point.X,
		Y:          point.Y,
		Type:       "mouseReleased",
		Button:     button,
		ClickCount: 1,
	})
}

func (m Mouse) Click(button input.MouseButton, point Point, delay time.Duration) (err error) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	if err = m.Move(MouseNone, point); err != nil {
		return err
	}
	if err = m.Press(button, point); err != nil {
		return err
	}
	time.Sleep(delay)
	if err = m.Release(button, point); err != nil {
		return err
	}
	return
}
