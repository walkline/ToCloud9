package events

import (
	"encoding/json"

	"github.com/nats-io/nats.go"
)

type AuctionHouseProducer interface {
	PublishAuctionCreated(payload *AuctionHouseEventAuctionCreatedPayload) error
	PublishBidPlaced(payload *AuctionHouseEventBidPlacedPayload) error
	PublishAuctionCanceled(payload *AuctionHouseEventAuctionCanceledPayload) error
	PublishAuctionExpired(payload *AuctionHouseEventAuctionExpiredPayload) error
}

type auctionHouseProducer struct {
	nc *nats.Conn
}

func NewAuctionHouseProducer(nc *nats.Conn) AuctionHouseProducer {
	return &auctionHouseProducer{nc: nc}
}

func (p *auctionHouseProducer) PublishAuctionCreated(payload *AuctionHouseEventAuctionCreatedPayload) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	return p.nc.Publish(AuctionHouseEventAuctionCreated, data)
}

func (p *auctionHouseProducer) PublishBidPlaced(payload *AuctionHouseEventBidPlacedPayload) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	return p.nc.Publish(AuctionHouseEventBidPlaced, data)
}

func (p *auctionHouseProducer) PublishAuctionCanceled(payload *AuctionHouseEventAuctionCanceledPayload) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	return p.nc.Publish(AuctionHouseEventAuctionCanceled, data)
}

func (p *auctionHouseProducer) PublishAuctionExpired(payload *AuctionHouseEventAuctionExpiredPayload) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	return p.nc.Publish(AuctionHouseEventAuctionExpired, data)
}
