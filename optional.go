package control

import (
	"errors"
	"reflect"
)

type Optional[T any] struct {
	value T
	err   error
}

func castTo[T any](value any, err error) (T, error) {
	var t T
	if err != nil {
		return t, err
	}
	if value == nil {
		return t, nil
	}
	switch typed := value.(type) {
	case T:
		return typed, nil
	default:
		return t, errors.New("can't cast remote object to " + reflect.TypeOf(t).String())
	}
}

func optional[T any](value any, err error) Optional[T] {
	tval, terr := castTo[T](value, err)
	return Optional[T]{value: tval, err: terr}
}

func (may Optional[T]) Unwrap() (T, error) {
	return may.value, may.err
}

func (may Optional[T]) Err() error {
	return may.err
}

func (may Optional[T]) Value() T {
	if may.err != nil {
		panic(may.err)
	}
	return may.value
}

func (may Optional[T]) IfPresent(f func(T)) {
	if may.err == nil {
		f(may.value)
	}
}
