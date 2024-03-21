package control

import (
	"errors"
	"time"
)

var (
	ClickPreventMisclick       = &MiddlewarePreventMisclick{deadline: time.Second.Milliseconds()}
	ClickForCurrentEntryChange = &MiddlewareCurrentEntryChange{deadline: time.Second.Milliseconds()}
)

type NodeMiddleware interface {
	Prelude(Node) error
	Postlude(Node) error
}

type MiddlewarePreventMisclick struct {
	deadline int64
	promise  RemoteObject
}

func (t *MiddlewarePreventMisclick) Prelude(n Node) (err error) {
	t.promise, err = n.asyncEval(`function (d) {
		let self = this;
		return new Promise((resolve, reject) => {
			// let timer = setTimeout(() => self.isConnected ? reject('deadline reached') : resolve(), d)
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
				// clearTimeout(timer)
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

func (t *MiddlewarePreventMisclick) Postlude(n Node) error {
	_, err := n.frame.AwaitPromise(t.promise)
	if err != nil {
		// click can cause navigate with context lost
		if err.Error() == `Cannot find context with specified id` {
			return nil
		}
	}
	return err
}

type MiddlewareCurrentEntryChange struct {
	deadline int64
	promise  RemoteObject
}

func (t *MiddlewareCurrentEntryChange) Prelude(n Node) (err error) {
	t.promise, err = n.asyncEval(`function(d) {
		return new Promise((resolve,reject) => {
			setTimeout(reject, d, 'deadline reached')
			navigation.addEventListener("currententrychange", resolve)
		})
	}`, t.deadline)
	return err
}

func (t *MiddlewareCurrentEntryChange) Postlude(n Node) error {
	_, err := n.frame.AwaitPromise(t.promise)
	return err
}
