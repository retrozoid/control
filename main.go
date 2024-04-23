package control

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/retrozoid/control/cdp"
	"github.com/retrozoid/control/chrome"
	"github.com/retrozoid/control/protocol/target"
)

func Take(args ...string) (session *Session, dfr func(), err error) {
	return TakeWithContext(context.TODO(), nil, args...)
}

func TakeWithContext(ctx context.Context, logger *slog.Logger, chromeArgs ...string) (session *Session, dfr func(), err error) {
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
			transport.Log(slog.LevelError, "can't close transport", "err", err)
		}
		if err = browser.WaitCloseGracefully(); err != nil {
			transport.Log(slog.LevelError, "can't close browser gracefully", "err", err)
		}
	}
	return session, teardown, nil
}

// Future with session's context
type Future[T any] interface {
	Get() (T, error)
	Cancel()
}

func WithSessionContext[T any](session *Session, future cdp.Future[T]) Future[T] {
	return sessionContextFuture[T]{
		context:  session.context,
		deadline: session.timeout,
		future:   future,
	}
}

type sessionContextFuture[T any] struct {
	context  context.Context
	deadline time.Duration
	future   cdp.Future[T]
}

func (f sessionContextFuture[T]) Get() (T, error) {
	withTimeout, cancel := context.WithTimeout(f.context, f.deadline)
	defer cancel()
	return f.future.Get(withTimeout)
}

func (f sessionContextFuture[T]) Cancel() {
	f.future.Cancel()
}

func Subscribe[T any](s *Session, method string, filter func(T) bool) Future[T] {
	var (
		channel, cancel = s.Subscribe()
	)
	future := cdp.NewPromise(func(resolve func(T), reject func(error)) {
		defer cancel()
		for value := range channel {
			if value.Method == method {
				var result T
				err := json.Unmarshal(value.Params, &result)
				if err != nil {
					reject(err)
					return
				}
				if filter(result) {
					resolve(result)
					return
				}
			}
		}
	})
	return WithSessionContext(s, future.Finally(cancel))
}
