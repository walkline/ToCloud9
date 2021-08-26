package service

type Listener interface {
	Listen() error
	Stop() error
}
