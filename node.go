package control

import (
	"errors"
	"fmt"
	"math"
	"time"

	"github.com/retrozoid/control/key"
	"github.com/retrozoid/control/protocol/dom"
	"github.com/retrozoid/control/protocol/overlay"
	"github.com/retrozoid/control/protocol/page"
	"github.com/retrozoid/control/protocol/runtime"
)

type NoSuchSelectorError string
type TargetOverlappedError string

var (
	ErrElementUnclickable = errors.New("element is not clickable")
	ErrElementUnvisible   = errors.New("element is not visible")
	ErrElementUnstable    = errors.New("element is not stable")
	ErrElementDetached    = errors.New("element is detached")
	ErrNoPredicateMatch   = errors.New("no predicate match")
)

func (s NoSuchSelectorError) Error() string {
	return fmt.Sprintf("no such selector found: `%s`", string(s))
}

func (s TargetOverlappedError) Error() string {
	return fmt.Sprintf("target is overlapped by: `%s`", string(s))
}

type Node struct {
	object            RemoteObject
	requestedSelector string
	frame             *Frame
}

type NodeList struct {
	Nodes []*Node
}

type Point struct {
	X float64
	Y float64
}

type Quad []Point

func (p Point) Equal(a Point) bool {
	return p.X == a.X && p.Y == a.Y
}

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

func (e Node) Highlight() error {
	return overlay.HighlightNode(e.frame, overlay.HighlightNodeArgs{
		HighlightConfig: &overlay.HighlightConfig{
			GridHighlightConfig: &overlay.GridHighlightConfig{
				RowGapColor:      &dom.RGBA{R: 127, G: 32, B: 210, A: 0.3},
				RowHatchColor:    &dom.RGBA{R: 127, G: 32, B: 210, A: 0.8},
				ColumnGapColor:   &dom.RGBA{R: 127, G: 32, B: 210, A: 0.3},
				ColumnHatchColor: &dom.RGBA{R: 127, G: 32, B: 210, A: 0.8},
				RowLineColor:     &dom.RGBA{R: 127, G: 32, B: 210},
				ColumnLineColor:  &dom.RGBA{R: 127, G: 32, B: 210},
				RowLineDash:      true,
				ColumnLineDash:   true,
			},
			FlexContainerHighlightConfig: &overlay.FlexContainerHighlightConfig{
				ContainerBorder: &overlay.LineStyle{
					Color:   &dom.RGBA{R: 127, G: 32, B: 210},
					Pattern: "dashed",
				},
				ItemSeparator: &overlay.LineStyle{
					Color:   &dom.RGBA{R: 127, G: 32, B: 210},
					Pattern: "dashed",
				},
				LineSeparator: &overlay.LineStyle{
					Color:   &dom.RGBA{R: 127, G: 32, B: 210},
					Pattern: "dashed",
				},
				MainDistributedSpace: &overlay.BoxStyle{
					HatchColor: &dom.RGBA{R: 127, G: 32, B: 210, A: 0.8},
					FillColor:  &dom.RGBA{R: 127, G: 32, B: 210, A: 0.3},
				},
				CrossDistributedSpace: &overlay.BoxStyle{
					HatchColor: &dom.RGBA{R: 127, G: 32, B: 210, A: 0.8},
					FillColor:  &dom.RGBA{R: 127, G: 32, B: 210, A: 0.3},
				},
				RowGapSpace: &overlay.BoxStyle{
					HatchColor: &dom.RGBA{R: 127, G: 32, B: 210, A: 0.8},
					FillColor:  &dom.RGBA{R: 127, G: 32, B: 210, A: 0.3},
				},
				ColumnGapSpace: &overlay.BoxStyle{
					HatchColor: &dom.RGBA{R: 127, G: 32, B: 210, A: 0.8},
					FillColor:  &dom.RGBA{R: 127, G: 32, B: 210, A: 0.3},
				},
			},
			FlexItemHighlightConfig: &overlay.FlexItemHighlightConfig{
				BaseSizeBox: &overlay.BoxStyle{
					HatchColor: &dom.RGBA{R: 127, G: 32, B: 210, A: 0.8},
				},
				BaseSizeBorder: &overlay.LineStyle{
					Color:   &dom.RGBA{R: 127, G: 32, B: 210},
					Pattern: "dotted",
				},
				FlexibilityArrow: &overlay.LineStyle{
					Color: &dom.RGBA{R: 127, G: 32, B: 210},
				},
			},
			ContrastAlgorithm: overlay.ContrastAlgorithm("aa"),
			ContentColor:      &dom.RGBA{R: 111, G: 168, B: 220, A: 0.66},
			PaddingColor:      &dom.RGBA{R: 147, G: 196, B: 125, A: 0.55},
			BorderColor:       &dom.RGBA{R: 255, G: 229, B: 153, A: 0.66},
			MarginColor:       &dom.RGBA{R: 246, G: 178, B: 107, A: 0.66},
			EventTargetColor:  &dom.RGBA{R: 255, G: 196, B: 196, A: 0.66},
			ShapeColor:        &dom.RGBA{R: 96, G: 82, B: 177, A: 0.8},
			ShapeMarginColor:  &dom.RGBA{R: 96, G: 82, B: 127, A: 0.6},
		},
		ObjectId: e.GetRemoteObjectID(),
	})
}

