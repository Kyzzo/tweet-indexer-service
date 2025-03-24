package stream

type Consumer interface {
	Consume() <-chan []byte
	Connect(brokers []string, topic string, cgroup string)
}
