package v1

import (
	"context"
	"sync"

	"github.com/retrozoid/control"
	"github.com/retrozoid/control/protocol/browser"
	"github.com/retrozoid/control/protocol/network"
	"github.com/retrozoid/control/protocol/page"
	"github.com/retrozoid/control/protocol/runtime"
	"github.com/retrozoid/control/protocol/target"
	"github.com/retrozoid/control/transport"
)

type BrowserContext struct {
	Client *transport.Client
}

func New(client *transport.Client) BrowserContext {
	return BrowserContext{Client: client}
}

func (b BrowserContext) Call(method string, send, recv interface{}) error {
	return b.Client.Call("", method, send, recv)
}

func (b BrowserContext) Crash() error {
	return browser.Crash(b)
}

func (b BrowserContext) Close() error {
	return b.Client.Close()
}

func (b BrowserContext) SetDiscoverTargets(discover bool) error {
	return target.SetDiscoverTargets(b, target.SetDiscoverTargetsArgs{Discover: discover})
}

func (b BrowserContext) runSession(targetID target.TargetID, sessionID target.SessionID) (session *Session, err error) {
	session = &Session{
		id:         sessionID,
		tid:        targetID,
		browser:    b,
		eventPool:  make(chan transport.Event, 20000),
		publisher:  transport.NewPublisher(),
		executions: &sync.Map{},
	}
	session.context, session.cancelCtx = context.WithCancel(b.Client.Context())
	session.Input = Input{s: session, mx: &sync.Mutex{}}
	session.Network = Network{s: session}
	session.Emulation = Emulation{s: session}

	go session.handleEventPool()
	session.detach = b.Client.Register(session)

	if err = page.Enable(session); err != nil {
		return nil, err
	}
	if err = runtime.Enable(session); err != nil {
		return nil, err
	}
	if err = runtime.AddBinding(session, runtime.AddBindingArgs{Name: control.bindClick}); err != nil {
		return nil, err
	}
	if err = page.SetLifecycleEventsEnabled(session, page.SetLifecycleEventsEnabledArgs{Enabled: true}); err != nil {
		return nil, err
	}
	if err = target.SetDiscoverTargets(session, target.SetDiscoverTargetsArgs{Discover: true}); err != nil {
		return nil, err
	}
	// maxPostDataSize - The Longest post body size (in bytes) that would be included in requestWillBeSent notification
	if err = network.Enable(session, network.EnableArgs{MaxPostDataSize: 20 * 1024}); err != nil {
		return nil, err
	}
	return
}

func (b BrowserContext) AttachPageTarget(id target.TargetID) (*Session, error) {
	val, err := target.AttachToTarget(b, target.AttachToTargetArgs{
		TargetId: id,
		Flatten:  true,
	})
	if err != nil {
		return nil, err
	}
	return b.runSession(id, val.SessionId)
}

func (b BrowserContext) CreatePageTarget(url string) (*Session, error) {
	if url == "" {
		url = Blank // headless chrome crash when url is empty
	}
	r, err := target.CreateTarget(b, target.CreateTargetArgs{Url: url})
	if err != nil {
		return nil, err
	}
	return b.AttachPageTarget(r.TargetId)
}

func (b BrowserContext) ActivateTarget(id target.TargetID) error {
	return target.ActivateTarget(b, target.ActivateTargetArgs{
		TargetId: id,
	})
}

func (b BrowserContext) CloseTarget(id target.TargetID) (err error) {
	err = target.CloseTarget(b, target.CloseTargetArgs{TargetId: id})
	/* Target.detachedFromTarget event may come before the response of CloseTarget call */
	if err == ErrDetachedFromTarget {
		return nil
	}
	return err
}

func (b BrowserContext) GetTargets() ([]*target.TargetInfo, error) {
	val, err := target.GetTargets(b, target.GetTargetsArgs{})
	if err != nil {
		return nil, err
	}
	return val.TargetInfos, nil
}
