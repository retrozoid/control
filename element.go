package control

import (
	"errors"
	"fmt"
	"log/slog"
	"math"

	"github.com/retrozoid/control/protocol/dom"
	"github.com/retrozoid/control/protocol/runtime"
)

var (
	ErrElementNotVisible           = errors.New("element not visible")
	ErrMismatchTypeDeserialization = errors.New("mismatch type of remote object")
	ErrElementIsOutOfViewport      = errors.New("element is out of viewport")
	ErrIterNoResult                = errors.New("no filter result")
)

type NoSuchSelectorError string

func (s NoSuchSelectorError) Error() string {
	return fmt.Sprintf("no such selector found: `%s`", string(s))
}

type Point struct {
	X float64
	Y float64
}

type Quad []Point

func convertQuads(dq []dom.Quad) []Quad {
	var p = make([]Quad, len(dq))
	for n, q := range dq {
		p[n] = Quad{
			Point{q[0], q[1]},
			Point{q[2], q[3]},
			Point{q[4], q[5]},
			Point{q[6], q[7]},
		}
	}
	return p
}

// Middle calc middle of quad
func (q Quad) Middle() Point {
	x := 0.0
	y := 0.0
	for i := 0; i < 4; i++ {
		x += q[i].X
		y += q[i].Y
	}
	return Point{X: x / 4, Y: y / 4}
}

// Area calc area of quad
func (q Quad) Area() float64 {
	var area float64
	var x1, x2, y1, y2 float64
	var vertices = len(q)
	for i := 0; i < vertices; i++ {
		x1 = q[i].X
		y1 = q[i].Y
		x2 = q[(i+1)%vertices].X
		y2 = q[(i+1)%vertices].Y
		area += (x1*y2 - x2*y1) / 2
	}
	return math.Abs(area)
}

func (e Node) ObjectID() runtime.RemoteObjectId {
	return e.JsObject.ObjectID()
}

func (e Node) OwnFrame() Frame {
	return e.frame
}

func (e Node) Call(method string, send, recv interface{}) error {
	return e.frame.Call(method, send, recv)
}

func (e Node) eval(function string, args ...any) (any, error) {
	return e.frame.callFunctionOn(e, function, true, args...)
}

func (e Node) asyncEval(function string, args ...any) (JsObject, error) {
	value, err := e.frame.callFunctionOn(e, function, false, args...)
	if err != nil {
		return nil, err
	}
	return value.(JsObject), nil
}

func (e Node) dispatchEvents(events ...any) error {
	_, err := e.eval(`function(l){for(const e of l)this.dispatchEvent(new Event(e,{'bubbles':!0}))}`, events...)
	return err
}

func (e Node) info(msg string, args ...any) {
	e.frame.session.Log(slog.LevelInfo, msg, args...)
}

func (e Node) query(cssSelector string) (any, error) {
	cssSelector = safeSelector(cssSelector)
	return e.eval(`function(s){return this.querySelector(s)}`, cssSelector)

}

func (e Node) Query(cssSelector string) MaybeNode {
	cssSelector = safeSelector(cssSelector)
	value, err := e.eval(`function(s){return this.querySelector(s)}`, cssSelector)
	node := e.frame.newNode(cssSelector, value, err)
	e.info("Query", "self", e.cssSelector, "cssSelector", cssSelector, "err", node.Err)
	return node
}

func (e Node) QueryAll(cssSelector string) MaybeNodes {
	cssSelector = safeSelector(cssSelector)
	value, err := e.eval(`function(s){return this.querySelectorAll(s)}`, cssSelector)
	nodes := e.frame.newNodes(cssSelector, value, err)
	e.info("QueryAll", "self", e.cssSelector, "cssSelector", cssSelector, "err", nodes.Err)
	return nodes
}

func (e Node) AsFrame() (Frame, error) {
	f, err := e.asFrame()
	e.info("Frame", "self", e.cssSelector, "value", f, "err", err)
	return f, err
}

