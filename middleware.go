package control

import "errors"

type Middleware interface {
	Prelude(Node) error
	Postlude(Node) error
}

type MdlMisclick struct {
	deadline int64
	JsObject
}

var (
	mdlMisclick = &MdlMisclick{deadline: 1000}
)

func (t *MdlMisclick) Prelude(n Node) (err error) {
	t.JsObject, err = n.asyncEval(`function (d) {
		let self = this;
		return new Promise((resolve, reject) => {
			let timer = setTimeout(() => reject('deadline reached'), d)
			let isTarget = e => {
				if (e.isTrusted) {
					for (let d = e.target; d; d = d.parentNode) {
						if (d === self) {
							return true
						}
					}
				}
				return false
			}
			let t = (event) => {
				clearTimeout(timer)
				if (isTarget(event)) {
					resolve()
				} else {
					event.stopPropagation()
					event.preventDefault()
					event.stopImmediatePropagation()
					reject("misclicked")
				}
			};
			window.addEventListener("click", t, { capture: true, once: true, passive: false });
		});
	}`, t.deadline)
	if err != nil {
		return errors.Join(err, errors.New("addEventListener for click failed"))
	}
	return nil
}

func (t *MdlMisclick) Postlude(n Node) error {
	_, err := n.frame.AwaitPromise(t.JsObject)
	if err != nil {
		switch err.Error() {
		// click can cause navigate with context lost
		case `Cannot find context with specified id`:
			return nil
		default:
			return err
		}
	}
	return nil
}

type MdlCurrentEntryChange struct {
	deadline int64
	JsObject
}

func (t *MdlCurrentEntryChange) Prelude(n Node) (err error) {
	t.JsObject, err = n.asyncEval(`function(d) {
		return new Promise((resolve,reject) => {
			setTimeout(reject, d, 'deadline reached')
			navigation.addEventListener("currententrychange",resolve)
		})
	}`, t.deadline)
	return err
}

func (t *MdlCurrentEntryChange) Postlude(n Node) error {
	_, err := n.frame.AwaitPromise(t.JsObject)
	return err
}
