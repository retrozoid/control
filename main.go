package control

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"time"

	"github.com/retrozoid/control/cdp"
	"github.com/retrozoid/control/chrome"
	"github.com/retrozoid/control/protocol/target"
)

func Take(args ...string) (session *Session, dfr func(), err error) {
	return TakeWithContext(context.TODO(), nil, args...)
}

func TakeWithContext(ctx context.Context, logger *log.Logger, chromeArgs ...string) (session *Session, dfr func(), err error) {
	browser, err := chrome.Launch(ctx, chromeArgs...)
	if err != nil {
		return nil, nil, errors.Join(err, errors.New("chrome launch failed"))
	}
	tab, err := browser.NewTab(http.DefaultClient, "")
	if err != nil {
		return nil, nil, errors.Join(err, errors.New("failed to open a new tab"))
	}
	transport, err := cdp.DefaultDial(ctx, browser.WebSocketUrl, logger)
	if err != nil {
		return nil, nil, errors.Join(err, errors.New("websocket dial failed"))
	}
	session, err = NewSession(transport, target.TargetID(tab.ID))
	if err != nil {
		return nil, nil, errors.Join(err, errors.New("failed to create a new session"))
	}
	teardown := func() {
		if err := transport.Close(); err != nil {
			log.Println(err)
		}
		if err = browser.WaitCloseGracefully(); err != nil {
			log.Println(err)
		}
	}
	return session, teardown, nil
}

// Future with session's context
type FutureWithDeadline[T any] interface {
	Get() (T, error)
	Cancel()
}

type deadlineFuture[T any] struct {
	context  context.Context
	deadline time.Duration
	future   cdp.Future[T]
}

func (f deadlineFuture[T]) Get() (T, error) {
	timeoutContext, cancel := context.WithTimeout(f.context, f.deadline)
	defer cancel()
	return f.future.Get(timeoutContext)
}

func (f deadlineFuture[T]) Cancel() {
	f.future.Cancel()
}

func MakeFuture[T any](s *Session, method string, filter func(T) bool) FutureWithDeadline[T] {
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
					return
				}
			}
		}
	}()
	return deadlineFuture[T]{
		context:  s.context,
		deadline: s.Timeout,
		future:   future,
	}
}
