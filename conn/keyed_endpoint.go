package conn

// KeyedEndpoint is an Endpoint implementation which is aware of the
// remote peer/endpoint's public key. This is useful for logic in the
// bind to determine if a given packet was sent from a peer from an
// address other than the one in the state.
//
// This effectively enables changing the address of the peer in the
// state such that we can maintain a QOS conn for the peer even if its
// addresses change.
//
// The specific scenario that made us add this is that for peers behind
// certain NAT gateways, the public UDP address assigned to outbound
// traffic (on the gateway itself) is endpoint-dependent (can be thought
// of as destination ip:port-dependent) such that the self-discovered
// address (via STUN) is different (mostly only the port is different)
// than the address that the remote peer sees on inbound traffic.
type KeyedEndpoint struct {
	Endpoint // embedded

	key [32]byte
}

// NewKeyedEndpoint returns a new KeyedEndpoint.
func NewKeyedEndpoint(ep Endpoint, key [32]byte) Endpoint {
	return &KeyedEndpoint{
		Endpoint: ep,
		key:      key,
	}
}

// GetInner gets the inner Endpoint for a RoamingEndpoint.
func (rep *KeyedEndpoint) GetInner() Endpoint { return rep.Endpoint }

// GetPublicKey gets the public key for a RoamingEndpoint.
func (rep *KeyedEndpoint) GetPublicKey() [32]byte { return rep.key }
