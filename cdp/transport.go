package cdp

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

var DefaultDialer = websocket.Dialer{
	ReadBufferSize:   8192,
	WriteBufferSize:  8192,
	HandshakeTimeout: 45 * time.Second,
	Proxy:            http.ProxyFromEnvironment,
}

type Transport struct {
	context context.Context
	cancel  func(error)
	conn    *websocket.Conn
	seq     uint64
	pending map[uint64]responsePromise
	mutex   sync.Mutex
	broker  broker
	logger  *slog.Logger
}

func DefaultDial(context context.Context, url string, logger *slog.Logger) (*Transport, error) {
	return Dial(context, DefaultDialer, url, logger)
}

func Dial(parent context.Context, dialer websocket.Dialer, url string, logger *slog.Logger) (*Transport, error) {
	conn, _, err := dialer.Dial(url, nil)
	if err != nil {
		return nil, err
	}
	conn.EnableWriteCompression(true)
	ctx, cancel := context.WithCancelCause(parent)
	transport := &Transport{
		context: ctx,
		cancel:  cancel,
		conn:    conn,
		seq:     1,
		broker:  makeBroker(),
		pending: make(map[uint64]responsePromise),
		logger:  logger,
	}
	go transport.broker.run()
	go func() {
		var readerr error
		for ; readerr == nil; readerr = transport.read() {
		}
		transport.cancel(readerr)
		transport.gracefullyClose()
	}()
	return transport, nil
}

func (t *Transport) Log(level slog.Level, msg string, args ...any) {
	if t.logger != nil {
		t.logger.Log(t.context, level, msg, args...)
	}
}

func (t *Transport) Context() context.Context {
	return t.context
}

func (t *Transport) isClosed() bool {
	select {
	case <-t.context.Done():
		return true
	default:
		return false
	}
}

func (t *Transport) error() error {
	return context.Cause(t.context)
}

func (t *Transport) Close() error {
	if t.isClosed() {
		return t.error()
	}
	_, err := t.Send(&Request{Method: "Browser.close"}).Get(t.context)
	if err != nil {
		return err
	}
	t.cancel(errors.New("gracefully closed"))
	return t.conn.Close()
}

func (t *Transport) gracefullyClose() {
	t.mutex.Lock()
	t.broker.Cancel()
	err := t.error()
	for key, value := range t.pending {
		value.Reject(err)
		delete(t.pending, key)
	}
	t.mutex.Unlock()
}

func (t *Transport) Subscribe(sessionID string) (chan Message, func()) {
	if t.isClosed() {
		return nil, nil
	}
	ch := t.broker.Subscribe(sessionID)
	return ch, func() {
		t.broker.Unsubscribe(ch)
	}
}

func (t *Transport) Send(request *Request) ResponseFuture {
	var resolver, future = MakePromise[Response](func() {})
	if t.isClosed() {
		resolver.Reject(t.error())
		return future
	}
	t.mutex.Lock()
	seq := t.seq
	t.seq++
	t.pending[seq] = resolver
	request.ID = seq
	t.Log(slog.LevelDebug, "send ->", "request", request.String())
	t.mutex.Unlock()

	if err := t.conn.WriteJSON(request); err != nil {
		t.mutex.Lock()
		delete(t.pending, seq)
		t.mutex.Unlock()
		resolver.Reject(err)
	}
	return future
}

func (t *Transport) read() error {
	var response = Response{}
	if err := t.conn.ReadJSON(&response); err != nil {
		return err
	}
	t.Log(slog.LevelDebug, "recv <-", "response", response.String())

	if response.ID == 0 && response.Message != nil {
		t.broker.Publish(*response.Message)
		return nil
	}

	t.mutex.Lock()
	value, ok := t.pending[response.ID]
	delete(t.pending, response.ID)
	t.mutex.Unlock()

	if !ok {
		return errors.New("unexpected response " + response.String())
	}
	if response.Error != nil {
		value.Reject(response.Error)
		return nil
	}
	value.Resolve(response)
	return nil
}
