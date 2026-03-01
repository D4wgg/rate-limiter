package proxy

import (
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"

	"github.com/go-chi/chi/v5"
	"go.uber.org/zap"

	"github.com/example/rate-limiter/internal/config"
	"github.com/example/rate-limiter/internal/limiter"
)

type Handler struct {
	router  chi.Router
	limiter limiter.Limiter
	logger  *zap.Logger
}

func NewHandler(cfg *config.Config, l limiter.Limiter, logger *zap.Logger) (*Handler, error) {
	r := chi.NewRouter()
	h := &Handler{
		router:  r,
		limiter: l,
		logger:  logger,
	}

	// Health check endpoints для Kubernetes
	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("OK"))
	})

	r.Get("/ready", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("OK"))
	})

	for _, rt := range cfg.Routes {
		upstreamURL, err := url.Parse(rt.Upstream)
		if err != nil {
			return nil, err
		}

		proxy := httputil.NewSingleHostReverseProxy(upstreamURL)
		routePath := rt.Route
		methods := rt.Methods
		if len(methods) == 0 {
			methods = []string{http.MethodGet, http.MethodPost, http.MethodPut, http.MethodDelete, http.MethodPatch}
		}

		for _, m := range methods {
			method := strings.ToUpper(m)
			h.registerRoute(method, routePath, proxy, rt)
		}
	}

	return h, nil
}

func (h *Handler) registerRoute(method, route string, proxy *httputil.ReverseProxy, rt config.RouteConfig) {
	h.router.Method(method, route, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		// Key per route+method; can be extended to include client id, API key, etc.
		key := "rl:" + method + ":" + route

		allowed, err := h.limiter.Allow(ctx, key, rt.Limit.Window, rt.Limit.RPS)
		if err != nil {
			h.logger.Error("rate limiter error",
				zap.Error(err),
				zap.String("method", method),
				zap.String("route", route),
				zap.String("key", key),
			)
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}

		if !allowed {
			h.logger.Info("rate limit exceeded",
				zap.String("method", method),
				zap.String("route", route),
				zap.String("remote_addr", r.RemoteAddr),
				zap.Int("limit", rt.Limit.RPS),
			)
			w.Header().Set("Retry-After", "1")
			w.WriteHeader(http.StatusTooManyRequests)
			_, _ = w.Write([]byte(http.StatusText(http.StatusTooManyRequests)))
			return
		}

		// Логируем успешный запрос на уровне debug
		h.logger.Debug("proxying request",
			zap.String("method", method),
			zap.String("route", route),
			zap.String("remote_addr", r.RemoteAddr),
		)

		proxy.ServeHTTP(w, r)
	}))
}

func (h *Handler) Router() http.Handler {
	return h.router
}

