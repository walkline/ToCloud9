package service

import "context"

type LayerProvisioner interface {
	EnsureLayer(ctx context.Context, realmID, layerID uint32) error
	DeleteLayer(ctx context.Context, realmID, layerID uint32) error
}

type NoopLayerProvisioner struct{}

func (NoopLayerProvisioner) EnsureLayer(context.Context, uint32, uint32) error { return nil }
func (NoopLayerProvisioner) DeleteLayer(context.Context, uint32, uint32) error { return nil }
