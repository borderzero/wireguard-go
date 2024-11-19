package conn

import (
	"net/netip"

	"github.com/borderzero/border0-go/lib/nacl"
)

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
	inner Endpoint
	key   *nacl.PublicKey
}

// NewKeyedEndpoint returns a new KeyedEndpoint.
func NewKeyedEndpoint(ep Endpoint, key *nacl.PublicKey) Endpoint {
	return &KeyedEndpoint{inner: ep, key: key}
}

// ClearSrc clears the source address.
func (rep *KeyedEndpoint) ClearSrc() { rep.inner.ClearSrc() }

// SrcToString returns the local source address (ip:port).
func (rep *KeyedEndpoint) SrcToString() string { return rep.inner.SrcToString() }

// SrcToString returns the destination address (ip:port).
func (rep *KeyedEndpoint) DstToString() string { return rep.inner.DstToString() }

// DstToBytes returns a byte slice used for mac2 cookie calculations.
func (rep *KeyedEndpoint) DstToBytes() []byte { return rep.inner.DstToBytes() }

// DstIP returns the destination IP address.
func (rep *KeyedEndpoint) DstIP() netip.Addr { return rep.inner.DstIP() }

// DstIP returns the source IP address.
func (rep *KeyedEndpoint) SrcIP() netip.Addr { return rep.inner.SrcIP() }

// GetInner gets the inner Endpoint for a RoamingEndpoint.
func (rep *KeyedEndpoint) GetInner() Endpoint { return rep.inner }

// GetPublicKey gets the public key for a RoamingEndpoint.
func (rep *KeyedEndpoint) GetPublicKey() *nacl.PublicKey { return rep.key }
