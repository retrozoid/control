package control

import (
	"context"
	"errors"
	"strings"

	"github.com/ecwid/control/protocol/common"
	"github.com/ecwid/control/protocol/page"
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
	future := MakeFuture(f.session, "Page.loadEventFired", func(fired page.LoadEventFired) bool {
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
	timeoutContext, cancel := context.WithTimeout(f.session.context, f.session.Timeout)
	defer cancel()
	_, err = future.Get(timeoutContext)
	return err
}

func (f Frame) Reload(ignoreCache bool, scriptToEvaluateOnLoad string) error {
	future := MakeFuture(f.session, "Page.loadEventFired", func(fired page.LoadEventFired) bool {
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
	timeoutContext, cancel := context.WithTimeout(f.session.context, f.session.Timeout)
	defer cancel()
	_, err = future.Get(timeoutContext)
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
