package device

import "context"

type Manager interface {
	GetDevices(ctx context.Context, limit, offset int) (map[string]Device, error)
	GetConcreteDevice(ctx context.Context, id string) (*Device, error)
	CreateDevice(ctx context.Context, dto *Device) error
	UpdateDevice(ctx context.Context, dto *Device) error
	DeleteDevice(ctx context.Context, id string) error
}
