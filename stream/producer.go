package stream

type Producer interface {
	Produce(key []byte, msg []byte) error
}
