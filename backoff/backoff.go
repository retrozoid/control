package backoff

import (
	"math/rand"
	"time"
)

var DefaultBackoffTimeout = time.Minute

// Sleep ...
// 0 = 0s, 1 = 1s, 2 = 2s, 3 = 4s, 4 = 8s, 5 = 17s,
// 6 = 32s, 7 = 1m5s, 8 = 2m9s, 9 = 4m23s, 10 = 8m58s
func sleep(attempt int) {
	backoff := float64(uint(1) << (uint(attempt) - 1))
	backoff += backoff * (0.1 * rand.Float64())
	time.Sleep(time.Second * time.Duration(backoff))
}

func Exec(fn func() error) {
	var err error
	var retry = 0
	for start := time.Now(); time.Since(start) < DefaultBackoffTimeout; {
		if retry > 0 {
			sleep(retry)
		}
		if err = fn(); err == nil {
			return
		}
		retry++
	}
	panic(err)
}

func Value[T any](fn func() (T, error)) T {
	var value T
	var err error
	var retry = 0
	for start := time.Now(); time.Since(start) < DefaultBackoffTimeout; {
		if retry > 0 {
			sleep(retry)
		}
		if value, err = fn(); err == nil {
			return value
		}
		retry++
	}
	panic(err)
}
