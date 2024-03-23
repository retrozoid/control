package control

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/retrozoid/control/protocol/common"
	"github.com/retrozoid/control/protocol/dom"
	"github.com/retrozoid/control/protocol/page"
)

const (
	truncateLongStringLen = 1024
	document              = "document"
)

type Queryable interface {
	Query(string) Optional[*Node]
	QueryAll(string) Optional[*NodeList]
	OwnFrame() *Frame
}

type Frame struct {
	session     *Session
	id          common.FrameId
	cssSelector string
	parent      *Frame
}

func (f Frame) GetSession() *Session {
	return f.session
}

func (f Frame) executionContextID() string {
	if value, ok := f.session.frames.Load(f.id); ok {
		return value.(string)
	}
	return ""
}

func (f Frame) Call(method string, send, recv any) error {
	return f.session.Call(method, send, recv)
}

func (f *Frame) OwnFrame() *Frame {
	return f
}

func (f *Frame) Parent() *Frame {
	return f.parent
}

func (f Frame) Log(t time.Time, msg string, args ...any) {
	args = append(args, "frameId", f.id)
	f.session.Log(t, msg, args...)
}

func (f Frame) Navigate(url string) error {
	now := time.Now()
	err := f.navigate(url)
	f.Log(now, "Navigate", "url", url, "err", err)
	return err
}

func (f Frame) navigate(url string) error {
	future := Subscribe(f.session, "Page.loadEventFired", func(_ page.LoadEventFired) bool {
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
	if _, err = future.Get(); err != nil {
		return errors.Join(err, errors.New("navigation did not complete"))
	}
	return nil
}

func (f Frame) Reload(ignoreCache bool, scriptToEvaluateOnLoad string) error {
	now := time.Now()
	err := f.reload(ignoreCache, scriptToEvaluateOnLoad)
	f.Log(now, "Reload", "ignoreCache", ignoreCache, "scriptToEvaluateOnLoad", scriptToEvaluateOnLoad, "err", err)
	return err
}

func (f Frame) reload(ignoreCache bool, scriptToEvaluateOnLoad string) error {
	future := Subscribe(f.session, "Page.loadEventFired", func(_ page.LoadEventFired) bool {
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
	if _, err = future.Get(); err != nil {
		return errors.Join(err, errors.New("reload did not complete"))
	}
	return nil
}

func truncate(value string, length int) string {
	if len(value) > length {
		var b = strings.Builder{}
		b.WriteString(value[:length])
		b.WriteString(" (truncated ")
		b.WriteString(fmt.Sprint(len(value[length:])))
		b.WriteString(" bytes)")
		return b.String()
	}
	return value
}

func (f Frame) Evaluate(expression string, awaitPromise bool) Optional[any] {
	now := time.Now()
	value, err := f.evaluate(expression, awaitPromise)
	f.Log(now, "Evaluate",
		"expression", truncate(expression, truncateLongStringLen),
		"awaitPromise", awaitPromise,
		"value", truncate(fmt.Sprint(value), truncateLongStringLen),
		"err", err,
	)
	return Optional[any]{value: value, err: err}
}

func (f Frame) getNodeForLocation(p Point) (dom.NodeId, error) {
	value, err := dom.GetNodeForLocation(f, dom.GetNodeForLocationArgs{
		X:                         int(p.X),
		Y:                         int(p.Y),
		IncludeUserAgentShadowDOM: true,
		IgnorePointerEventsNone:   true,
	})
	if err != nil {
		return 0, nil
	}
	return value.NodeId, nil
}

func (f Frame) Document() Optional[*Node] {
	value, err := f.evaluate(document, true)
	opt := optional[*Node](value, err)
	if opt.err == nil && opt.value == nil {
		opt.err = NoSuchSelectorError(document)
	}
	if opt.value != nil {
		opt.value.requestedSelector = document
	}
	return opt
}

func (f Frame) Query(cssSelector string) Optional[*Node] {
	doc, err := f.Document().Unwrap()
	if err != nil {
		return Optional[*Node]{err: err}
	}
	return doc.Query(cssSelector)
}

func (f Frame) QueryAll(cssSelector string) Optional[*NodeList] {
	doc, err := f.Document().Unwrap()
	if err != nil {
		return Optional[*NodeList]{err: err}
	}
	return doc.QueryAll(cssSelector)
}

func (f Frame) Click(point Point) error {
	return f.session.mouse.Click(MouseLeft, point, time.Millisecond*85)
}

func (f Frame) Hover(point Point) error {
	return f.session.mouse.Move(MouseNone, point)
}

func (f Frame) GetLayout() Optional[page.GetLayoutMetricsVal] {
	view, err := page.GetLayoutMetrics(f)
	if err != nil {
		return Optional[page.GetLayoutMetricsVal]{err: err}
	}
	return Optional[page.GetLayoutMetricsVal]{value: *view}
}

func (f Frame) GetNavigationEntry() Optional[page.NavigationEntry] {
	val, err := page.GetNavigationHistory(f)
	if err != nil {
		return Optional[page.NavigationEntry]{err: err}
	}
	if val.CurrentIndex == -1 {
		return Optional[page.NavigationEntry]{value: page.NavigationEntry{Url: Blank}}
	}
	return Optional[page.NavigationEntry]{value: *val.Entries[val.CurrentIndex]}
}

func (f Frame) GetCurrentURL() Optional[string] {
	now := time.Now()
	opt := optional[string](f.getCurrentURL())
	f.Log(now, "GetCurrentURL", "value", opt.value, "err", opt.err)
	return opt
}

func (f Frame) getCurrentURL() (string, error) {
	e, err := f.GetNavigationEntry().Unwrap()
	if err != nil {
		return "", err
	}
	return e.Url, nil
}

func (f Frame) NavigateHistory(delta int) error {
	now := time.Now()
	err := f.navigateHistory(delta)
	f.Log(now, "NavigateHistory", "delta", delta, "err", err)
	return err
}

func (f Frame) navigateHistory(delta int) error {
	val, err := page.GetNavigationHistory(f)
	if err != nil {
		return err
	}
	move := val.CurrentIndex + delta
	if move >= 0 && move < len(val.Entries) {
		return page.NavigateToHistoryEntry(f, page.NavigateToHistoryEntryArgs{
			EntryId: val.Entries[move].Id,
		})
	}
	return nil
}
