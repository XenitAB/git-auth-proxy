package server

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httputil"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/go-logr/logr"
	pkggin "github.com/xenitab/pkg/gin"

	"github.com/xenitab/git-auth-proxy/pkg/auth"
)

type Server struct {
	srv *http.Server
}

func NewServer(logger logr.Logger, addr string, authz *auth.Authorizer) *Server {
	router := pkggin.Default(logger, "proxy")
	router.GET("/readyz", readinessHandler)
	router.GET("/healthz", livenessHandler)
	router.NoRoute(proxyHandler(authz))
	// The ReadTimeout is set to 5 min make sure that strange requests don't live forever
	// But in general the external request should set a good timeout value for it's request.
	srv := &http.Server{ReadTimeout: 5 * time.Minute, Addr: addr, Handler: router}
	return &Server{
		srv: srv,
	}
}

func (s *Server) ListenAndServe() error {
	return s.srv.ListenAndServe()
}

func (s *Server) Shutdown(ctx context.Context) error {
	return s.srv.Shutdown(ctx)
}

func readinessHandler(c *gin.Context) {
	c.Status(http.StatusOK)
}

func livenessHandler(c *gin.Context) {
	c.Status(http.StatusOK)
}

func proxyHandler(authz *auth.Authorizer) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Get the token from the request
		token, err := getTokenFromRequest(c.Request)
		if err != nil {
			c.Header("WWW-Authenticate", "Basic realm=\"Restricted\"")
			c.String(http.StatusUnauthorized, "Missing basic authentication")
			return
		}
		// Check basic auth with local auth configuration
		err = authz.IsPermitted(c.Request.URL.EscapedPath(), token)
		if err != nil {
			//nolint: errcheck //ignore
			c.Error(fmt.Errorf("Received unauthorized request: %w", err))
			c.String(http.StatusForbidden, "User not permitted")
			return
		}
		// Authenticate the request with the proper token
		req, url, err := authz.UpdateRequest(c.Request.Context(), c.Request, token)
		if err != nil {
			//nolint: errcheck //ignore
			c.Error(fmt.Errorf("Could not authenticate request: %w", err))
			c.String(http.StatusInternalServerError, "Internal server error")
			return
		}

		// TODO (Philip): Add caching of the proxy
		// Forward the request to the correct proxy
		proxy := httputil.NewSingleHostReverseProxy(url)
		proxy.ServeHTTP(c.Writer, req)
	}
}
