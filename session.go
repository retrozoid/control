package control

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"sync"
	"time"

	"github.com/retrozoid/control/cdp"
	"github.com/retrozoid/control/protocol/browser"
	"github.com/retrozoid/control/protocol/common"
	"github.com/retrozoid/control/protocol/dom"
	"github.com/retrozoid/control/protocol/layertree"
	"github.com/retrozoid/control/protocol/network"
	"github.com/retrozoid/control/protocol/overlay"
	"github.com/retrozoid/control/protocol/page"
	"github.com/retrozoid/control/protocol/runtime"
	"github.com/retrozoid/control/protocol/target"
)

// The Longest post body size (in bytes) that would be included in requestWillBeSent notification
var (
	MaxPostDataSize = 20 * 1024 // 20KB
)

const Blank = "about:blank"

var (
	ErrTargetDestroyed             error = errors.New("target destroyed")
	ErrTargetDetached              error = errors.New("session detached from target")
	ErrNetworkIdleReachedTimeout   error = errors.New("session network idle reached timeout")
	ErrLayerTreeIdleReachedTimeout error = errors.New("session layer tree idle reached timeout")
)

type TargetCrashedError []byte

func (t TargetCrashedError) Error() string {
	return string(t)
}

func mustUnmarshal[T any](u cdp.Message) T {
	var value T
	err := json.Unmarshal(u.Params, &value)
	if err != nil {
		panic(err)
	}
	return value
}

type Session struct {
	timeout          time.Duration
	context          context.Context
	transport        *cdp.Transport
	targetID         target.TargetID
	sessionID        string
	frames           *sync.Map
	Frame            *Frame
	highlightEnabled bool
	mouse            Mouse
	kb               Keyboard
	touch            Touch
}

func (s *Session) Transport() *cdp.Transport {
	return s.transport
}

func (s *Session) Log(t time.Time, msg string, args ...any) {
	level := slog.LevelInfo
	args = append(args, "sessionId", s.sessionID, "duration", time.Since(t).String())
	for n := range args {
		switch a := args[n].(type) {
		case error:
			if a != nil {
				args[n] = a.Error()
				level = slog.LevelWarn
			}
		}
	}
	s.transport.Log(level, msg, args...)
}

func (s *Session) GetID() string {
	return s.sessionID
}

func (s *Session) IsDone() bool {
	select {
	case <-s.context.Done():
		return true
	default:
		return false
	}
}

