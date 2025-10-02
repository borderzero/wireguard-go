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
type KeyedEndpoint interface {
	// FromPeer is called at least once per successfully Cryptokey Routing ID'd
	// [ReceiveFunc] packets batch for a given node key. wireguard-go will
	// always call it for the latest/tail packet in the batch, only ever
	// suppressing calls for older packets.
	FromPeer(peerPublicKey [32]byte)
}
