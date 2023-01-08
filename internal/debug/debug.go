package debug

import (
	"fmt"
	"net/http"
	"net/http/pprof"

	"github.com/gorilla/mux"
	log "github.com/sirupsen/logrus"
	"goauthentik.io/internal/config"
	"goauthentik.io/internal/utils/web"
)

func EnableDebugServer() {
	l := log.WithField("logger", "authentik.go_debugger")
	if !config.Get().Debug {
		l.Info("not enabling debug server, set `AUTHENTIK_DEBUG` to `true` to enable it.")
		return
	}
	h := mux.NewRouter()
	h.HandleFunc("/debug/pprof/", pprof.Index)
	h.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
	h.HandleFunc("/debug/pprof/profile", pprof.Profile)
	h.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
	h.HandleFunc("/debug/pprof/trace", pprof.Trace)
	h.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		h.Walk(func(route *mux.Route, router *mux.Router, ancestors []*mux.Route) error {
			tpl, err := route.GetPathTemplate()
			if err != nil {
				return nil
			}
			w.Write([]byte(fmt.Sprintf("<a href='%[1]s'>%[1]s</a><br>", tpl)))
			return nil
		})
	})
	go func() {
		l.WithField("listen", config.Get().Listen.Debug).Info("Starting Debug server")
		err := http.ListenAndServe(
			config.Get().Listen.Debug,
			web.NewLoggingHandler(l, nil)(h),
		)
		if l != nil {
			l.WithError(err).Warn("failed to start debug server")
		}
	}()
}
