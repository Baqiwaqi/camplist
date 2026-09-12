package auth

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"regexp"
	"strings"

	"github.com/coreos/go-oidc"
	"github.com/gorilla/sessions"
	"golang.org/x/oauth2"
)

type Auth struct {
	oauthCfg    *oauth2.Config
	verifier    *oidc.IDTokenVerifier
	cookieStore *sessions.CookieStore
}

type Config struct {
	ClientID     string
	ClientSecret string
	RedirectURL  string
	SessionKey   string
	CookieSecure bool
}

const SESSION_COOKIE_KEY = "auth-session"
const USER_ID_KEY = "userID"
const USER_NAME_KEY = "userName"
const sessionMaxAge = 365 * 24 * 60 * 60

func New(cfg Config) (*Auth, error) {
	store := newCookieStore(cfg.SessionKey, cfg.CookieSecure)

	provider, err := oidc.NewProvider(context.TODO(), "https://accounts.google.com")
	if err != nil {
		return &Auth{}, err
	}

	// Configure an OpenID Connect aware OAuth2 client.
	oauthCfg := oauth2.Config{
		ClientID:     cfg.ClientID,
		ClientSecret: cfg.ClientSecret,
		RedirectURL:  cfg.RedirectURL,

		// Discovery returns the OAuth2 endpoints.
		Endpoint: provider.Endpoint(),

		// "openid" is a required scope for OpenID Connect flows.
		Scopes: []string{oidc.ScopeOpenID, "email", "profile"},
	}

	verifier := provider.Verifier(&oidc.Config{ClientID: oauthCfg.ClientID})

	auth := &Auth{
		oauthCfg:    &oauthCfg,
		verifier:    verifier,
		cookieStore: store,
	}

	return auth, nil
}

func newCookieStore(sessionKey string, secure bool) *sessions.CookieStore {
	store := sessions.NewCookieStore([]byte(sessionKey))
	store.Options = &sessions.Options{
		Path:     "/",
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   sessionMaxAge,
	}
	// CookieStore also enforces MaxAge while decoding signed cookies.
	store.MaxAge(sessionMaxAge)
	return store
}

// sessionContext carries the signed-in user from the cookie session into the
// request context. ok is false when nobody is signed in.
func (a *Auth) sessionContext(r *http.Request) (context.Context, bool) {
	ses, _ := a.cookieStore.Get(r, SESSION_COOKIE_KEY)

	id, ok := ses.Values[USER_ID_KEY].(string)
	if !ok || id == "" {
		return r.Context(), false
	}

	name, _ := ses.Values[USER_NAME_KEY].(string)
	email, _ := ses.Values["email"].(string)
	verified, _ := ses.Values["emailVerified"].(bool)

	ctx := context.WithValue(r.Context(), USER_NAME_KEY, name)
	ctx = context.WithValue(ctx, USER_ID_KEY, id)
	ctx = context.WithValue(ctx, profileKey{}, Profile{Subject: id, Name: name, Email: email, EmailVerified: verified})
	return ctx, true
}

