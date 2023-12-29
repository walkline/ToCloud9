package queue

type Handler interface {
	Handle()
}

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
	if handler == nil {
		return
	}

	h.queue <- handler
}

func (h *handlersFIFOQueue) Pop() Handler {
	select {
	case handler := <-h.queue:
		return handler
	// no more handlers, return nil
	default:
		return nil
	}
}

type HandlerFunc func()

func (f HandlerFunc) Handle() {
	f()
}
