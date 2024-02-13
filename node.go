package control

import (
	"errors"
	"fmt"
	"log/slog"
	"math"

	"github.com/retrozoid/control/protocol/dom"
	"github.com/retrozoid/control/protocol/page"
	"github.com/retrozoid/control/protocol/runtime"
)

type NoSuchSelectorError string

var ErrElementNotClickable = errors.New("node is not clickable")
var ErrNoPredicateMatch = errors.New("no predicate match")

func (s NoSuchSelectorError) Error() string {
	return fmt.Sprintf("no such selector found: `%s`", string(s))
}

type Node struct {
	JsObject
	self        *dom.Node
	cssSelector string
	frame       *Frame
	sibling     *Node
}

func (n Node) NextSibling() *Node {
	return n.sibling
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

func (e Node) OwnFrame() *Frame {
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
	_, err := e.eval(`function(l){for(const e of l)this.dispatchEvent(new Event(e,{'bubbles':!0}))}`, events)
	return err
}

func (e Node) log(msg string, args ...any) {
	args = append(args, "self", e.cssSelector)
	e.frame.Log(slog.LevelInfo, msg, args...)
}

func (e Node) HasClass(class string) Optional[bool] {
	value, err := e.eval(`function(c){return this.classList.contains(c)}`)
	return castValue[bool](value, err)
}

func (e Node) CallFunctionOn(function string, args ...any) Optional[any] {
	value, err := e.eval(function, args...)
	e.log("CallFunctionOn", "function", function, "args", args, "value", value, "err", err)
	return castValue[any](value, err)
}

func (e Node) Query(cssSelector string) Optional[*Node] {
	cssSelector = safeSelector(cssSelector)
	var (
		value, err = e.eval(`function(s){return this.querySelector(s)}`, cssSelector)
		node       = e.frame.newNode(cssSelector, value, err)
	)
	e.log("Query", "cssSelector", cssSelector, "err", node.Err())
	return node
}

func (e Node) QueryAll(cssSelector string) Optional[*Node] {
	cssSelector = safeSelector(cssSelector)
	var (
		value, err = e.eval(`function(s){return this.querySelectorAll(s)}`, cssSelector)
		node       = e.frame.newNode(cssSelector, value, err)
	)
	e.log("QueryAll", "cssSelector", cssSelector, "err", node.Err())
	return node
}

func (e Node) ToFrame() Optional[*Frame] {
	value, err := dom.DescribeNode(e, dom.DescribeNodeArgs{
		ObjectId: e.ObjectID(),
	})
	if err != nil {
		e.log("ToFrame", "err", err)
		return Optional[*Frame]{err: err}
	}
	e.log("ToFrame", "value", value.Node.FrameId, "err", err)
	frame := &Frame{
		id:          value.Node.FrameId,
		session:     e.frame.session,
		cssSelector: e.cssSelector,
	}
	e.frame.descendant = frame
	return Optional[*Frame]{value: frame}
}

func (e Node) ScrollIntoView() error {
	return dom.ScrollIntoViewIfNeeded(e, dom.ScrollIntoViewIfNeededArgs{
		ObjectId: e.ObjectID(),
	})
}

func (e Node) GetTextContent() Optional[string] {
	value, err := e.eval(`function(){return this.textContent.trim()}`)
	e.log("GetTextContent", "content", value, "err", err)
	return castValue[string](value, err)
}

func (e Node) GetValue() Optional[string] {
	value, err := e.eval(`function(){switch(this.tagName){case"INPUT":case"TEXTAREA":return this.value;case"SELECT":return Array.from(this.selectedOptions).map(b=>b.innerText).join();default:return this.innerText||this.textContent.trim();}}`)
	e.log("GetTextContent", "content", value, "err", err)
	return castValue[string](value, err)
}

func (e Node) Focus() error {
	err := dom.Focus(e, dom.FocusArgs{
		ObjectId: e.ObjectID(),
	})
	e.log("Focus", "err", err)
	return err
}

func (e Node) Blur() error {
	_, err := e.eval(`function(){this.blur()}`)
	e.log("Blur", "err", err)
	return err
}

func (e Node) InsertText(value string) error {
	err := e.insertText(value)
	e.log("InsertText", "text", value, "err", err)
	return err
}

func (e Node) insertText(value string) (err error) {
	if err = e.Focus(); err != nil {
		return err
	}
	if err = NewKeyboard(e).Insert(value); err != nil {
		return err
	}
	if err = e.dispatchEvents("input", "change"); err != nil {
		return err
	}
	return nil
}

func (e Node) SetValue(value string) (err error) {
	defer func() {
		e.log("SetValue", "value", value, "err", err)
	}()
	if err = e.ClearValue(); err != nil {
		return err
	}
	err = e.InsertText(value)
	return
}

func (e Node) ClearValue() (err error) {
	defer func() {
		e.log("ClearValue", "err", err)
	}()
	_, err = e.eval(`function(){('INPUT'===this.nodeName||'TEXTAREA'===this.nodeName)?this.value='':this.innerText=''}`)
	if err != nil {
		return err
	}
	if err = e.dispatchEvents("input", "change"); err != nil {
		return err
	}
	err = e.Blur() // to fire change event
	return err
}

func (e Node) Visibility() bool {
	value, err := e.eval(`function(){return this.checkVisibility()}`)
	e.log("Visibility", "value", value, "err", err)
	if err != nil {
		return false
	}
	return value.(bool)
}

func (e Node) Upload(files ...string) error {
	err := dom.SetFileInputFiles(e, dom.SetFileInputFilesArgs{
		ObjectId: e.ObjectID(),
		Files:    files,
	})
	e.log("Upload", "files", files, "err", err)
	return err
}

func (e Node) addEventListener(name string) (JsObject, error) {
	eval := fmt.Sprintf(`()=>new Promise(e=>{let t=i=>{this.removeEventListener('%s',t),e(i)};this.addEventListener('%s',t,{capture:!0})})`, name, name)
	return e.asyncEval(eval)
}

func (e Node) Click() error {
	err := e.click()
	e.log("Click", "err", err)
	return err
}

func (e Node) click() (err error) {
	if err = e.ScrollIntoView(); err != nil {
		return err
	}
	point, err := e.ClickablePoint().Unwrap()
	if err != nil {
		return err
	}
	// layout, err := e.frame.GetLayout().Unwrap()
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
	// 	return ErrElementNotClickable
	// }
	// self, err := dom.DescribeNode(e, dom.DescribeNodeArgs{
	// 	ObjectId: e.ObjectID(),
	// })
	// if err != nil {
	// 	return err
	// }
	// if nodeForLocation.BackendNodeId != self.Node.BackendNodeId {
	// 	// overlay, err := dom.DescribeNode(e, dom.DescribeNodeArgs{
	// 	// 	BackendNodeId: nodeForLocation.BackendNodeId,
	// 	// })
	// 	// if err != nil {
	// 	// 	return err
	// 	// }
	// 	return ErrElementNotClickable
	// }
	promise, err := e.addEventListener("click")
	if err != nil {
		return err
	}
	if err = e.frame.Click(point); err != nil {
		return err
	}
	_, err = e.frame.AwaitPromise(promise)
	if err != nil && err.Error() == `Cannot find context with specified id` {
		// click can cause navigate with context lost
		return nil
	}
	return err
}

func (e Node) ClickablePoint() Optional[Point] {
	r, err := e.getContentQuad(true)
	if err != nil {
		return Optional[Point]{err: err}
	}
	return Optional[Point]{value: r.Middle()}
}

func (e Node) Clip() Optional[page.Viewport] {
	value, err := e.eval(`function() {
		const e = this.getBoundingClientRect(), t = this.ownerDocument.documentElement.getBoundingClientRect();
		return [e.left - t.left, e.top - t.top, e.width, e.height];
	}`)
	if err != nil {
		return Optional[page.Viewport]{err: err}
	}
	if arr, ok := value.([]any); ok {
		return Optional[page.Viewport]{
			value: page.Viewport{
				X:      arr[0].(float64),
				Y:      arr[1].(float64),
				Width:  arr[2].(float64),
				Height: arr[3].(float64),
			},
		}
	}
	return Optional[page.Viewport]{err: errors.New("clip: eval result is not array")}
}

func (e Node) getContentQuad(viewportCorrection bool) (Quad, error) {
	val, err := dom.GetContentQuads(e, dom.GetContentQuadsArgs{
		ObjectId: e.ObjectID(),
	})
	if err != nil {
		return nil, err
	}
	quads := convertQuads(val.Quads)
	if len(quads) == 0 { // should be at least one
		return nil, errors.New("node has no visible bounds")
	}
	layout, err := e.frame.GetLayout().Unwrap()
	if err != nil {
		return nil, err
	}
	for _, quad := range quads {
		/* correction is get sub-quad of element that in viewport
		 _______________  <- Viewport top
		|  1 _______ 2  |
		|   |visible|   | visible part of element
		|__4|visible|3__| <- Viewport bottom
		|   |invisib|   | this invisible part of element omits if viewportCorrection
		|...............|
		*/
		if viewportCorrection {
			for i := 0; i < len(quad); i++ {
				quad[i].X = math.Min(math.Max(quad[i].X, 0), float64(layout.CssLayoutViewport.ClientWidth))
				quad[i].Y = math.Min(math.Max(quad[i].Y, 0), float64(layout.CssLayoutViewport.ClientHeight))
			}
		}
		if quad.Area() > 1 {
			return quad, nil
		}
	}
	return nil, errors.New("node is out of viewport")
}

func (e Node) Hover() (err error) {
	p, err := e.ClickablePoint().Unwrap()
	defer func() {
		e.log("Hover", "err", err)
	}()
	if err != nil {
		return err
	}
	err = e.frame.Hover(p)
	return err
}

func (e Node) GetComputedStyle(style string, pseudo string) Optional[string] {
	var pseudoVar any = nil
	if pseudo != "" {
		pseudoVar = pseudo
	}
	value, err := e.eval(`function(p,s){return getComputedStyle(this, p)[s]}`, pseudoVar, style)
	e.log("GetComputedStyle", "style", style, "pseudo", pseudo, "value", value, "err", err)
	return castValue[string](value, err)
}

func (e Node) SetAttribute(attr, value string) error {
	_, err := e.eval(`function(a,v){this.setAttribute(a,v)}`, attr, value)
	e.log("SetAttribute", "attr", attr, "attr_value", value, "err", err)
	return err
}

func (e Node) GetAttribute(attr string) Optional[string] {
	value, err := e.eval(`function(a){return this.getAttribute(a)}`, attr)
	e.log("GetAttribute", "attr", attr, "value", value, "err", err)
	return castValue[string](value, err)
}

func (e Node) GetRectangle() Optional[dom.Rect] {
	q, err := e.getContentQuad(false)
	e.log("GetRectangle", "quad", q, "err", err)
	if err != nil {
		return Optional[dom.Rect]{err: err}
	}
	rect := dom.Rect{
		X:      q[0].X,
		Y:      q[0].Y,
		Width:  q[1].X - q[0].X,
		Height: q[3].Y - q[0].Y,
	}
	return Optional[dom.Rect]{value: rect}
}

func (e Node) SelectByValues(values ...string) (err error) {
	defer func() {
		e.log("SelectByValues", "values", values, "err", err)
	}()
	_, err = e.eval(`function(a){const b=Array.from(this.options);this.value=void 0;for(const c of b)if(c.selected=a.includes(c.value),c.selected&&!this.multiple)break}`, values)
	if err != nil {
		return err
	}
	err = e.dispatchEvents("click", "input", "change")
	return err
}

func (e Node) SelectByTexts(values ...string) error {
	// todo
	return nil
}

func (e Node) GetSelected(textContent bool) Optional[[]string] {
	values, err := e.eval(`function(text){return Array.from(this.options).filter(a=>a.selected).map(a=>text?a.textContent.trim():a.value)}`, textContent)
	e.log("GetSelected", "returnTextContent", textContent, "returnAttributeValue", !textContent, "values", values, "err", err)
	if err != nil {
		return Optional[[]string]{err: err}
	}
	stringsValues := make([]string, len(values.([]any)))
	for n, val := range values.([]any) {
		stringsValues[n] = val.(string)
	}
	return Optional[[]string]{value: stringsValues}
}

func (e Node) SetCheckbox(check bool) (err error) {
	defer func() {
		e.log("SetCheckbox", "check", check, "err", err)
	}()
	_, err = e.eval(`function(v){this.checked=v}`, check)
	if err != nil {
		return err
	}
	err = e.dispatchEvents("click", "input", "change")
	return err
}

func (e Node) IsChecked() Optional[bool] {
	value, err := e.eval(`function(){return this.checked}`)
	e.log("IsChecked", "value", value, "err", err)
	return castValue[bool](value, err)
}

func (node *Node) Map(mapFn func(*Node) (string, error)) ([]string, error) {
	var r []string
	for p := node; p != nil; p = p.NextSibling() {
		val, err := mapFn(p)
		if err != nil {
			return r, err
		}
		r = append(r, val)
	}
	return r, nil
}

func (node *Node) Foreach(predicate func(*Node) error) error {
	for p := node; p != nil; p = p.NextSibling() {
		if err := predicate(p); err != nil {
			return err
		}
	}
	return nil
}

func (node *Node) First(predicate func(*Node) (bool, error)) Optional[*Node] {
	for p := node; p != nil; p = p.NextSibling() {
		val, err := predicate(p)
		if err != nil {
			return Optional[*Node]{err: err}
		}
		if val {
			return Optional[*Node]{value: p}
		}
	}
	return Optional[*Node]{err: ErrNoPredicateMatch}
}