package control

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/ecwid/control/cdp"
	"github.com/ecwid/control/protocol/browser"
	"github.com/ecwid/control/protocol/common"
	"github.com/ecwid/control/protocol/dom"
	"github.com/ecwid/control/protocol/network"
	"github.com/ecwid/control/protocol/page"
	"github.com/ecwid/control/protocol/runtime"
	"github.com/ecwid/control/protocol/target"
)

// The Longest post body size (in bytes) that would be included in requestWillBeSent notification
var MaxPostDataSize = 20 * 1024 // 20KB

func mustUnmarshal[T any](u cdp.Message) T {
	var value T
	err := json.Unmarshal(u.Params, &value)
	if err != nil {
		panic(err)
	}
	return value
}

type Session struct {
	Timeout   time.Duration
	context   context.Context
	transport *cdp.Transport
	targetID  target.TargetID
	sessionID string
	frames    *syncFrames
	Frame
}

func (s *Session) Transport() *cdp.Transport {
	return s.transport
}

func (s *Session) GetID() string {
	return s.sessionID
}

func (s *Session) Call(method string, send, recv any) error {
	select {
	case <-s.context.Done():
		return context.Cause(s.context)
	default:
	}
	future := s.transport.Send(&cdp.Request{
		SessionID: s.sessionID,
		Method:    method,
		Params:    send,
	})
	defer future.Cancel()

	ctxTo, cancel := context.WithTimeout(s.context, s.Timeout)
	defer cancel()
	value, err := future.Get(ctxTo)
	if err != nil {
		return err
	}

	if recv != nil {
		return json.Unmarshal(value.Result, recv)
	}
	return nil
}

func (s *Session) Subscribe() (channel chan cdp.Message, cancel func()) {
	return s.transport.Subscribe(s.sessionID)
}

func (s *Session) Close() error {
	return target.CloseTarget(s, target.CloseTargetArgs{
		TargetId: s.targetID,
	})
}

func NewSession(transport *cdp.Transport, targetID target.TargetID) (*Session, error) {
	var session = &Session{
		transport: transport,
		targetID:  targetID,
		Timeout:   60 * time.Second,
		frames:    &syncFrames{value: make(map[common.FrameId]string)},
	}
	session.Frame = Frame{
		session: session,
		id:      common.FrameId(session.targetID),
	}
	var cancel func(error)
	session.context, cancel = context.WithCancelCause(transport.Context())
	val, err := target.AttachToTarget(session, target.AttachToTargetArgs{
		TargetId: targetID,
		Flatten:  true,
	})
	if err != nil {
		return nil, err
	}
	session.sessionID = string(val.SessionId)
	channel, unsubscribe := session.Subscribe()
	go func() {
		for msg := range channel {
			if err := session.handle(msg); err != nil {
				unsubscribe()
				cancel(err)
			}
		}
	}()

	if err = page.Enable(session); err != nil {
		return nil, err
	}
	if err = dom.Enable(session, dom.EnableArgs{}); err != nil {
		return nil, err
	}
	if err = runtime.Enable(session); err != nil {
		return nil, err
	}
	if err = target.SetDiscoverTargets(session, target.SetDiscoverTargetsArgs{Discover: true}); err != nil {
		return nil, err
	}
	if err = network.Enable(session, network.EnableArgs{MaxPostDataSize: MaxPostDataSize}); err != nil {
		return nil, err
	}
	return session, nil
}

func (s *Session) handle(message cdp.Message) error {
	switch message.Method {

	case "Runtime.executionContextCreated":
		executionContextCreated := mustUnmarshal[runtime.ExecutionContextCreated](message)
		aux := executionContextCreated.Context.AuxData.(map[string]any)
		frameID := aux["frameId"].(string)
		s.frames.Set(common.FrameId(frameID), executionContextCreated.Context.UniqueId)

	case "Page.frameDetached":
		frameDetached := mustUnmarshal[page.FrameDetached](message)
		s.frames.Remove(frameDetached.FrameId)

	case "Target.detachedFromTarget":
		return errors.New("detached from target")

	case "Target.targetDestroyed":
		targetDestroyed := mustUnmarshal[target.TargetDestroyed](message)
		if s.targetID == targetDestroyed.TargetId {
			return errors.New("target destroyed")
		}

	case "Target.targetCrashed":
		targetCrashed := mustUnmarshal[target.TargetCrashed](message)
		if s.targetID == targetCrashed.TargetId {
			return errors.New(string(message.Params))
		}
	}

	return nil
}

func (s *Session) CaptureScreenshot(format string, quality int, clip *page.Viewport, fromSurface, captureBeyondViewport, optimizeForSpeed bool) ([]byte, error) {
	val, err := page.CaptureScreenshot(s, page.CaptureScreenshotArgs{
		Format:                format,
		Quality:               quality,
		Clip:                  clip,
		FromSurface:           fromSurface,
		CaptureBeyondViewport: captureBeyondViewport,
		OptimizeForSpeed:      optimizeForSpeed,
	})
	if err != nil {
		return nil, err
	}
	return val.Data, nil
}

func (s *Session) SetDownloadBehavior(behavior string, downloadPath string, eventsEnabled bool) error {
	return browser.SetDownloadBehavior(s, browser.SetDownloadBehaviorArgs{
		Behavior:      behavior,
		DownloadPath:  downloadPath,
		EventsEnabled: eventsEnabled, // default false
	})
}

func (s *Session) GetTargetCreated() FutureWithDeadline[target.TargetCreated] {
	return MakeFuture(s, "Target.targetCreated", func(t target.TargetCreated) bool {
		return t.TargetInfo.Type == "page" && t.TargetInfo.OpenerId == s.targetID
	})
}
