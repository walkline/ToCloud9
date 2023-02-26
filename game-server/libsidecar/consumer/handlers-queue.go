package consumer

import "fmt"

type HandlersQueue interface {
	Push(handler Handler)
	Pop() Handler
}

type handlersFIFOQueue struct {
	queue chan Handler
}

func NewHandlersFIFOQueue() HandlersQueue {
	const bufferSize = 1000
	return &handlersFIFOQueue{
		queue: make(chan Handler, bufferSize),
	}
}

func (h *handlersFIFOQueue) Push(handler Handler) {
	h.queue <- handler
}

func (h *handlersFIFOQueue) Pop() Handler {
	select {
	case handler := <-h.queue:
		fmt.Println("return value!")
		return handler
	// no more handlers, return nil
	default:
		return nil
	}
}