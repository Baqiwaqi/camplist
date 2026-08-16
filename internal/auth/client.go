package auth

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"log"
	"net/http"

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
}

const SESSION_COOKIE_KEY = "auth-session"
const USER_ID_KEY = "userID"
const USER_NAME_KEY = "userName"

func New(cfg Config) (*Auth, error) {
	store := sessions.NewCookieStore([]byte(cfg.SessionKey))
	store.Options = &sessions.Options{
		Path:     "/",
		HttpOnly: true,
		Secure:   false, // dev only: http://localhost
		SameSite: http.SameSiteLaxMode,
		MaxAge:   600,
	}

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

func (a *Auth) RequireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ses, _ := a.cookieStore.Get(r, SESSION_COOKIE_KEY)

		id, ok := ses.Values[USER_ID_KEY].(string)
		if !ok {
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}

		name, _ := ses.Values[USER_NAME_KEY].(string)

		ctx := context.WithValue(r.Context(), USER_NAME_KEY, name)
		ctx = context.WithValue(ctx, USER_ID_KEY, id)

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
	if err := ses.Save(r, w); err != nil {
		log.Printf("Saveing session errorr: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, a.oauthCfg.AuthCodeURL(state), http.StatusFound)
}

func (a *Auth) DeleteHandler(w http.ResponseWriter, r *http.Request) {
	ses, err := a.cookieStore.Get(r, SESSION_COOKIE_KEY)
	if err != nil {
		log.Printf("Error getting cookie store %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

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

	delete(ses.Values, "state")
	if err := ses.Save(r, w); err != nil {
		log.Printf("error setting userID")
		http.Error(w, "Login failed", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/", http.StatusSeeOther)

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
