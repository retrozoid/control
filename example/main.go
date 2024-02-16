package main

import (
	"context"
	"log"
	"log/slog"

	"github.com/retrozoid/control"
	"github.com/retrozoid/control/backoff"
)

func main() {
	session, dfr, err := control.TakeWithContext(context.TODO(), slog.Default(), "--no-startup-window")
	if err != nil {
		panic(err)
	}
	defer dfr()

	err = session.Frame.Navigate("https://zoid.ecwid.com")
	if err != nil {
		panic(err)
	}

	val := backoff.Value(func() ([]string, error) {
		return session.Frame.QueryAll(".grid-product__title-inner").Value().Map(func(n *control.Node) (string, error) {
			return n.GetText().Unwrap()
		})
	})
	log.Println(val)

	session.Frame.Query(`.pager__count-pages`).Value().Clip().Value()

	backoff.Exec(func() error {
		return session.Frame.Query(`.pager__count-pages`).Value().Click()
	})
}
