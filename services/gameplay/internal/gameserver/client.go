package gameserver

import (
	"context"
	"encoding/json"
	"sync/atomic"
	"time"

	"github.com/spazzle-io/spazzle-api/services/gameplay/internal/gameevents"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

const (
	WriteWait             = 5 * time.Second
	MaxMessageSize        = 1 << 18 // 0.25 MB
	ClientSendChanBufSize = 256
)

type TimingProfile struct {
	PongWait   time.Duration
	PingPeriod time.Duration
}

var (
	DefaultTiming = TimingProfile{
		PongWait:   50 * time.Second,
		PingPeriod: 45 * time.Second,
	}
	AggressiveTiming = TimingProfile{
		PongWait:   10 * time.Second,
		PingPeriod: 9 * time.Second,
	}
)

type Client struct {
	userID        uuid.UUID
	connID        uuid.UUID
	gameServer    *GameServer
	conn          *websocket.Conn
	send          chan *OutgoingMessage
	isSpectating  bool
	timingUpdate  chan TimingProfile
	currentTiming atomic.Pointer[TimingProfile]
}

type clientOptions struct {
	disablePumps bool
}

type ClientOption func(*clientOptions)

func WithoutPumps() ClientOption {
	return func(o *clientOptions) {
		o.disablePumps = true
	}
}

func NewClient(
	ctx context.Context,
	gameServer *GameServer,
	conn *websocket.Conn,
	userID uuid.UUID,
	isSpectating bool,
	opts ...ClientOption,
) (*Client, error) {
	options := &clientOptions{}
	for _, opt := range opts {
		opt(options)
	}

	client := Client{
		gameServer:   gameServer,
		conn:         conn,
		send:         make(chan *OutgoingMessage, ClientSendChanBufSize),
		userID:       userID,
		connID:       uuid.New(),
		isSpectating: isSpectating,
		timingUpdate: make(chan TimingProfile, 1),
	}

	defaultTiming := DefaultTiming
	client.currentTiming.Store(&defaultTiming)

	if !options.disablePumps {
		go client.readPump(ctx)
		go client.writePump(ctx)
	}

	client.getLogger().Info().Msg("created ws client")

	return &client, nil
}

func (c *Client) getLogger() *zerolog.Logger {
	logger := log.With().
		Str("user_id", c.userID.String()).
		Str("conn_id", c.connID.String()).
		Str("server_id", c.gameServer.GetServerId().String()).
		Logger()

	return &logger
}

func (c *Client) UpdateTiming(profile TimingProfile) {
	select {
	case c.timingUpdate <- profile:
	default:
		c.getLogger().Debug().Msg("timing update dropped, client may be disconnected")
	}
}

func (c *Client) readPump(ctx context.Context) {
	defer func() {
		_ = c.gameServer.Unregister(c)
		_ = c.conn.Close()
	}()

	c.conn.SetReadLimit(MaxMessageSize)
	err := c.conn.SetReadDeadline(time.Now().UTC().Add(c.currentTiming.Load().PongWait))
	if err != nil {
		c.getLogger().Warn().Err(err).Msg("failed to set ws read deadline")
		return
	}

	c.conn.SetPongHandler(func(string) error {
		err := c.conn.SetReadDeadline(time.Now().UTC().Add(c.currentTiming.Load().PongWait))
		if err != nil {
			c.getLogger().Warn().Err(err).Msg("failed to set ws read deadline on pong")
			return err
		}
		return nil
	})

	for {
		select {
		case <-ctx.Done():
			return
		default:
			_, message, err := c.conn.ReadMessage()
			if err != nil {
				if websocket.IsUnexpectedCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) {
					c.getLogger().Warn().Err(err).Msg("unexpected ws read error")
				} else {
					c.getLogger().Debug().Msg("client disconnected normally")
				}
				return
			}

			c.gameServer.handleClientWsMessage(c, message)
		}
	}
}

func (c *Client) writePump(ctx context.Context) {
	ticker := time.NewTicker(c.currentTiming.Load().PingPeriod)

	defer func() {
		ticker.Stop()
		_ = c.conn.Close()
	}()

	for {
		select {
		case outgoingMsg, ok := <-c.send:
			err := c.conn.SetWriteDeadline(time.Now().UTC().Add(WriteWait))
			if err != nil {
				c.getLogger().Warn().Err(err).Msg("failed to set ws write deadline")
				return
			}

			if !ok {
				c.getLogger().Debug().Msg("client send channel closed, disconnecting")
				_ = c.conn.WriteMessage(websocket.CloseMessage, nil)
				return
			}

			w, err := c.conn.NextWriter(websocket.TextMessage)
			if err != nil {
				c.getLogger().Warn().Err(err).Msg("failed to get ws writer")
				return
			}

			messagePayload, err := json.Marshal(outgoingMsg.WsMessage)
			if err != nil {
				c.getLogger().Warn().Err(err).Msg("failed to marshal ws message payload")
				return
			}

			_, err = w.Write(messagePayload)
			if err != nil {
				c.getLogger().Warn().Err(err).Msg("failed to send message to client ws connection")
				if outgoingMsg.RequiresWorkflowAck {
					ackReason := "failed to send message to client ws connection"
					c.gameServer.ackGameEvent(outgoingMsg.CorrelationID, gameevents.AckStatusFailed, ackReason)
				}
				return
			}

			if outgoingMsg.RequiresWorkflowAck {
				ackReason := "sent message to client ws connection"
				c.gameServer.ackGameEvent(outgoingMsg.CorrelationID, gameevents.AckStatusDelivered, ackReason)
			}

			if err := w.Close(); err != nil {
				c.getLogger().Warn().Err(err).Msg("failed to close client ws writer")
				return
			}

		case newTiming := <-c.timingUpdate:
			timing := newTiming
			c.currentTiming.Store(&timing)
			ticker.Reset(timing.PingPeriod)

			err := c.conn.SetWriteDeadline(time.Now().UTC().Add(WriteWait))
			if err != nil {
				c.getLogger().Warn().Err(err).Msg("failed to set ws write deadline after timing update")
				return
			}

			err = c.conn.WriteMessage(websocket.PingMessage, nil)
			if err != nil {
				c.getLogger().Warn().Err(err).Msg("failed to send ping to client ws connection after timing update")
				return
			}

			err = c.conn.SetReadDeadline(time.Now().UTC().Add(timing.PongWait))
			if err != nil {
				c.getLogger().Warn().Err(err).Msg("failed to set ws read deadline after timing update")
				return
			}

			c.getLogger().Info().
				Dur("pong_wait", timing.PongWait).
				Dur("ping_period", timing.PingPeriod).
				Msg("updated client timing")

		case <-ticker.C:
			err := c.conn.SetWriteDeadline(time.Now().UTC().Add(WriteWait))
			if err != nil {
				c.getLogger().Warn().Err(err).Msg("failed to set ws write deadline")
				return
			}

			err = c.conn.WriteMessage(websocket.PingMessage, nil)
			if err != nil {
				c.getLogger().Warn().Err(err).Msg("failed to send ping to client ws connection")
				return
			}

		case <-ctx.Done():
			return
		}
	}
}
