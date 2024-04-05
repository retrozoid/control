package control

import (
	"fmt"
	"reflect"
)

type Optional[T any] struct {
	value T
	err   error
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

func (op Optional[T]) Unwrap() (T, error) {
	return op.value, op.err
}

func (op Optional[T]) Err() error {
	return op.err
}

func (op Optional[T]) Value() T {
	if op.err != nil {
		panic(op.err)
	}
	return op.value
}

func (op Optional[T]) IfPresent(f func(T)) {
	if op.err == nil {
		f(op.value)
	}
}
