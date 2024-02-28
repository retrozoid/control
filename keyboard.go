package control

import (
	"time"

	"github.com/retrozoid/control/key"
	"github.com/retrozoid/control/protocol"
	"github.com/retrozoid/control/protocol/input"
)

type Keyboard struct {
	caller protocol.Caller
}

func NewKeyboard(caller protocol.Caller) Keyboard {
	return Keyboard{caller: caller}
}

func (k Keyboard) Down(key key.Definition) error {
	if key.Text == "" && len(key.Key) == 1 {
		key.Text = key.Key
	}
	return input.DispatchKeyEvent(k.caller, input.DispatchKeyEventArgs{
		Type:                  "keyDown",
		WindowsVirtualKeyCode: key.KeyCode,
		Code:                  key.Code,
		Key:                   key.Key,
		Text:                  key.Text,
		Location:              key.Location,
	})
}

func (k Keyboard) Up(key key.Definition) error {
	return input.DispatchKeyEvent(k.caller, input.DispatchKeyEventArgs{
		Type:                  "keyUp",
		WindowsVirtualKeyCode: key.KeyCode,
		Code:                  key.Code,
		Key:                   key.Key,
	})
}

func (k Keyboard) Insert(text string) error {
	return input.InsertText(k.caller, input.InsertTextArgs{Text: text})
}

func (k Keyboard) Press(key key.Definition, delay time.Duration) (err error) {
	if err = k.Down(key); err != nil {
		return err
	}
	if delay > 0 {
		time.Sleep(delay)
	}
	return k.Up(key)
}
