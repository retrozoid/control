package control

import (
	"encoding/json"

	"github.com/ecwid/control/cdp"
)

func MakeFuture[T any](s *Session, method string, filter func(T) bool) cdp.Future[T] {
	channel, cancel := s.Subscribe()
	promise, future := cdp.MakePromise[T](cancel)
	go func() {
		for value := range channel {
			if value.Method == method {
				var result T
				err := json.Unmarshal(value.Params, &result)
				if err != nil {
					promise.Reject(err)
					return
				}
				if filter(result) {
					promise.Resolve(result)
				}
				return
			}
		}
	}()
	return future
}
