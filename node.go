package control

import (
	"errors"
	"fmt"
	"log/slog"
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
	ErrTargetNotClickable = errors.New("target is not clickable")
	ErrTargetNotVisible   = errors.New("target is not visible")
	ErrTargetNotStable    = errors.New("target is not stable")
	ErrNoPredicateMatch   = errors.New("no predicate match")
)

func (s NoSuchSelectorError) Error() string {
	return fmt.Sprintf("no such selector found: `%s`", string(s))
}

func (s TargetOverlappedError) Error() string {
	return fmt.Sprintf("target is overlapped by: `%s`", string(s))
}

type Node struct {
	JsObject
	cssSelector string
	frame       *Frame
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
		ObjectId: e.ObjectID(),
	})
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
	if v, ok := value.(JsObject); ok {
		return v, nil
	}
	return nil, fmt.Errorf("interface conversion failed, `%+v` not JsObject", value)
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
	return optional[bool](value, err)
}

func (e Node) CallFunctionOn(function string, args ...any) Optional[any] {
	value, err := e.eval(function, args...)
	e.log("CallFunctionOn", "function", function, "args", args, "value", value, "err", err)
	return optional[any](value, err)
}

func (e Node) Query(cssSelector string) Optional[*Node] {
	value, err := e.eval(`function(s){return this.querySelector(s)}`, cssSelector)
	opt := optional[*Node](value, err)
	if opt.err == nil && opt.value == nil {
		opt.err = NoSuchSelectorError(cssSelector)
	}
	if opt.value != nil {
		if DebugHighlightEnabled {
			_ = opt.value.Highlight()
		}
		opt.value.cssSelector = cssSelector
	}
	e.log("Query", "cssSelector", cssSelector, "err", opt.err)
	return opt
}

func (e Node) QueryAll(cssSelector string) Optional[*NodeList] {
	value, err := e.eval(`function(s){return this.querySelectorAll(s)}`, cssSelector)
	opt := optional[*NodeList](value, err)
	if opt.err == nil && opt.value == nil {
		opt.err = NoSuchSelectorError(cssSelector)
	}
	e.log("QueryAll", "cssSelector", cssSelector, "err", opt.err)
	return opt
}

func (e Node) ContentFrame() Optional[*Frame] {
	value, err := e.frame.describeNode(e)
	if err != nil {
		e.log("ContentFrame", "err", err)
		return Optional[*Frame]{err: err}
	}
	frame := &Frame{
		id:          value.FrameId,
		session:     e.frame.session,
		cssSelector: e.cssSelector,
		parent:      e.frame,
	}
	e.log("ContentFrame", "value", value.FrameId, "err", err)
	return Optional[*Frame]{value: frame}
}

func (e Node) ScrollIntoView() error {
	return dom.ScrollIntoViewIfNeeded(e, dom.ScrollIntoViewIfNeededArgs{
		ObjectId: e.ObjectID(),
	})
}

func (e Node) GetRawTextContent() Optional[string] {
	value, err := e.eval(`function(){return this.textContent.trim()}`)
	e.log("GetRawTextContent", "content", value, "err", err)
	return optional[string](value, err)
}

func (e Node) GetText() Optional[string] {
	value, err := e.eval(`function(){return ('INPUT'===this.nodeName||'TEXTAREA'===this.nodeName)?this.value:this.innerText}`)
	e.log("GetText", "content", value, "err", err)
	return optional[string](value, err)
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
	return err
}

func (e Node) clearInput(kb Keyboard) error {
	_, err := e.eval(`function(){('INPUT'===this.nodeName||'TEXTAREA'===this.nodeName)?this.select():this.innerText=''}`)
	if err != nil {
		return err
	}
	return kb.Press(key.Keys[key.Backspace], time.Millisecond*41)
}

func (e Node) InsertText(value string) error {
	err := e.setText(value, false)
	e.log("InsertText", "text", value, "err", err)
	return err
}

func (e Node) SetText(value string) error {
	err := e.setText(value, true)
	e.log("SetText", "value", value, "err", err)
	return err
}

func (e Node) setText(value string, clearBefore bool) (err error) {
	if err = e.Focus(); err != nil {
		return err
	}
	kb := NewKeyboard(e)
	if clearBefore {
		if err = e.clearInput(kb); err != nil {
			return err
		}
	}
	if err = kb.Insert(value); err != nil {
		return err
	}
	return nil
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
	// onClick, err := e.asyncEval(`function(d){return new Promise((e,j)=>{let t=i=>{this.removeEventListener('click',t),e(i)};this.addEventListener('click',t,{capture:!0});setTimeout(j,d);})}`, 1000)
	/*
		function(d) {
			let t = this
			return new Promise((a, b) => {
				let c = { capture: !0, once: !1 }
				let g = (i) => {
					for (let d = i; d; d = d.parentNode) {
						if (d === t) {
							return !0
						}
					}
					return !1
				}
				let f = (e) => {
					if (e.isTrusted && g(e.target)) {
						a()
						document.removeEventListener("click", f, c);
					} else {
						e.stopPropagation()
						e.preventDefault()
						b((b.target.outerHTML || "").substring(0, 256))
					}
				}
				document.addEventListener("click", f, c);
				setTimeout(b, d);
			})
		}
	*/
	onClick, err := e.asyncEval(`function(e){let t=this;return new Promise(((r,n)=>{let o={capture:!0,once:!1},i=e=>{e.isTrusted&&(e=>{for(let r=e;r;r=r.parentNode)if(r===t)return!0;return!1})(e.target)?(r(),document.removeEventListener("click",i,o)):(e.stopPropagation(),e.preventDefault(),n((n.target.outerHTML||"").substring(0,256)))};document.addEventListener("click",i,o),setTimeout(n,e)}))}`, 1000)
	if err != nil {
		return errors.Join(err, errors.New("addEventListener for click failed"))
	}
	if err = e.frame.Click(point); err != nil {
		return err
	}
	_, err = e.frame.AwaitPromise(onClick)
	if err != nil {
		switch err.Error() {
		// click can cause navigate with context lost
		case `Cannot find context with specified id`:
			return nil
		case "":
			return ErrTargetNotClickable
		default:
			return TargetOverlappedError(err.Error())
		}
	}
	return err
}

func (e Node) ClickablePoint() Optional[Point] {
	if !e.Visibility() {
		return Optional[Point]{err: ErrTargetNotVisible}
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
	return Optional[Point]{err: ErrTargetNotStable}
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

func (e Node) Hover() error {
	err := e.hover()
	e.log("Hover", "err", err)
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
	value, err := e.eval(`function(p,s){return getComputedStyle(this, p)[s]}`, pseudoVar, style)
	e.log("GetComputedStyle", "style", style, "pseudo", pseudo, "value", value, "err", err)
	return optional[string](value, err)
}

func (e Node) SetAttribute(attr, value string) error {
	_, err := e.eval(`function(a,v){this.setAttribute(a,v)}`, attr, value)
	e.log("SetAttribute", "attr", attr, "attr_value", value, "err", err)
	return err
}

func (e Node) GetAttribute(attr string) Optional[string] {
	value, err := e.eval(`function(a){return this.getAttribute(a)}`, attr)
	e.log("GetAttribute", "attr", attr, "value", value, "err", err)
	return optional[string](value, err)
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
	return optional[bool](value, err)
}

func (nl NodeList) Map(mapFn func(*Node) (string, error)) ([]string, error) {
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
