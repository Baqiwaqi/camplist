package main

import (
	db "camplist/internal"
	"camplist/internal/auth"
	"camplist/internal/packing"
	"camplist/internal/web"
	"context"
	"errors"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/joho/godotenv"
)

func main() {
	if err := godotenv.Load(); err != nil {
		log.Println("no .env file found")
	}
	endpoint := os.Getenv("DB_URL")
	key := os.Getenv("DB_KEY")

	if endpoint == "" {
		log.Fatal("DB_URL is required")
	}

	if key == "" {
		log.Fatal("DB_KEY is required")
	}

	client, err := db.NewClient(endpoint, key, "dev", "packing_list")
	if err != nil {
		log.Fatal(err)
	}
	packingStore := packing.NewStore(client.Container())

	clientID := os.Getenv("GOOGLE_CLIENT_ID")
	clientSecret := os.Getenv("GOOGLE_CLIENT_SECRET")
	if clientID == "" || clientSecret == "" {
		log.Fatal("Missing auth client credentials")
	}

	redirectURL := os.Getenv("REDIRECT_URL")
	if redirectURL == "" {
		log.Fatal("redirect url is missing from environment variables")
	}

	callbackURL, err := url.Parse(redirectURL)
	if err != nil || callbackURL.Host == "" || (callbackURL.Scheme != "http" && callbackURL.Scheme != "https") {
		log.Fatal("REDIRECT_URL must be an absolute http or https URL")
	}
	secureCookies := callbackURL.Scheme == "https"

	sessionKey := os.Getenv("SESSION_KEY")
	if sessionKey == "" {
		log.Fatal("session key missing from environment variables")
	}

	auth, err := auth.New(auth.Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		RedirectURL:  redirectURL,
		SessionKey:   sessionKey,
		CookieSecure: secureCookies,
	})
	if err != nil {
		log.Fatal(err)
	}

	r := web.Routes(web.Config{
		PackingStore: packingStore,
		Auth:         auth,
	})

	csrfKey := os.Getenv("CSRF_KEY")

	if csrfKey == "" {
		log.Fatal("CSRF_KEY is required")
	}

	csrfMiddleware := web.CSRFProtection([]byte(csrfKey), secureCookies, callbackURL.Host)

	server := &http.Server{
		Addr:              ":3000",
		Handler:           csrfMiddleware(r),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	serverErrors := make(chan error, 1)
	go func() { serverErrors <- server.ListenAndServe() }()
	log.Println("Camplist listening on :3000")
	select {
	case err := <-serverErrors:
		if !errors.Is(err, http.ErrServerClosed) {
			log.Fatal(err)
		}
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			log.Printf("server shutdown: %v", err)
		}
	}

}
