package cdp

import (
	"context"
	"errors"
	"sync"
)

var ErrPromiseCanceled = errors.New("promise canceled")

type (
	ResponseFuture  = Future[Response]
	responsePromise = Promise[Response]
)

type Future[T any] interface {
	Get(context.Context) (T, error)
	Cancel()
}

type Promise[T any] interface {
	Resolve(T)
	Reject(error)
}

func MakePromise[T any](onClose func()) (Promise[T], Future[T]) {
	value := &promise[T]{
		fulfilled: make(chan struct{}, 1),
		onClose:   onClose,
	}
	return value, value
}

type promise[T any] struct {
	once      sync.Once
	fulfilled chan struct{}
	value     T
	err       error
	onClose   func()
}

func (u *promise[T]) Get(parent context.Context) (T, error) {
	defer u.Cancel()
	select {
	case <-parent.Done():
		return u.value, context.Cause(parent)
	case <-u.fulfilled:
		return u.value, u.err
	}
}

func (u *promise[T]) Cancel() {
	u.Reject(ErrPromiseCanceled)
}

func (u *promise[T]) Resolve(value T) {
	u.once.Do(func() {
		u.value = value
		close(u.fulfilled)
		u.onClose()
	})
}

func (u *promise[T]) Reject(err error) {
	u.once.Do(func() {
		u.err = err
		close(u.fulfilled)
		u.onClose()
	})
}
