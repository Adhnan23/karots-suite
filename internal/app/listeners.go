package app

import (
	"context"
	"errors"
	"log"
	"net/http"

	"karots-suite/internal/plugin"

	"github.com/labstack/echo/v4"
)

// ServeListeners serves the app on each plugin listener in the background. A
// port that can't be bound is logged, never fatal: the till must still start.
// shutdown stops them all.
func ServeListeners(e *echo.Echo, ls []plugin.Listener) (shutdown func(context.Context) error) {
	srvs := make([]*http.Server, 0, len(ls))
	for _, l := range ls {
		srv := &http.Server{Addr: l.Addr, Handler: e, TLSConfig: l.TLS}
		srvs = append(srvs, srv)
		go func() {
			log.Printf("%s: also listening on %s (HTTPS)", l.Key, l.Addr)
			if err := srv.ListenAndServeTLS("", ""); err != nil && !errors.Is(err, http.ErrServerClosed) {
				log.Printf("%s: can't listen on %s: %v", l.Key, l.Addr, err)
			}
		}()
	}
	return func(ctx context.Context) error {
		var errs []error
		for _, s := range srvs {
			errs = append(errs, s.Shutdown(ctx))
		}
		return errors.Join(errs...)
	}
}
