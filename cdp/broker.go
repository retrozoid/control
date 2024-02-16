package cdp

var BrokerChannelSize = 50000

type subchan struct {
	sessionID string
	channel   chan Message
}

type broker struct {
	cancel  chan struct{}
	publish chan Message
	sub     chan subchan
	unsub   chan chan Message
}

func makeBroker() broker {
	return broker{
		cancel:  make(chan struct{}),
		publish: make(chan Message),
		sub:     make(chan subchan),
		unsub:   make(chan chan Message),
	}
}

func (b broker) run() {
	var value = map[chan Message]string{}
	for {
		select {

		case subchan := <-b.sub:
			value[subchan.channel] = subchan.sessionID

		case channel := <-b.unsub:
			delete(value, channel)
			close(channel)

		case <-b.cancel:
			for msgCh := range value {
				close(msgCh)
			}
			close(b.sub)
			// close(b.unsub)
			close(b.publish)
			return

		case message := <-b.publish:
			for msgCh, sessionID := range value {
				if message.SessionID == "" || sessionID == "" || message.SessionID == sessionID {
					msgCh <- message
				}
			}

		}
	}
}

func (b broker) Subscribe(sessionID string) chan Message {
	schan := subchan{
		sessionID: sessionID,
		channel:   make(chan Message, BrokerChannelSize),
	}
	b.sub <- schan
	return schan.channel
}

func (b broker) Unsubscribe(value chan Message) {
	b.unsub <- value
}

func (b broker) Publish(msg Message) {
	b.publish <- msg
}

func (b broker) Cancel() {
	close(b.cancel)
}