func (e Node) GetRemoteObjectID() runtime.RemoteObjectId {
	return e.object.GetRemoteObjectID()
}

func (e Node) OwnFrame() *Frame {
	return e.frame
}

func (e Node) Call(method string, send, recv interface{}) error {
	return e.frame.Call(method, send, recv)
}

func (e Node) IsConnected() bool {
	value, err := e.eval(`function(){return this.isConnected}`)
	if err != nil {
		return false
	}
	return value.(bool)
}

func (e Node) ReleaseObject() error {
	err := runtime.ReleaseObject(e, runtime.ReleaseObjectArgs{ObjectId: e.GetRemoteObjectID()})
	if err != nil && err.Error() == `Cannot find context with specified id` {
		return nil
	}
	return err
}

func (e Node) eval(function string, args ...any) (any, error) {
	return e.frame.callFunctionOn(e, function, true, args...)
}

func (e Node) asyncEval(function string, args ...any) (RemoteObject, error) {
	value, err := e.frame.callFunctionOn(e, function, false, args...)
	if err != nil {
		return nil, err
	}
	if v, ok := value.(RemoteObject); ok {
		return v, nil
	}
	return nil, fmt.Errorf("interface conversion failed, `%+v` not JsObject", value)
}

func (e Node) dispatchEvents(events ...any) error {
	_, err := e.eval(`function(l){for(const e of l)this.dispatchEvent(new Event(e,{'bubbles':!0}))}`, events)
	return err
}

func (e Node) log(t time.Time, msg string, args ...any) {
	args = append(args, "self", e.requestedSelector)
	e.frame.Log(t, msg, args...)
}

func (e Node) HasClass(class string) Optional[bool] {
	t := time.Now()
	value, err := e.eval(`function(c){return this.classList.contains(c)}`)
	e.log(t, "HasClass", "class", class, "value", value, "err", err)
	return optional[bool](value, err)
}

func (e Node) CallFunctionOn(function string, args ...any) Optional[any] {
	t := time.Now()
	value, err := e.eval(function, args...)
	e.log(t, "CallFunctionOn", "function", function, "args", args, "value", value, "err", err)
	return optional[any](value, err)
}

func (e Node) AsyncCallFunctionOn(function string, args ...any) Optional[RemoteObject] {
	t := time.Now()
	value, err := e.asyncEval(function, args...)
	e.log(t, "AsyncCallFunctionOn", "function", function, "args", args, "value", value, "err", err)
	return optional[RemoteObject](value, err)
}

func (e Node) Query(cssSelector string) Optional[*Node] {
	t := time.Now()
	value, err := e.eval(`function(s){return this.querySelector(s)}`, cssSelector)
	opt := optional[*Node](value, err)
	if opt.err == nil && opt.value == nil {
		opt.err = NoSuchSelectorError(cssSelector)
	}
	if opt.value != nil {
		if e.frame.session.highlightEnabled {
			_ = opt.value.Highlight()
		}
		opt.value.requestedSelector = cssSelector
	}
	e.log(t, "Query", "cssSelector", cssSelector, "err", opt.err)
	return opt
}

func (e Node) QueryAll(cssSelector string) Optional[*NodeList] {
	t := time.Now()
	value, err := e.eval(`function(s){return this.querySelectorAll(s)}`, cssSelector)
	opt := optional[*NodeList](value, err)
	if opt.err == nil && opt.value == nil {
		opt.err = NoSuchSelectorError(cssSelector)
	}
	e.log(t, "QueryAll", "cssSelector", cssSelector, "err", opt.err)
	return opt
}

func (e Node) ContentFrame() Optional[*Frame] {
	t := time.Now()
	opt := optional[*Frame](e.contentFrame())
	e.log(t, "ContentFrame", "value", opt.value, "err", opt.err)
	return opt
}

func (e Node) contentFrame() (*Frame, error) {
	value, err := e.frame.describeNode(e)
	if err != nil {
		return nil, err
	}
	return &Frame{
		id:          value.FrameId,
		session:     e.frame.session,
		cssSelector: e.requestedSelector,
		parent:      e.frame,
	}, nil
}

