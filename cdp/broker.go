package cdp

var BrokerChannelSize = 10

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
		publish: make(chan Message, 10),
		sub:     make(chan subchan, 1),
		unsub:   make(chan chan Message, 1),
	}
}

func (b broker) run() {
	var (
		value = map[chan Message]string{}
	)
	for {
		select {

		case <-b.cancel:
			for msgCh := range value {
				close(msgCh)
			}
			close(b.sub)
			// close(b.unsub)
			close(b.publish)
			return

		case subchan := <-b.sub:
			value[subchan.channel] = subchan.sessionID

		case channel := <-b.unsub:
			delete(value, channel)
			close(channel)

		case message := <-b.publish:
			for msgCh, channelID := range value {
				if message.SessionID == "" || channelID == "" || message.SessionID == channelID {
					select {
					case msgCh <- message:
					default:
					}
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
