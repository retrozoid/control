package main

import (
	"log"

	"github.com/ecwid/control"
	"github.com/ecwid/control/backoff"
)

func main() {
	session, dfr, err := control.Take("--no-startup-window")
	if err != nil {
		panic(err)
	}
	defer dfr()

	err = session.Navigate("https://zoid.ecwid.com")
	if err != nil {
		panic(err)
	}

	val := backoff.Value(func() (string, error) {
		return session.Query(".pager__count-pages").GetTextContent()
	})
	log.Println(val)

	backoff.Exec(func() error {
		return session.Query(`.pager__count-pages`).Click()
	})
}
