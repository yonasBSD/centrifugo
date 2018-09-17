package centrifuge

import (
	"github.com/centrifugal/centrifuge/internal/proto"
)

// Define some aliases to internal protocol types generated by gogoprotobuf.
// This allows us to simply expose only subset of useful types to centrifuge
// package keeping types that is not supposed to be in public API hidden from
// library users.
type (
	// Error represents client reply error.
	Error = proto.Error
	// Raw represents raw bytes.
	Raw = proto.Raw
	// Publication allows to deliver custom payload to all channel subscribers.
	Publication = proto.Publication
	// Join sent to channel after someone subscribed.
	Join = proto.Join
	// Leave sent to channel after someone unsubscribed.
	Leave = proto.Leave
	// ClientInfo is short information about client connection.
	ClientInfo = proto.ClientInfo
	// Encoding represents client connection transport encoding format.
	Encoding = proto.Encoding
)