package main

import (
	"context"
	"log"
	"log/slog"
	// "os"

	"github.com/retrozoid/control"
	"github.com/retrozoid/control/backoff"
)

func main() {
	// slogger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
	// 	Level: slog.LevelInfo,
	// }))
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
		return session.Frame.QueryAll(".grid-product__title-inner").MustGet().Map(func(n control.Node) (string, error) {
			return n.GetTextContent()
		})
	})
	log.Println(val)

	backoff.Exec(func() error {
		e := session.Frame.Query(`.pager__count-pages`).MustGet()
		e.GetViewportRect()
		return e.Click()
	})
}
