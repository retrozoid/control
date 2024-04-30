package cdp

var BrokerChannelSize = 50000

type subscriber struct {
	sessionID string
	channel   chan Message
}

type broker struct {
	cancel  chan struct{}
	publish chan Message
	sub     chan subscriber
	unsub   chan chan Message
}

func makeBroker() broker {
	return broker{
		cancel:  make(chan struct{}),
		publish: make(chan Message),
		sub:     make(chan subscriber),
		unsub:   make(chan chan Message),
	}
}

func (b broker) run() {
	var value = map[chan Message]subscriber{}
	for {
		select {

		case sub := <-b.sub:
			value[sub.channel] = sub

		case channel := <-b.unsub:
			if _, ok := value[channel]; ok {
				delete(value, channel)
				close(channel)
			}

		case <-b.cancel:
			for msgCh := range value {
				close(msgCh)
			}
			close(b.sub)
			// close(b.unsub)
			close(b.publish)
			return

		case message := <-b.publish:
			for _, subscriber := range value {
				if message.SessionID == "" || subscriber.sessionID == "" || message.SessionID == subscriber.sessionID {
					subscriber.channel <- message
				}
			}
		}
	}
}

func (b broker) Subscribe(sessionID string) chan Message {
	sub := subscriber{
		sessionID: sessionID,
		channel:   make(chan Message, BrokerChannelSize),
	}
	b.sub <- sub
	return sub.channel
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