func (e Node) ScrollIntoView() error {
	return dom.ScrollIntoViewIfNeeded(e, dom.ScrollIntoViewIfNeededArgs{
		ObjectId: e.GetRemoteObjectID(),
	})
}

func (e Node) GetText() Optional[string] {
	t := time.Now()
	value, err := e.eval(`function(){return ('INPUT'===this.nodeName||'TEXTAREA'===this.nodeName)?this.value:this.innerText}`)
	e.log(t, "GetText", "content", value, "err", err)
	return optional[string](value, err)
}

func (e Node) Focus() error {
	return dom.Focus(e, dom.FocusArgs{
		ObjectId: e.GetRemoteObjectID(),
	})
}

func (e Node) Blur() error {
	_, err := e.eval(`function(){this.blur()}`)
	return err
}

func (e Node) clearInput() error {
	_, err := e.eval(`function(){('INPUT'===this.nodeName||'TEXTAREA'===this.nodeName)?this.select():this.innerText=''}`)
	if err != nil {
		return err
	}
	return e.frame.session.kb.Press(key.Keys[key.Backspace], time.Millisecond*85)
}

func (e Node) InsertText(value string) error {
	t := time.Now()
	err := e.setText(value, false)
	e.log(t, "InsertText", "text", value, "err", err)
	return err
}

func (e Node) SetText(value string) error {
	t := time.Now()
	err := e.setText(value, true)
	e.log(t, "SetText", "value", value, "err", err)
	return err
}

func (e Node) setText(value string, clearBefore bool) (err error) {
	if err = e.Focus(); err != nil {
		return err
	}
	if clearBefore {
		if err = e.clearInput(); err != nil {
			return err
		}
	}
	if err = e.frame.session.kb.Insert(value); err != nil {
		return err
	}
	return nil
}

func (e Node) checkVisibility() bool {
	value, err := e.eval(`function(){return this.checkVisibility({
		opacityProperty: false,
		visibilityProperty: true,
	  })}`)
	if err != nil {
		return false
	}
	return value.(bool)
}

func (e Node) Visibility() bool {
	t := time.Now()
	value := e.checkVisibility()
	e.log(t, "Visibility", "value", value)
	return value
}

func (e Node) Upload(files ...string) error {
	t := time.Now()
	err := dom.SetFileInputFiles(e, dom.SetFileInputFilesArgs{
		ObjectId: e.GetRemoteObjectID(),
		Files:    files,
	})
	e.log(t, "Upload", "files", files, "err", err)
	return err
}

func (e Node) Click() error {
	return e.ClickFor(ClickPreventMisclick)
}

func (e Node) ClickFor(middle NodeMiddleware) error {
	t := time.Now()
	err := e.click(middle)
	e.log(t, "Click", "err", err)
	return err
}

func (e Node) click(middle NodeMiddleware) (err error) {
	if err = e.ScrollIntoView(); err != nil {
		return err
	}
	point, err := e.ClickablePoint().Unwrap()
	if err != nil {
		return err
	}
	if err = middle.Prelude(e); err != nil {
		return err
	}
	if err = e.frame.Click(point); err != nil {
		return err
	}
	return middle.Postlude(e)
}

func (e Node) ClickablePoint() Optional[Point] {
	if !e.checkVisibility() {
		return Optional[Point]{err: ErrElementUnvisible}
	}
	var (
		r0, r1 Quad
		err    error
	)
	r0, err = e.getContentQuad(true)
	if err != nil {
		return Optional[Point]{err: err}
	}
	_, err = e.frame.evaluate(`new Promise(requestAnimationFrame)`, true)
	if err != nil {
		return Optional[Point]{err: err}
	}
	r1, err = e.getContentQuad(true)
	if err != nil {
		return Optional[Point]{err: err}
	}
	if r0.Middle().Equal(r1.Middle()) {
		return Optional[Point]{value: r0.Middle()}
	}
	return Optional[Point]{err: ErrElementUnstable}
}

func (e Node) Clip() Optional[page.Viewport] {
	t := time.Now()
	opt := optional[page.Viewport](e.clip())
	e.log(t, "Clip", "value", opt.value, "err", opt.err)
	return opt
}

func (e Node) clip() (page.Viewport, error) {
	value, err := e.eval(`function() {
		const e = this.getBoundingClientRect(), t = this.ownerDocument.documentElement.getBoundingClientRect();
		return [e.left - t.left, e.top - t.top, e.width, e.height, window.devicePixelRatio];
	}`)
	if err != nil {
		return page.Viewport{}, err
	}
	if arr, ok := value.([]any); ok {
		return page.Viewport{
			X:      arr[0].(float64),
			Y:      arr[1].(float64),
			Width:  arr[2].(float64),
			Height: arr[3].(float64),
			Scale:  arr[4].(float64),
		}, nil
	}
	return page.Viewport{}, errors.New("clip: eval result is not array")
}

