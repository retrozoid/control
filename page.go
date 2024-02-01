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

type Maybe[T any] struct {
	Value T
	Err   error
}

func (may Maybe[T]) MustGet() T {
	if may.Err != nil {
		panic(may.Err)
	}
	return may.Value
}

type Nodes []Node
type MaybeNode = Maybe[Node]
type MaybeNodes = Maybe[Nodes]

type Queryable interface {
	Query(string) MaybeNode
	QueryAll(string) MaybeNodes
	OwnFrame() Frame
}

type Frame struct {
	session *Session
	id      common.FrameId
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

func (f Frame) OwnFrame() Frame {
	return f
}

func (f Frame) Navigate(url string) error {
	err := f.navigate(url)
	f.session.Log(slog.LevelInfo, "Navigate", "url", url, "err", err)
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
	f.session.Log(slog.LevelInfo, "ignoreCache", ignoreCache, "scriptToEvaluateOnLoad", scriptToEvaluateOnLoad, "err", err)
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

func (f Frame) newNode(selector string, value any, err error) MaybeNode {
	if err != nil {
		return MaybeNode{Err: err}
	}
	if value == nil {
		return MaybeNode{Err: NoSuchSelectorError(selector)}
	}
	if n, ok := value.(Node); ok {
		if DebugOverlays && selector != "document" {
			_ = overlay.HighlightNode(f, overlay.HighlightNodeArgs{
				HighlightConfig: &overlay.HighlightConfig{
					ShowInfo:     false,
					ContentColor: &dom.RGBA{R: 255, G: 0, B: 255, A: 0.2},
				},
				ObjectId: n.ObjectID(),
			})
		}
		n.cssSelector = selector
		return MaybeNode{Value: n}
	}
	return MaybeNode{Err: ErrMismatchTypeDeserialization}
}

func (f Frame) newNodes(selector string, value any, err error) MaybeNodes {
	if err != nil {
		return MaybeNodes{Err: err}
	}
	if value == nil {
		return MaybeNodes{Err: NoSuchSelectorError(selector)}
	}
	if n, ok := value.([]Node); ok {
		return MaybeNodes{Value: n}
	}
	return MaybeNodes{Err: ErrMismatchTypeDeserialization}
}

func safeSelector(v string) string {
	v = strings.TrimSpace(v)
	v = strings.ReplaceAll(v, `"`, `\"`)
	return v
}

func (f Frame) Document() MaybeNode {
	value, err := f.Evaluate(`document.documentElement`, true)
	node := f.newNode("document", value, err)
	f.session.Log(slog.LevelInfo, "GetDocumentElement", "cssSelector", "document.documentElement", "err", node.Err)
	return node
}

func (f Frame) Query(cssSelector string) MaybeNode {
	doc := f.Document()
	if doc.Err != nil {
		return doc
	}
	return doc.MustGet().Query(cssSelector)
}

func (f Frame) QueryAll(cssSelector string) MaybeNodes {
	doc := f.Document()
	if doc.Err != nil {
		return MaybeNodes{Err: doc.Err}
	}
	return doc.MustGet().QueryAll(cssSelector)
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
