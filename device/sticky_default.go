//go:build !linux

package device

import (
	"github.com/borderzero/wireguard-go/conn"
	"github.com/borderzero/wireguard-go/rwcancel"
)

func (device *Device) startRouteListener(bind conn.Bind) (*rwcancel.RWCancel, error) {
	return nil, nil
}