func (e Node) getContentQuad(viewportCorrection bool) (Quad, error) {
	val, err := dom.GetContentQuads(e, dom.GetContentQuadsArgs{
		ObjectId: e.GetRemoteObjectID(),
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

func (e Node) Hover() error {
	t := time.Now()
	err := e.hover()
	e.log(t, "Hover", "err", err)
	return err
}

func (e Node) hover() error {
	if err := e.ScrollIntoView(); err != nil {
		return err
	}
	p, err := e.ClickablePoint().Unwrap()
	if err != nil {
		return err
	}
	return e.frame.Hover(p)
}

func (e Node) GetComputedStyle(style string, pseudo string) Optional[string] {
	var pseudoVar any = nil
	if pseudo != "" {
		pseudoVar = pseudo
	}
	t := time.Now()
	value, err := e.eval(`function(p,s){return getComputedStyle(this, p)[s]}`, pseudoVar, style)
	e.log(t, "GetComputedStyle", "style", style, "pseudo", pseudo, "value", value, "err", err)
	return optional[string](value, err)
}

func (e Node) SetAttribute(attr, value string) error {
	t := time.Now()
	_, err := e.eval(`function(a,v){this.setAttribute(a,v)}`, attr, value)
	e.log(t, "SetAttribute", "attr", attr, "attr_value", value, "err", err)
	return err
}

func (e Node) GetAttribute(attr string) Optional[string] {
	t := time.Now()
	value, err := e.eval(`function(a){return this.getAttribute(a)}`, attr)
	e.log(t, "GetAttribute", "attr", attr, "value", value, "err", err)
	return optional[string](value, err)
}

func (e Node) GetRectangle() Optional[dom.Rect] {
	t := time.Now()
	q, err := e.getContentQuad(false)
	e.log(t, "GetRectangle", "quad", q, "err", err)
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

func (e Node) SelectByValues(values ...string) error {
	t := time.Now()
	err := e.selectByValues(values...)
	e.log(t, "SelectByValues", "values", values, "err", err)
	return err
}

func (e Node) selectByValues(values ...string) error {
	_, err := e.eval(`function(a){const b=Array.from(this.options);this.value=void 0;for(const c of b)if(c.selected=a.includes(c.value),c.selected&&!this.multiple)break}`, values)
	if err != nil {
		return err
	}
	return e.dispatchEvents("click", "input", "change")
}

func (e Node) SelectByTexts(values ...string) error {
	// todo
	panic("SelectByTexts not implemented")
}

func (e Node) GetSelected(textContent bool) Optional[[]string] {
	t := time.Now()
	opt := optional[[]string](e.getSelected(textContent))
	e.log(t, "GetSelected", "returnTextContent", textContent, "returnAttributeValue", !textContent, "values", opt.value, "err", opt.err)
	return opt
}

func (e Node) getSelected(textContent bool) ([]string, error) {
	values, err := e.eval(`function(text){return Array.from(this.options).filter(a=>a.selected).map(a=>text?a.textContent.trim():a.value)}`, textContent)
	if err != nil {
		return nil, err
	}
	stringsValues := make([]string, len(values.([]any)))
	for n, val := range values.([]any) {
		stringsValues[n] = val.(string)
	}
	return stringsValues, nil
}

func (e Node) SetCheckbox(check bool) error {
	t := time.Now()
	err := e.setCheckbox(check)
	e.log(t, "SetCheckbox", "check", check, "err", err)
	return err
}

func (e Node) setCheckbox(check bool) error {
	_, err := e.eval(`function(v){this.checked=v}`, check)
	if err != nil {
		return err
	}
	return e.dispatchEvents("click", "input", "change")
}

func (e Node) IsChecked() Optional[bool] {
	t := time.Now()
	value, err := e.eval(`function(){return this.checked}`)
	e.log(t, "IsChecked", "value", value, "err", err)
	return optional[bool](value, err)
}

func (nl NodeList) MapToString(mapFn func(*Node) (string, error)) ([]string, error) {
	var r []string
	for _, node := range nl.Nodes {
		val, err := mapFn(node)
		if err != nil {
			return r, err
		}
		r = append(r, val)
	}
	return r, nil
}

func (nl NodeList) Foreach(predicate func(*Node) error) error {
	for _, node := range nl.Nodes {
		if err := predicate(node); err != nil {
			return err
		}
	}
	return nil
}

func (nl NodeList) First(predicate func(*Node) (bool, error)) Optional[*Node] {
	for _, node := range nl.Nodes {
		val, err := predicate(node)
		if err != nil {
			return Optional[*Node]{err: err}
		}
		if val {
			return Optional[*Node]{value: node}
		}
	}
	return Optional[*Node]{err: ErrNoPredicateMatch}
}
