package auth

import (
	"context"
	"log"

	"github.com/labstack/echo/v4"
)

func (h *AuthHandler) Authenticate(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		sess, err := h.sessMgr.Acquire(nil, c, c)
		if err != nil {
			c.Logger().Error(err)
			return h.handleUnauthenticated(c)
		}

		user, err := sess.Get("user")
		if err != nil {
			c.Logger().Error(err)
			return h.handleUnauthenticated(c)
		}

		method, err := sess.String(sess.Get("method"))
		if err != nil {
			c.Logger().Infof("could not get method: %v", err)
		}

		// if using oidc, validate the token to check if they have not expired
		if method == "oidc" {
			rawIDToken, err := sess.Get("id_token")
			if err != nil || rawIDToken == nil {
				return h.handleUnauthenticated(c)
			}

			_, err = h.authconfig.verifier.Verify(context.Background(), rawIDToken.(string))
			if err != nil {
				log.Println(err)
				sess.Delete("method")
				sess.Delete("id_token")
				sess.Delete("user")
				return h.handleUnauthenticated(c)
			}
		}

		c.Set("user", user)

		return next(c)
	}
}