func (s *Session) Call(method string, send, recv any) error {
	select {
	case <-s.context.Done():
		return context.Cause(s.context)
	default:
	}
	future := s.transport.Send(&cdp.Request{
		SessionID: string(s.sessionID),
		Method:    method,
		Params:    send,
	})
	defer future.Cancel()

	ctxTo, cancel := context.WithTimeout(s.context, s.timeout)
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

func NewSession(transport *cdp.Transport, targetID target.TargetID) (*Session, error) {
	var session = &Session{
		transport: transport,
		targetID:  targetID,
		timeout:   60 * time.Second,
		frames:    &sync.Map{},
	}
	session.mouse = NewMouse(session)
	session.kb = NewKeyboard(session)
	session.touch = NewTouch(session)
	session.Frame = &Frame{
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
		if err := session.handle(channel); err != nil {
			unsubscribe()
			cancel(err)
		}
	}()

	if err = page.Enable(session); err != nil {
		return nil, err
	}
	if err = runtime.Enable(session); err != nil {
		return nil, err
	}
	if err = dom.Enable(session, dom.EnableArgs{IncludeWhitespace: "none"}); err != nil {
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

func (s *Session) EnableHighlight() error {
	if err := overlay.Enable(s); err != nil {
		return err
	}
	s.highlightEnabled = true
	return nil
}

func (s *Session) handle(channel chan cdp.Message) error {
	for message := range channel {
		switch message.Method {

		case "Runtime.executionContextCreated":
			executionContextCreated := mustUnmarshal[runtime.ExecutionContextCreated](message)
			aux := executionContextCreated.Context.AuxData.(map[string]any)
			frameID := aux["frameId"].(string)
			s.frames.Store(common.FrameId(frameID), executionContextCreated.Context.UniqueId)

		case "Page.frameDetached":
			frameDetached := mustUnmarshal[page.FrameDetached](message)
			s.frames.Delete(frameDetached.FrameId)

		case "Target.detachedFromTarget":
			detachedFromTarget := mustUnmarshal[target.DetachedFromTarget](message)
			if s.sessionID == string(detachedFromTarget.SessionId) {
				return ErrTargetDetached
			}

		case "Target.targetDestroyed":
			targetDestroyed := mustUnmarshal[target.TargetDestroyed](message)
			if s.targetID == targetDestroyed.TargetId {
				return ErrTargetDestroyed
			}

		case "Target.targetCrashed":
			targetCrashed := mustUnmarshal[target.TargetCrashed](message)
			if s.targetID == targetCrashed.TargetId {
				return TargetCrashedError(message.Params)
			}
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

func (s *Session) GetTargetCreated() Future[target.TargetCreated] {
	return Subscribe(s, "Target.targetCreated", func(t target.TargetCreated) bool {
		return t.TargetInfo.Type == "page" && t.TargetInfo.OpenerId == s.targetID
	})
}

func (s *Session) AttachToTarget(id target.TargetID) (*Session, error) {
	return NewSession(s.transport, id)
}

func (s *Session) CreatePageTargetTab(url string) (*Session, error) {
	if url == "" {
		url = Blank // headless chrome crash when url is empty
	}
	r, err := target.CreateTarget(s, target.CreateTargetArgs{Url: url})
	if err != nil {
		return nil, err
	}
	return s.AttachToTarget(r.TargetId)
}

func (s *Session) ActivateTarget(id target.TargetID) error {
	return target.ActivateTarget(s, target.ActivateTargetArgs{
		TargetId: id,
	})
}

func (s *Session) Activate() error {
	return s.ActivateTarget(s.targetID)
}

func (s *Session) Close() error {
	return s.CloseTarget(s.targetID)
}

func (s *Session) getDocument() (*dom.Node, error) {
	value, err := dom.GetDocument(s, dom.GetDocumentArgs{
		Depth:  1,
		Pierce: false,
	})
	if err != nil {
		return nil, err
	}
	return value.Root, nil
}

func (s *Session) CloseTarget(id target.TargetID) (err error) {
	err = target.CloseTarget(s, target.CloseTargetArgs{TargetId: id})
	/* Target.detachedFromTarget event may come before the response of CloseTarget call */
	if err == ErrTargetDetached {
		return nil
	}
	return err
}

func (s *Session) Click(point Point) error {
	return s.mouse.Click(MouseLeft, point, time.Millisecond*85)
}

func (s *Session) Swipe(from, to Point) error {
	return s.touch.Swipe(from, to)
}

func (s *Session) Hover(point Point) error {
	return s.mouse.Move(MouseNone, point)
}

func (s *Session) GetLayout() Optional[page.GetLayoutMetricsVal] {
	view, err := page.GetLayoutMetrics(s)
	if err != nil {
		return Optional[page.GetLayoutMetricsVal]{err: err}
	}
	return Optional[page.GetLayoutMetricsVal]{value: *view}
}

func (s *Session) GetNavigationEntry() Optional[page.NavigationEntry] {
	val, err := page.GetNavigationHistory(s)
	if err != nil {
		return Optional[page.NavigationEntry]{err: err}
	}
	if val.CurrentIndex == -1 {
		return Optional[page.NavigationEntry]{value: page.NavigationEntry{Url: Blank}}
	}
	return Optional[page.NavigationEntry]{value: *val.Entries[val.CurrentIndex]}
}

func (s *Session) GetCurrentURL() Optional[string] {
	now := time.Now()
	opt := optional[string](s.getCurrentURL())
	s.Log(now, "GetCurrentURL", "value", opt.value, "err", opt.err)
	return opt
}

func (s *Session) getCurrentURL() (string, error) {
	e, err := s.GetNavigationEntry().Unwrap()
	if err != nil {
		return "", err
	}
	return e.Url, nil
}

func (s *Session) NavigateHistory(delta int) error {
	now := time.Now()
	err := s.navigateHistory(delta)
	s.Log(now, "NavigateHistory", "delta", delta, "err", err)
	return err
}

func (s *Session) navigateHistory(delta int) error {
	val, err := page.GetNavigationHistory(s)
	if err != nil {
		return err
	}
	move := val.CurrentIndex + delta
	if move >= 0 && move < len(val.Entries) {
		return page.NavigateToHistoryEntry(s, page.NavigateToHistoryEntryArgs{
			EntryId: val.Entries[move].Id,
		})
	}
	return nil
}

func (s *Session) CaptureNetworkRequest(condition func(request *network.Request) bool, rejectOnLoadingFailed bool) Future[network.ResponseReceived] {
	var requestID network.RequestId

	channel, cancel := s.Subscribe()
	promise, future := cdp.NewPromise[network.ResponseReceived](cancel)

	go func() {
		for value := range channel {

			switch value.Method {

			case "Network.requestWillBeSent":
				requestWillBeSent := mustUnmarshal[network.RequestWillBeSent](value)
				if condition(requestWillBeSent.Request) {
					requestID = requestWillBeSent.RequestId
				}

			case "Network.responseReceived":
				responseReceived := mustUnmarshal[network.ResponseReceived](value)
				if responseReceived.RequestId == requestID {
					promise.Resolve(responseReceived)
					return
				}

			case "Network.loadingFailed":
				if rejectOnLoadingFailed {
					loadingFailed := mustUnmarshal[network.LoadingFailed](value)
					if loadingFailed.RequestId == requestID {
						promise.Reject(errors.New(loadingFailed.ErrorText))
						return
					}
				}
			}
		}
	}()
	return NewDeadlineFuture(s.context, s.timeout, future)
}

func (s *Session) NetworkIdle(threshold time.Duration, timeout time.Duration, init func()) error {
	var (
		channel, cancel = s.Subscribe()
		n               = time.Now()
		last            = n.Add(threshold)
		timer           = time.NewTimer(timeout)
		requests        = 0
		queue           = map[network.RequestId]*network.Request{}
	)
	defer func() {
		cancel()
		timer.Stop()
		s.Log(n, "NetworkIdle", "idle_threshold", threshold.String(), "requests", requests, "queue", queue)
	}()
	init()
	for {
		select {
		case value := <-channel:
			switch value.Method {

			case "Network.requestWillBeSent":
				willBeSent := mustUnmarshal[network.RequestWillBeSent](value)
				queue[willBeSent.RequestId] = willBeSent.Request
				last = time.Now()
				requests++

			case "Network.responseReceived":
				received := mustUnmarshal[network.ResponseReceived](value)
				delete(queue, received.RequestId)
				last = time.Now()

			case "Network.loadingFailed":
				loadingFailed := mustUnmarshal[network.LoadingFailed](value)
				delete(queue, loadingFailed.RequestId)
				last = time.Now()
			}
		case <-timer.C:
			return ErrNetworkIdleReachedTimeout
		default:
			if time.Since(last) > threshold && len(queue) == 0 {
				return nil
			}
		}
	}
}

func (s *Session) LayerTreeIdle(threshold time.Duration, timeout time.Duration) (err error) {
	var (
		channel, cancel = s.Subscribe()
		n               = time.Now()
		last            = n.Add(threshold)
		timer           = time.NewTimer(timeout)
	)
	err = layertree.Enable(s)
	defer func() {
		cancel()
		timer.Stop()
		err = layertree.Disable(s)
		s.Log(n, "LayerTreeIdle", "idle_threshold", threshold.String(), "error", err)
	}()
	if err != nil {
		return err
	}
	for {
		select {
		case value := <-channel:
			switch value.Method {
			case "LayerTree.layerPainted", "LayerTree.layerTreeDidChange":
				last = time.Now()
			}
		case <-timer.C:
			return ErrLayerTreeIdleReachedTimeout
		default:
			if time.Since(last) > threshold {
				return nil
			}
		}
	}
}
