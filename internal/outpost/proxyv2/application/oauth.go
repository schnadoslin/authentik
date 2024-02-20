package application

import (
	"net/http"
	"net/url"
	"strings"
	"time"

	"goauthentik.io/api/v3"
	"goauthentik.io/internal/outpost/proxyv2/constants"
)

const (
	redirectParam     = "rd"
	CallbackSignature = "X-authentik-auth-callback"
	LogoutSignature   = "X-authentik-logout"
)

func (a *Application) checkRedirectParam(r *http.Request) (string, bool) {
	rd := r.URL.Query().Get(redirectParam)
	if rd == "" {
		return "", false
	}
	u, err := url.Parse(rd)
	if err != nil {
		a.log.WithError(err).Warning("Failed to parse redirect URL")
		return "", false
	}
	// Check to make sure we only redirect to allowed places
	if a.Mode() == api.PROXYMODE_PROXY || a.Mode() == api.PROXYMODE_FORWARD_SINGLE {
		ext, err := url.Parse(a.proxyConfig.ExternalHost)
		if err != nil {
			return "", false
		}
		ext.Scheme = ""
		if !strings.Contains(u.String(), ext.String()) {
			a.log.WithField("url", u.String()).WithField("ext", ext.String()).Warning("redirect URI did not contain external host")
			return "", false
		}
	} else {
		if !strings.HasSuffix(u.Host, *a.proxyConfig.CookieDomain) {
			a.log.WithField("host", u.Host).WithField("dom", *a.proxyConfig.CookieDomain).Warning("redirect URI Host was not included in cookie domain")
			return "", false
		}
	}
	return u.String(), true
}

func (a *Application) handleAuthStart(rw http.ResponseWriter, r *http.Request) {
	s, _ := a.sessions.Get(r, a.SessionName())
	// Check if we already have a state in the session,
	// and if we do we don't do anything here
	currentState, ok := s.Values[constants.SessionOAuthState].(string)
	if ok {
		claims, err := a.checkAuth(rw, r)
		if err != nil && claims != nil {
			a.log.Trace("auth start request with existing authenticated session")
			a.redirect(rw, r)
			return
		}
		a.log.Trace("session already has state, sending redirect to current state")
		http.Redirect(rw, r, a.oauthConfig.AuthCodeURL(currentState), http.StatusFound)
		return
	}
	state, err := a.createState(r)
	if err != nil {
		a.log.WithError(err).Warning("failed to create state")
		return
	}
	s.Values[constants.SessionOAuthState] = state
	err = s.Save(r, rw)
	if err != nil {
		a.log.WithError(err).Warning("failed to save session")
	}
	http.Redirect(rw, r, a.oauthConfig.AuthCodeURL(state), http.StatusFound)
}

func (a *Application) handleAuthCallback(rw http.ResponseWriter, r *http.Request) {
	s, err := a.sessions.Get(r, a.SessionName())
	if err != nil {
		a.log.WithError(err).Trace("failed to get session")
	}
	state, ok := s.Values[constants.SessionOAuthState]
	if !ok {
		a.log.Warning("No state saved in session")
		a.redirect(rw, r)
		return
	}
	claims, err := a.redeemCallback(state.(string), r.URL, r.Context())
	if err != nil {
		a.log.WithError(err).Warning("failed to redeem code")
		rw.WriteHeader(400)
		// To prevent the user from just refreshing and cause more errors, delete
		// the state from the session
		delete(s.Values, constants.SessionOAuthState)
		err := s.Save(r, rw)
		if err != nil {
			a.log.WithError(err).Warning("failed to save session")
			rw.WriteHeader(400)
			return
		}
		return
	}
	s.Options.MaxAge = int(time.Until(time.Unix(int64(claims.Exp), 0)).Seconds())
	s.Values[constants.SessionClaims] = &claims
	err = s.Save(r, rw)
	if err != nil {
		a.log.WithError(err).Warning("failed to save session")
		rw.WriteHeader(400)
		return
	}
	a.redirect(rw, r)
}
