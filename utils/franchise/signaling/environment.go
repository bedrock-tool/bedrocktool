package signaling

type Environment struct {
	// ServiceURI is the URI of the service where connections should be directed.
	// It is the base URL used for dialing a WebSocket connection of a Conn.
	ServiceURI string `json:"serviceUri"`
	// StunURI is the URI of a STUN server available to connect. It seems unused as it is always
	// provided in a [nethernet.Credentials] received from a Conn.
	StunURI string `json:"stunUri"`
	// TurnURI is the URI of a TURN server available to connect. It seems unused as it is always
	// provided in a credentials received from a Conn.
	TurnURI string `json:"turnUri"`
}