func (e Node) asFrame() (Frame, error) {
	value, err := dom.DescribeNode(e, dom.DescribeNodeArgs{
		ObjectId: e.ObjectID(),
	})
	if err != nil {
		return Frame{}, err
	}
	result := Frame{
		id:      value.Node.FrameId,
		session: e.frame.session,
	}
	return result, nil
}

func (e Node) ScrollIntoView() error {
	return dom.ScrollIntoViewIfNeeded(e, dom.ScrollIntoViewIfNeededArgs{
		ObjectId: e.ObjectID(),
	})
}

func (e Node) GetTextContent() (string, error) {
	value, err := e.eval(`function(){return this.textContent.trim()}`)
	if err != nil {
		return "", err
	}
	e.info("GetTextContent", "self", e.cssSelector, "textContent", value, "err", err)
	return value.(string), nil
}

func (e Node) Focus() error {
	err := dom.Focus(e, dom.FocusArgs{
		ObjectId: e.ObjectID(),
	})
	e.info("Focus", "self", e.cssSelector, "err", err)
	return err
}

func (e Node) Blur() error {
	_, err := e.eval(`function(){this.blur()}`)
	e.info("Blur", "self", e.cssSelector, "err", err)
	return err
}

func (e Node) InsertText(value string) (err error) {
	defer e.info("InsertText", "self", e.cssSelector, "value", value, "err", &err)
	if err = e.Focus(); err != nil {
		return err
	}
	if err = NewKeyboard(e).Insert(value); err != nil {
		return err
	}
	if err = e.dispatchEvents("input", "change"); err != nil {
		return err
	}
	err = e.Blur() // to fire change event
	return err
}

func (e Node) SetValue(value string) (err error) {
	defer e.info("SetValue", "self", e.cssSelector, "value", value, "err", &err)
	if err = e.ClearValue(); err != nil {
		return err
	}
	err = e.InsertText(value)
	return
}

func (e Node) ClearValue() (err error) {
	defer e.info("ClearValue", "self", e.cssSelector, "err", &err)
	_, err = e.eval(`function(){this.value=''}`)
	if err != nil {
		return err
	}
	if err = e.dispatchEvents("input", "change"); err != nil {
		return err
	}
	err = e.Blur()
	return err
}

func (e Node) Visible() bool {
	value, err := e.eval(`function(){return this.checkVisibility()}`)
	e.info("Visible", "self", e.cssSelector, "is_visible", value, "err", err)
	if err != nil {
		return false
	}
	return value.(bool)
}

func (e Node) Upload(files ...string) (err error) {
	err = dom.SetFileInputFiles(e, dom.SetFileInputFilesArgs{
		ObjectId: e.ObjectID(),
		Files:    files,
	})
	e.info("Upload", "self", e.cssSelector, "files", files, "err", err)
	return err
}

func (e Node) addEventListener(name string) (JsObject, error) {
	eval := fmt.Sprintf(`()=>new Promise(e=>{let t=i=>{this.removeEventListener('%s',t),e(i)};this.addEventListener('%s',t,{capture:!0})})`, name, name)
	return e.asyncEval(eval)
}

func (e Node) Click() error {
	err := e.click()
	e.info("Click", "self", e.cssSelector, "err", err)
	return err
}

