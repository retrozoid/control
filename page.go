package control

import (
	"errors"
	"log/slog"
	"strings"

	"github.com/retrozoid/control/protocol/common"
	"github.com/retrozoid/control/protocol/dom"
	"github.com/retrozoid/control/protocol/overlay"
	"github.com/retrozoid/control/protocol/page"
)

const documentElement = "document.documentElement"

type Maybe[T any] struct {
	value T
	err   error
}

func (may Maybe[T]) Unwrap() (T, error) {
	return may.value, may.err
}

func (may Maybe[T]) Err() error {
	return may.err
}

func (may Maybe[T]) Value() T {
	if may.err != nil {
		panic(may.err)
	}
	return may.value
}

type MaybeNode = Maybe[*Node]

func queryError(err error) MaybeNode {
	return MaybeNode{err: err}
}

type Queryable interface {
	Query(string) MaybeNode
	QueryAll(string) MaybeNode
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

func (f Frame) Evaluate(expression string, awaitPromise bool) Maybe[any] {
	value, err := f.evaluate(expression, awaitPromise)
	f.Log(slog.LevelInfo, "Evaluate", "expression", expression, "awaitPromise", awaitPromise, "value", value, "err", err)
	return Maybe[any]{err: err, value: value}
}

func (f Frame) newNode(selector string, value any, err error) MaybeNode {
	if err != nil {
		return queryError(err)
	}
	if value == nil {
		return queryError(NoSuchSelectorError(selector))
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
		return MaybeNode{value: n}
	}
	f.Log(slog.LevelError, "can't cast remote object to Node", "value", value)
	return queryError(errors.New("can't cast remote object to Node"))
}

func safeSelector(v string) string {
	v = strings.TrimSpace(v)
	v = strings.ReplaceAll(v, `"`, `\"`)
	return v
}

func (f Frame) Document() MaybeNode {
	value, err := f.evaluate(documentElement, true)
	return f.newNode(documentElement, value, err)
}

func (f Frame) Query(cssSelector string) MaybeNode {
	doc := f.Document()
	if doc.Err() != nil {
		return doc
	}
	return doc.Value().Query(cssSelector)
}

func (f Frame) QueryAll(cssSelector string) MaybeNode {
	doc := f.Document()
	if doc.Err() != nil {
		return doc
	}
	return doc.Value().QueryAll(cssSelector)
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

func (f Frame) GetLayoutMetrics() (*page.GetLayoutMetricsVal, error) {
	view, err := page.GetLayoutMetrics(f)
	if err != nil {
		return nil, err
	}
	return view, nil
}
