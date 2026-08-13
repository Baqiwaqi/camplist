package auth

import (
	"context"
	"crypto/rand"
	"encoding/base64"
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

func (a *Auth) LoginHandler(w http.ResponseWriter, r *http.Request) {
	state, err := generateRandomState()
	if err != nil {
		log.Printf("generating state error: %v", err)
		http.Error(w, "Err generating random state", http.StatusInternalServerError)
		return
	}

	ses, _ := a.cookieStore.Get(r, "auth-session")
	ses.Values["state"] = state
	if err := ses.Save(r, w); err != nil {
		log.Printf("Saveing session errorr: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, a.oauthCfg.AuthCodeURL(state), http.StatusFound)
}

func (a *Auth) CallbackHandler(w http.ResponseWriter, r *http.Request) {
	authErr := r.URL.Query().Get("error")
	if authErr != "" {
		log.Printf("err state in callback")
		return
	}

	state := r.URL.Query().Get("state")
	ses, _ := a.cookieStore.Get(r, "auth-session")
	if state != ses.Values["state"] {
		log.Printf("state no eque state %v, %v ", state, ses.Values["state"])
		return
	}

	oauthToken, err := a.oauthCfg.Exchange(r.Context(), r.URL.Query().Get("code"))
	if err != nil {
		log.Printf("exchnage code error")
		return
	}

	rawIDToken, ok := oauthToken.Extra("id_token").(string)
	if !ok {
		log.Printf("get id_token error")
		return
	}

	idToken, err := a.verifier.Verify(r.Context(), rawIDToken)
	if err != nil {
		// handle error
		return
	}

	var claims struct {
		Sub           string `json:"sub"`
		Email         string `json:"email"`
		EmailVerified bool   `json:"email_verified"`
		Name          string `json:"name"`
		Picture       string `json:"picture"`
	}
	if err := idToken.Claims(&claims); err != nil {
		// handle error
		return
	}

	log.Printf("claims %v", claims)
	ses.Values["userID"] = claims.Sub

	_, ok = ses.Values["userID"].(string)
	if !ok {
		//handle err

		return
	}
	ses.Values["name"] = claims.Name

	delete(ses.Values, "state")
	if err := ses.Save(r, w); err != nil {
		// handle error
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
