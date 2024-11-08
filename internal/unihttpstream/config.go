package unihttpstream

import (
	"github.com/centrifugal/centrifugo/v5/internal/tools"

	"github.com/centrifugal/centrifuge"
)

type Config struct {
	// MaxRequestBodySize limits request body size.
	MaxRequestBodySize      int
	ConnectCodeToHTTPStatus tools.ConnectCodeToHTTPStatus
	centrifuge.PingPongConfig
}
