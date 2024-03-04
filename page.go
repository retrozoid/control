package control

import (
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/retrozoid/control/protocol/common"
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

func (f Frame) Log(level slog.Level, msg string, args ...any) {
	args = append(args, "frameId", f.id)
	f.session.Log(level, msg, args...)
}

func (f Frame) Navigate(url string) error {
	err := f.navigate(url)
	f.Log(slog.LevelInfo, "Navigate", "url", url, "err", err)
	return err
}

func (f Frame) navigate(url string) error {
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
	if _, err = future.Get(); err != nil {
		return err
	}
	return nil
}

func (f Frame) Reload(ignoreCache bool, scriptToEvaluateOnLoad string) error {
	err := f.reload(ignoreCache, scriptToEvaluateOnLoad)
	f.Log(slog.LevelInfo, "Reload", "ignoreCache", ignoreCache, "scriptToEvaluateOnLoad", scriptToEvaluateOnLoad, "err", err)
	return err
}

func (f Frame) reload(ignoreCache bool, scriptToEvaluateOnLoad string) error {
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
	if _, err = future.Get(); err != nil {
		return err
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
	value, err := f.evaluate(expression, awaitPromise)
	f.Log(slog.LevelInfo, "Evaluate", "expression", truncate(expression, truncateLongStringLen), "awaitPromise", awaitPromise, "value", truncate(fmt.Sprint(value), truncateLongStringLen), "err", err)
	return Optional[any]{value: value, err: err}
}

func safeSelector(v string) string {
	v = strings.TrimSpace(v)
	v = strings.ReplaceAll(v, `"`, `\"`)
	return v
}

func (f Frame) Document() Optional[*Node] {
	value, err := f.evaluate(document, true)
	opt := optional[*Node](value, err)
	if opt.err == nil && opt.value == nil {
		opt.err = NoSuchSelectorError(document)
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
	return NewMouse(f).Click(MouseLeft, point, time.Millisecond*42)
}

func (f Frame) Hover(point Point) error {
	return NewMouse(f).Move(MouseNone, point)
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
	e, err := f.GetNavigationEntry().Unwrap()
	if err != nil {
		f.Log(slog.LevelInfo, "GetCurrentURL", "err", err)
		return Optional[string]{err: err}
	}
	f.Log(slog.LevelInfo, "GetCurrentURL", "value", e.Url, "err", err)
	return Optional[string]{value: e.Url}
}

func (f Frame) NavigateHistory(delta int) error {
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
