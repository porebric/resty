package resty

import (
	"context"
	"github.com/porebric/configs"
	"time"
)

const (
	confServerPort   = "server_port"
	confCloseTimeout = "close_timeout"
)

type options struct {
	Port    int32
	Timeout time.Duration
}

func newOptions(ctx context.Context) *options {
	o := new(options)
	o.Port = int32(configs.Value(ctx, confServerPort).Int())
	o.Timeout = configs.Value(ctx, confCloseTimeout).Duration()

	if o.Timeout == 0 {
		o.Timeout = 3 * time.Second
	}
	if o.Port == 0 {
		o.Port = 8080
	}

	return o
}
