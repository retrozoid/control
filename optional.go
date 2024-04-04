package control

import (
	"fmt"
	"reflect"
)

type Optional[T any] struct {
	value T
	err   error
}

func castTo[T any](value any, err error) (T, error) {
	var nilValue T
	if err != nil {
		return nilValue, err
	}
	if value == nil {
		return nilValue, nil
	}
	switch typed := value.(type) {
	case T:
		return typed, nil
	default:
		return nilValue, fmt.Errorf("can't cast %s to %s", reflect.TypeOf(value), reflect.TypeOf(nilValue))
	}
}

func optional[T any](value any, err error) Optional[T] {
	var nilValue T
	if err != nil {
		return Optional[T]{err: err}
	}
	if value == nil {
		return Optional[T]{}
	}
	switch typed := value.(type) {
	case T:
		return Optional[T]{value: typed}
	default:
		return Optional[T]{err: fmt.Errorf("can't cast %s to %s", reflect.TypeOf(value), reflect.TypeOf(nilValue))}
	}
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
