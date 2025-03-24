package stream

type Producer interface {
	Produce(msg []byte) error
}
