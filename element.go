package control

import (
	"errors"
	"fmt"
	"math"

	"github.com/retrozoid/control/protocol/dom"
)

var (
	ErrElementNotFound        = errors.New("element not found")
	ErrElementNotVisible      = errors.New("element not visible")
	ErrElementIsNotNode       = errors.New("element is not node")
	ErrElementIsOutOfViewport = errors.New("element is out of viewport")
)

type Point struct {
	X float64
	Y float64
}

// Quad quad
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

func (e OptionalNode) Query(cssSelector string) OptionalNode {
	value, err := e.eval(`function(s){return this.querySelector(s)}`, safeSelector(cssSelector))
	return toOptionalNode(value, err)
}

func (e OptionalNode) AsFrame() (Frame, error) {
	if e.Err != nil {
		return Frame{}, e.Err
	}
	value, err := dom.DescribeNode(e, dom.DescribeNodeArgs{
		ObjectId: e.ObjectID(),
	})
	if err != nil {
		return Frame{}, err
	}
	result := Frame{
		id:      value.Node.FrameId,
		session: e.Value.frame.session,
	}
	return result, nil
}

func (e OptionalNode) ScrollIntoView() error {
	if e.Err != nil {
		return e.Err
	}
	return dom.ScrollIntoViewIfNeeded(e, dom.ScrollIntoViewIfNeededArgs{ObjectId: e.ObjectID()})
}

func (e OptionalNode) GetTextContent() (string, error) {
	value, err := e.eval(`function(){return this.textContent.trim()}`)
	if err != nil {
		return "", err
	}
	return value.(string), nil
}

func (e OptionalNode) Focus() error {
	if e.Err != nil {
		return e.Err
	}
	return dom.Focus(e, dom.FocusArgs{ObjectId: e.ObjectID()})
}

func (e OptionalNode) Blur() error {
	_, err := e.eval(`function(){this.blur()}`)
	return err
}

func (e OptionalNode) InsertText(value string) (err error) {
	if err = e.Focus(); err != nil {
		return err
	}
	if err = NewKeyboard(e).Insert(value); err != nil {
		return err
	}
	return e.Blur() // to fire change event
}

func (e OptionalNode) SetValue(value string) (err error) {
	if err = e.ClearValue(); err != nil {
		return err
	}
	err = e.InsertText(value)
	return
}

func (e OptionalNode) ClearValue() error {
	_, err := e.eval(`function(){this.value=''}`)
	return err
}

func (e OptionalNode) Visible() bool {
	value, err := e.eval(`function(){return this.checkVisibility()}`)
	if err != nil {
		return false
	}
	return value.(bool)
}

func (e OptionalNode) Upload(files ...string) error {
	if e.Err != nil {
		return e.Err
	}
	return dom.SetFileInputFiles(e, dom.SetFileInputFilesArgs{
		ObjectId: e.ObjectID(),
		Files:    files,
	})
}

func (e OptionalNode) addEventListener(name string) (JsObject, error) {
	eval := fmt.Sprintf(`()=>new Promise(e=>{let t=i=>{this.removeEventListener('%s',t),e(i)};this.addEventListener('%s',t)})`, name, name)
	return e.asyncEval(eval)
}

func (e OptionalNode) Click() (err error) {
	if err = e.ScrollIntoView(); err != nil {
		return err
	}
	point, err := e.ClickablePoint()
	if err != nil {
		return err
	}
	promise, err := e.addEventListener("click")
	if err != nil {
		return err
	}
	if err = e.Value.frame.Click(point); err != nil {
		return err
	}
	_, err = e.Value.frame.awaitPromise(promise)
	return err
}

func (e OptionalNode) ClickablePoint() (Point, error) {
	r, err := e.GetContentQuad()
	if err != nil {
		return Point{}, err
	}
	return r.Middle(), nil
}

func (e OptionalNode) GetContentQuad() (Quad, error) {
	if e.Err != nil {
		return nil, e.Err
	}
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

func (e OptionalNode) Hover() (err error) {
	p, err := e.ClickablePoint()
	if err != nil {
		return err
	}
	return e.Value.frame.Hover(p)
}
