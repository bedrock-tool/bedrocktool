package signaling

import (
	"context"
	"log/slog"
	"math/rand/v2"
	"net/http"
	"strconv"

	"github.com/bedrock-tool/bedrocktool/utils/franchise/authservice"
	"github.com/coder/websocket"
	"github.com/df-mc/go-nethernet"
)

// Dialer provides methods and fields to establish a Conn to a signaling service.
// It allows specifying options for the connection and handles various authentication
// and environment configuration.
type Dialer struct {
	Service *SignalingService

	// Options specifies the options for dialing the signaling service over
	// a WebSocket connection. If nil, a new *websocket.DialOptions will be
	// created. Note that the [websocket.DialOptions.HTTPClient] and its Transport
	// will be overridden with a [franchise.Transport] for authorization.
	Options *websocket.DialOptions

	// NetworkID specifies a unique ID for the network. If zero, a random value will
	// be automatically set from [rand.Uint64]. It is included in the URI for establishing
	// a WebSocket connection.
	NetworkID string

	// Log is used to logging messages at various levels. If nil, the default
	// [slog.Logger] will be set from [slog.Default].
	Log *slog.Logger
}

// DialContext establishes a Conn to the signaling service using the [oauth2.TokenSource] for
// authentication and authorization with franchise services. It obtains the necessary [franchise.Discovery]
// and [Environment] needed, then calls DialWithIdentityAndEnvironment internally. It is the
// method that is typically used when no configuration of identity and environment is required.
func (d Dialer) DialContext(ctx context.Context, mcToken *authservice.MCToken) (*Conn, error) {
	if d.Options == nil {
		d.Options = &websocket.DialOptions{}
	}
	if d.Options.HTTPClient == nil {
		d.Options.HTTPClient = &http.Client{}
	}
	if d.NetworkID == "" {
		d.NetworkID = strconv.FormatUint(rand.Uint64(), 10)
	}
	if d.Log == nil {
		d.Log = slog.Default()
	}

	if d.Options.HTTPHeader == nil {
		d.Options.HTTPHeader = make(http.Header)
	}
	d.Options.HTTPHeader.Set("Authorization", mcToken.AuthorizationHeader)

	c, _, err := websocket.Dial(ctx, d.Service.Config.Url("/ws/v1.0/signaling/%d", d.NetworkID), d.Options)
	if err != nil {
		return nil, err
	}

	conn := &Conn{
		conn: c,
		d:    d,

		credentialsReceived: make(chan struct{}),

		closed: make(chan struct{}),

		notifiers: make(map[uint32]nethernet.Notifier),
	}
	go conn.read()
	return conn, nil
}
