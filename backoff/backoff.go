package backoff

import (
	"math/rand"
	"time"
)

var (
	DefaultBackoffTimeout = time.Second * 10
	DefaultBackoffTick    = 500 * time.Millisecond
	DefaultBackoffAttempt = 7
)

// Sleep ...
// 0 = 0s, 1 = 1s, 2 = 2s, 3 = 4s, 4 = 8s, 5 = 17s,
// 6 = 32s, 7 = 1m5s, 8 = 2m9s, 9 = 4m23s, 10 = 8m58s
func sleep(attempt int) {
	backoff := float64(uint(1) << (uint(attempt) - 1))
	backoff += backoff * (0.1 * rand.Float64())
	time.Sleep(time.Second * time.Duration(backoff))
}

func recoverFunc(f func() error) (err any) {
	defer func() {
		if pnc := recover(); pnc != nil {
			err = pnc
		}
	}()
	err = f()
	return
}

func recoverFuncValue[T any](f func() (T, error)) (value T, err any) {
	defer func() {
		if pnc := recover(); pnc != nil {
			err = pnc
		}
	}()
	value, err = f()
	return
}

func Exec(fn func() error) {
	var (
		err   any
		retry = 0
		start = time.Now()
	)
	for time.Since(start) < DefaultBackoffTimeout {
		if retry > 0 {
			time.Sleep(DefaultBackoffTick)
		}
		if err = recoverFunc(fn); err == nil {
			return
		}
		retry++
	}
	panic(err)
}

func MustValue[T any](fn func() T) T {
	return Value[T](func() (T, error) {
		return fn(), nil
	})
}

func Value[T any](fn func() (T, error)) T {
	var (
		value T
		err   any
		retry = 0
		start = time.Now()
	)
	for time.Since(start) < DefaultBackoffTimeout {
		if retry > 0 {
			time.Sleep(DefaultBackoffTick)
		}
		if value, err = recoverFuncValue(fn); err == nil {
			return value
		}
		retry++
	}
	panic(err)
}
