package handlers

import (
	"bytes"
	"log"
	"time"

	"net/http"

	"github.com/cvhariharan/autopilot/internal/ui"
	"github.com/gorilla/websocket"
	"github.com/labstack/echo/v4"
	"github.com/redis/go-redis/v9"
)

var (
	upgrader = websocket.Upgrader{}
)

// HandleLogStreaming uses SSE to stream action logs
func (h *Handler) HandleLogStreaming(c echo.Context) error {
	// Upgrade to WebSocket connection
	ws, err := upgrader.Upgrade(c.Response(), c.Request(), nil)
	if err != nil {
		return err
	}
	defer ws.Close()

	lastProcessedID := "0"

	for {
		// Read from Redis stream starting from the last processed ID
		result, err := h.redisClient.XRead(c.Request().Context(), &redis.XReadArgs{
			Streams: []string{c.Param("flow"), lastProcessedID},
			Count:   10,
			Block:   0, // Block indefinitely until new messages arrive
		}).Result()

		// Handle potential errors
		if err != nil {
			if err == redis.Nil {
				// No new messages, continue waiting
				continue
			}
			log.Println("Redis read error:", err)
			return err
		}

		// Process each stream
		for _, stream := range result {
			for _, message := range stream.Messages {
				if _, close := message.Values["closed"]; close {
					ws.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(int(http.StateClosed), "Connection closed"))
					return nil
				}

				var buf bytes.Buffer
				if err := ui.LogMessage(message.Values["log"].(string)).Render(c.Request().Context(), &buf); err != nil {
					return err
				}

				if err := ws.WriteMessage(websocket.TextMessage, buf.Bytes()); err != nil {
					return err
				}

				lastProcessedID = message.ID
			}
		}

		time.Sleep(1 * time.Second)
	}
}