// OptionalAuth loads the signed-in user when there is one and lets visitors
// through. Pages that look different for visitors and members use it.
func (a *Auth) OptionalAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		ctx, _ := a.sessionContext(r)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func (a *Auth) RequireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		ctx, ok := a.sessionContext(r)
		if !ok {
			if strings.HasPrefix(r.URL.Path, "/api/") {
				w.Header().Set("Content-Type", "application/json")
				w.Header().Set("Cache-Control", "no-store")
				w.WriteHeader(http.StatusUnauthorized)
				w.Write([]byte(`{"code":"signin"}`))
				return
			}
			if r.Header.Get("HX-Request") == "true" {
				w.Header().Set("HX-Redirect", "/login")
				w.WriteHeader(http.StatusOK)
				return
			}
			if strings.HasPrefix(r.URL.Path, "/join/") {
				http.Redirect(w, r, "/auth/login?returnTo="+url.QueryEscape(r.URL.Path), http.StatusSeeOther)
				return
			}
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func UserID(ctx context.Context) (string, error) {
	id, ok := ctx.Value(USER_ID_KEY).(string)
	if !ok {
		return "", fmt.Errorf("Error getting userID")
	}

	return id, nil
}

func UserName(ctx context.Context) string {
	name, _ := ctx.Value(USER_NAME_KEY).(string)
	return name
}

func (a *Auth) LoginHandler(w http.ResponseWriter, r *http.Request) {
	state, err := generateRandomState()
	if err != nil {
		log.Printf("generating state error: %v", err)
		http.Error(w, "Err generating random state", http.StatusInternalServerError)
		return
	}

	ses, _ := a.cookieStore.Get(r, SESSION_COOKIE_KEY)

	ses.Values["state"] = state
	ses.Values["returnTo"] = safeReturn(r.URL.Query().Get("returnTo"))
	if err := ses.Save(r, w); err != nil {
		log.Printf("Saveing session errorr: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	options := []oauth2.AuthCodeOption{}
	if r.URL.Query().Get("switch") == "1" {
		options = append(options, oauth2.SetAuthURLParam("prompt", "select_account"))
	}
	http.Redirect(w, r, a.oauthCfg.AuthCodeURL(state, options...), http.StatusFound)
}

func (a *Auth) DeleteHandler(w http.ResponseWriter, r *http.Request) {
	// CookieStore returns a fresh session even when decoding an old cookie fails.
	// Signing out must still expire that cookie.
	ses, _ := a.cookieStore.Get(r, SESSION_COOKIE_KEY)

	ses.Options.MaxAge = -1

	if err := ses.Save(r, w); err != nil {
		log.Printf("Error getting cookie store %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/login", http.StatusFound)
}

func (a *Auth) CallbackHandler(w http.ResponseWriter, r *http.Request) {
	err := r.URL.Query().Get("error")
	if err != "" {
		log.Printf("exchange code: %v", err)
		http.Error(w, "Login failed", http.StatusInternalServerError)
		log.Printf("err state in callback")
		return
	}

	state := r.URL.Query().Get("state")
	ses, _ := a.cookieStore.Get(r, SESSION_COOKIE_KEY)
	if state != ses.Values["state"] {
		log.Printf("state no eque state %v, %v ", state, ses.Values["state"])
		http.Error(w, "Login failed", http.StatusInternalServerError)
		return
	}

	oauthToken, exErr := a.oauthCfg.Exchange(r.Context(), r.URL.Query().Get("code"))
	if exErr != nil {
		log.Printf("exchange code: %v", exErr)
		http.Error(w, "Login failed", http.StatusInternalServerError)
		return
	}

	rawIDToken, ok := oauthToken.Extra("id_token").(string)
	if !ok {
		log.Printf("get id_token error")
		http.Error(w, "Login failed", http.StatusInternalServerError)
		return
	}

	idToken, vErr := a.verifier.Verify(r.Context(), rawIDToken)
	if vErr != nil {
		// handle error
		log.Printf("Verify token error: %v", vErr)
		http.Error(w, "Login failed", http.StatusInternalServerError)
		return
	}

	var claims Claims

	if err := idToken.Claims(&claims); err != nil {
		// handle error
		log.Printf("populate claims %v", err)
		http.Error(w, "Login failed", http.StatusInternalServerError)
		return
	}

	if err := claims.Valid(); err != nil {
		log.Print(err)
		http.Error(w, "Login failed", http.StatusInternalServerError)
		return
	}
	ses.Values[USER_ID_KEY] = claims.Sub
	ses.Values[USER_NAME_KEY] = claims.Name
	ses.Values["email"] = claims.Email
	ses.Values["emailVerified"] = claims.EmailVerified
	returnTo, _ := ses.Values["returnTo"].(string)
	delete(ses.Values, "returnTo")

	delete(ses.Values, "state")
	if err := ses.Save(r, w); err != nil {
		log.Printf("error setting userID")
		http.Error(w, "Login failed", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, safeReturn(returnTo), http.StatusSeeOther)

}

func generateRandomState() (string, error) {
	b := make([]byte, 32)
	_, err := rand.Read(b)
	if err != nil {
		return "", err
	}

	state := base64.RawURLEncoding.EncodeToString(b)

	return state, nil
}

func Subject(ctx context.Context) string { subject, _ := UserID(ctx); return subject }

type profileKey struct{}
type Profile struct {
	Subject, Name, Email string
	EmailVerified        bool
}

func UserProfile(ctx context.Context) Profile {
	if p, ok := ctx.Value(profileKey{}).(Profile); ok {
		return p
	}
	return Profile{Subject: Subject(ctx), Name: UserName(ctx)}
}

var invitationReturn = regexp.MustCompile(`^/join/[A-Za-z0-9_-]{1,200}/packing-(session|list)/[A-Za-z0-9_-]{1,100}/[A-Za-z0-9_-]{43}$`)

func safeReturn(path string) string {
	if invitationReturn.MatchString(path) {
		return path
	}
	return "/"
}
