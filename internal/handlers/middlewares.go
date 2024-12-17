package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"log"

	"github.com/labstack/echo/v4"
)

func (h *Handler) Authenticate(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		sess, err := h.sessMgr.Acquire(nil, c, c)
		if err != nil {
			log.Println(err)
			return h.handleUnauthenticated(c)
		}

		rawIDToken, err := sess.Get("id_token")
		if err != nil {
			log.Println(err)
			return h.handleUnauthenticated(c)
		}

		// Verify the token
		_, err = h.authconfig.verifier.Verify(context.Background(), rawIDToken.(string))
		if err != nil {
			log.Println(err)
			sess.Delete("id_token")
			sess.Delete("user")
			return h.handleUnauthenticated(c)
		}

		// Set user info in context
		if userInfo, err := sess.Get("user"); err == nil {
			var user UserInfo
			userBytes, err := json.Marshal(userInfo)
			if err != nil {
				c.Logger().Error(err)
				return h.handleUnauthenticated(c)
			}

			if err := json.NewDecoder(bytes.NewBuffer(userBytes)).Decode(&user); err != nil {
				c.Logger().Error(err)
				return h.handleUnauthenticated(c)
			}

			c.Set("user", user)
			return next(c)
		}

		log.Println(err)
		return h.handleUnauthenticated(c)
	}
}
