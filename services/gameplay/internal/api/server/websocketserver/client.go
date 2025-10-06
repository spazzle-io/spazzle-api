package websocketserver

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

const (
	WriteWait             = 5 * time.Second
	PongWait              = 50 * time.Second
	PingPeriod            = (PongWait * 9) / 10 // 45 seconds
	MaxMessageSize        = 1 << 18             // 0.25 MB
	ClientSendChanBufSize = 256
)

type Client struct {
	userId     uuid.UUID
	connId     uuid.UUID
	gameServer *GameServer
	conn       *websocket.Conn
	send       chan []byte
}

func NewClient(gameServer *GameServer, conn *websocket.Conn, userId uuid.UUID) (*Client, error) {
	connId, err := uuid.NewRandom()
	if err != nil {
		log.Error().
			Err(err).
			Str("user_id", userId.String()).
			Msg("failed to generate ws client connection id")
		return nil, err
	}

	client := Client{
		gameServer: gameServer,
		conn:       conn,
		send:       make(chan []byte, ClientSendChanBufSize),
		userId:     userId,
		connId:     connId,
	}

	client.getLogger().Info().Msg("created ws client")

	return &client, nil
}

func (c *Client) getLogger() *zerolog.Logger {
	logger := log.With().
		Str("user_id", c.userId.String()).
		Str("conn_id", c.connId.String()).
		Str("server_id", c.gameServer.serverId.String()).
		Logger()

	return &logger
}

func (c *Client) readPump(ctx context.Context) {
	defer func() {
		_ = c.gameServer.Unregister(c)
		_ = c.conn.Close()
	}()

	c.conn.SetReadLimit(MaxMessageSize)
	err := c.conn.SetReadDeadline(time.Now().UTC().Add(PongWait))
	if err != nil {
		c.getLogger().Warn().Err(err).Msg("failed to set ws read deadline")
		return
	}

	c.conn.SetPongHandler(func(string) error {
		err := c.conn.SetReadDeadline(time.Now().UTC().Add(PongWait))
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

			// TODO: Handle different message types
			err = c.gameServer.Broadcast(message)
			if err != nil {
				c.getLogger().Warn().Err(err).Msg("failed to broadcast ws message")
				return
			}
		}
	}
}

func (c *Client) writePump(ctx context.Context) {
	ticker := time.NewTicker(PingPeriod)

	defer func() {
		ticker.Stop()
		_ = c.conn.Close()
	}()

	for {
		select {
		case message, ok := <-c.send:
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

			_, err = w.Write(message)
			if err != nil {
				c.getLogger().Warn().Err(err).Msg("failed to send message to client ws connection")
				return
			}

			if err := w.Close(); err != nil {
				c.getLogger().Warn().Err(err).Msg("failed to close client ws writer")
				return
			}
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
