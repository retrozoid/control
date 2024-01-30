package control

import (
	"errors"
	"strings"

	"github.com/retrozoid/control/protocol/common"
	"github.com/retrozoid/control/protocol/page"
)

type Frame struct {
	session *Session
	id      common.FrameId
}

func (f Frame) executionContextID() string {
	// todo retry
	return f.session.frames.Get(f.id)
}

func (f Frame) Call(method string, send, recv any) error {
	return f.session.Call(method, send, recv)
}

func (f Frame) Navigate(url string) error {
	future := MakeFuture(f.session, "Page.loadEventFired", func(_ page.LoadEventFired) bool {
		return true
	})
	defer future.Cancel()
	nav, err := page.Navigate(f, page.NavigateArgs{
		Url:     url,
		FrameId: f.id,
	})
	if err != nil {
		return err
	}
	if nav.ErrorText != "" {
		return errors.New(nav.ErrorText)
	}
	if nav.LoaderId == "" {
		return nil
	}
	_, err = future.Get()
	return err
}

func (f Frame) Reload(ignoreCache bool, scriptToEvaluateOnLoad string) error {
	future := MakeFuture(f.session, "Page.loadEventFired", func(_ page.LoadEventFired) bool {
		return true
	})
	defer future.Cancel()
	err := page.Reload(f, page.ReloadArgs{
		IgnoreCache:            ignoreCache,
		ScriptToEvaluateOnLoad: scriptToEvaluateOnLoad,
	})
	if err != nil {
		return err
	}
	_, err = future.Get()
	return err
}

func safeSelector(v string) string {
	v = strings.TrimSpace(v)
	v = strings.ReplaceAll(v, `"`, `\"`)
	return v
}

func (f Frame) Query(cssSelector string) OptionalNode {
	value, err := f.Evaluate(`document.querySelector("`+safeSelector(cssSelector)+`")`, true)
	return toOptionalNode(value, err)
}

func (f Frame) QueryAll(cssSelector string) OptionalNode {
	value, err := f.Evaluate(`document.querySelectorAll("`+safeSelector(cssSelector)+`")`, true)
	return toOptionalNode(value, err)
}

func (f Frame) Click(point Point) error {
	return NewMouse(f).Click(MouseLeft, point)
}

func (f Frame) Hover(point Point) error {
	return NewMouse(f).Move(MouseNone, point)
}

func (f Frame) GetNavigationEntry() (*page.NavigationEntry, error) {
	val, err := page.GetNavigationHistory(f)
	if err != nil {
		return nil, err
	}
	if val.CurrentIndex == -1 {
		return &page.NavigationEntry{Url: Blank}, nil
	}
	return val.Entries[val.CurrentIndex], nil
}
