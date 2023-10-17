package control

import (
	"context"
	"log"
	"net/http"

	"github.com/ecwid/control/cdp"
	"github.com/ecwid/control/chrome"
	"github.com/ecwid/control/protocol/target"
)

func Take(args ...string) (session *Session, dfr func()) {
	ctx := context.TODO()
	browser, err := chrome.Launch(ctx, args...)
	if err != nil {
		log.Panicln(err)
	}
	tab, err := browser.NewTab(http.DefaultClient, "")
	if err != nil {
		log.Panicln(err)
	}
	transport, err := cdp.DefaultDial(ctx, browser.WebSocketUrl)
	if err != nil {
		log.Panicln(err)
	}
	session, err = NewSession(transport, target.TargetID(tab.ID))
	if err != nil {
		log.Panicln(err)
	}
	return session, func() {
		if err := transport.Close(); err != nil {
			log.Println(err)
		}
		if err = browser.WaitCloseGracefully(); err != nil {
			log.Println(err)
		}
	}
}
