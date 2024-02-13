package control

import (
	"errors"
	"fmt"
	"log/slog"
	"reflect"
	"strings"
	"time"

	"github.com/retrozoid/control/protocol/common"
	"github.com/retrozoid/control/protocol/dom"
	"github.com/retrozoid/control/protocol/overlay"
	"github.com/retrozoid/control/protocol/page"
)

const (
	documentElement       = "document.documentElement"
	truncateLongStringLen = 1024
)

type Optional[T any] struct {
	value T
	err   error
}

func castValue[T any](value any, err error) Optional[T] {
	if err != nil {
		return Optional[T]{err: err}
	}
	var t T
	if value == nil {
		return Optional[T]{value: t}
	}
	var ok bool
	if t, ok = value.(T); ok {
		return Optional[T]{value: t}
	}
	return Optional[T]{err: fmt.Errorf("can't cast %s to %s", reflect.TypeOf(value), reflect.TypeOf(t))}
}

func (may Optional[T]) Unwrap() (T, error) {
	return may.value, may.err
}

func (may Optional[T]) Err() error {
	return may.err
}

func (may Optional[T]) Value() T {
	if may.err != nil {
		panic(may.err)
	}
	return may.value
}

func (may Optional[T]) IfPresent(f func(T)) {
	if may.err == nil {
		f(may.value)
	}
}

type Queryable interface {
	Query(string) Optional[*Node]
	QueryAll(string) Optional[*Node]
	OwnFrame() *Frame
}

type Frame struct {
	session     *Session
	id          common.FrameId
	cssSelector string
	descendant  *Frame
}

func (f Frame) GetSession() *Session {
	return f.session
}

func (f Frame) executionContextID() string {
	// todo retry
	return f.session.frames.Get(f.id)
}

func (f Frame) Call(method string, send, recv any) error {
	return f.session.Call(method, send, recv)
}

func (f *Frame) OwnFrame() *Frame {
	return f
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
		b.WriteString(")")
		return b.String()
	}
	return value
}

func (f Frame) Evaluate(expression string, awaitPromise bool) Optional[any] {
	value, err := f.evaluate(expression, awaitPromise)
	f.Log(slog.LevelInfo, "Evaluate", "expression", truncate(expression, truncateLongStringLen), "awaitPromise", awaitPromise, "value", truncate(fmt.Sprint(value), truncateLongStringLen), "err", err)
	return Optional[any]{value: value, err: err}
}

func (f Frame) newNode(selector string, value any, err error) Optional[*Node] {
	if err != nil {
		return Optional[*Node]{err: err}
	}
	if value == nil {
		return Optional[*Node]{err: NoSuchSelectorError(selector)}
	}
	if n, ok := value.(*Node); ok {
		if DebugHighlightEnabled && selector != documentElement {
			_ = overlay.HighlightNode(f, overlay.HighlightNodeArgs{
				HighlightConfig: &overlay.HighlightConfig{
					ContentColor: &dom.RGBA{R: 255, G: 0, B: 255, A: 0.2},
				},
				ObjectId: n.ObjectID(),
			})
		}
		n.cssSelector = selector
		return Optional[*Node]{value: n}
	}
	f.Log(slog.LevelError, "can't cast remote object to Node", "value", value)
	return Optional[*Node]{err: errors.New("can't cast remote object to Node")}
}

func safeSelector(v string) string {
	v = strings.TrimSpace(v)
	v = strings.ReplaceAll(v, `"`, `\"`)
	return v
}

func (f Frame) Document() Optional[*Node] {
	value, err := f.evaluate(documentElement, true)
	return f.newNode(documentElement, value, err)
}

func (f Frame) Query(cssSelector string) Optional[*Node] {
	doc := f.Document()
	if doc.Err() != nil {
		return doc
	}
	return doc.Value().Query(cssSelector)
}

func (f Frame) QueryAll(cssSelector string) Optional[*Node] {
	doc := f.Document()
	if doc.Err() != nil {
		return doc
	}
	return doc.Value().QueryAll(cssSelector)
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