func (e Node) click() (err error) {
	if err = e.ScrollIntoView(); err != nil {
		return err
	}
	point, err := e.ClickablePoint()
	if err != nil {
		return err
	}
	// layout, err := e.frame.GetLayoutMetrics()
	// if err != nil {
	// 	return err
	// }
	// nodeForLocation, err := dom.GetNodeForLocation(e, dom.GetNodeForLocationArgs{
	// 	X:                         int(point.X) + layout.CssLayoutViewport.PageX,
	// 	Y:                         int(point.Y) + layout.CssLayoutViewport.PageY,
	// 	IncludeUserAgentShadowDOM: true,
	// 	IgnorePointerEventsNone:   true,
	// })
	// if err != nil {
	// 	return err
	// }
	// if nodeForLocation.FrameId != e.frame.id {
	// 	return ErrClickOverlayFrame
	// }
	// self, err := dom.DescribeNode(e, dom.DescribeNodeArgs{
	// 	ObjectId: e.ObjectID(),
	// })
	// if err != nil {
	// 	return err
	// }
	// if nodeForLocation.BackendNodeId != self.Node.BackendNodeId {
	// 	overlay, err := dom.DescribeNode(e, dom.DescribeNodeArgs{
	// 		BackendNodeId: nodeForLocation.BackendNodeId,
	// 	})
	// 	if err != nil {
	// 		return err
	// 	}
	// 	return OverlapError{Node: overlay.Node}
	// }
	promise, err := e.addEventListener("click")
	if err != nil {
		return err
	}
	if err = e.frame.Click(point); err != nil {
		return err
	}
	_, err = e.frame.AwaitPromise(promise)
	return err
}

func (e Node) ClickablePoint() (Point, error) {
	r, err := e.GetContentQuad()
	if err != nil {
		return Point{}, err
	}
	return r.Middle(), nil
}

// Deprecated
func (e Node) GetViewportRect() (dom.Rect, error) {
	var r = dom.Rect{}
	value, err := e.eval(`function() {
		const e = this.getBoundingClientRect(), t = this.ownerDocument.documentElement.getBoundingClientRect();
		return [e.left - t.left, e.top - t.top, e.width, e.height];
	}`)
	if err != nil {
		return r, err
	}
	if tuple, ok := value.([]any); ok {
		r = dom.Rect{
			X:      tuple[0].(float64),
			Y:      tuple[1].(float64),
			Width:  tuple[2].(float64),
			Height: tuple[3].(float64),
		}
		return r, nil
	}
	return r, ErrMismatchTypeDeserialization
}

func (e Node) GetContentQuad() (Quad, error) {
	val, err := dom.GetContentQuads(e, dom.GetContentQuadsArgs{
		ObjectId: e.ObjectID(),
	})
	if err != nil {
		return nil, err
	}
	quads := convertQuads(val.Quads)
	if len(quads) == 0 { // should be at least one
		return nil, ErrElementNotVisible
	}
	for _, quad := range quads {
		if quad.Area() > 1 {
			return quad, nil
		}
	}
	return nil, ErrElementIsOutOfViewport
}

func (e Node) Hover() (err error) {
	defer e.info("Hover", "self", e.cssSelector, "err", &err)
	var p Point
	p, err = e.ClickablePoint()
	if err != nil {
		return err
	}
	err = e.frame.Hover(p)
	return err
}

func (e Node) GetComputedStyle(style string, pseudo string) (string, error) {
	var pseudoVar any = nil
	if pseudo != "" {
		pseudoVar = pseudo
	}
	value, err := e.eval(`function(p,s){return getComputedStyle(this, p)[s]}`, pseudoVar, style)
	e.info("GetComputedStyle", "self", e.cssSelector, "style", style, "pseudo", pseudo, "value", value, "err", err)
	if err != nil {
		return "", err
	}
	return value.(string), nil
}

func (nodes Nodes) Map(mapFn func(Node) (string, error)) ([]string, error) {
	var r = make([]string, len(nodes))
	var err error
	for n := range nodes {
		r[n], err = mapFn(nodes[n])
		if err != nil {
			return r, err
		}
	}
	return r, nil
}

func (nodes Nodes) First(pred func(Node) (bool, error)) Node {
	for n := range nodes {
		ok, err := pred(nodes[n])
		if err != nil {
			panic(err)
		}
		if ok {
			return nodes[n]
		}
	}
	panic(ErrIterNoResult)
}
