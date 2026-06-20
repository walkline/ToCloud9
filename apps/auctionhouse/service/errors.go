package service

import "errors"

var (
	ErrAuctionNotFound      = errors.New("auction not found")
	ErrBidOwnAuction        = errors.New("cannot bid on own auction")
	ErrBidTooLow            = errors.New("bid too low")
	ErrBidIncrementTooLow   = errors.New("bid increment too low")
)
