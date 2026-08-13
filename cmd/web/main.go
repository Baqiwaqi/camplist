package main

import (
	db "camplist/internal"
	"camplist/internal/auth"
	"camplist/internal/packing"
	"camplist/internal/web"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/gorilla/csrf"
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

	sessionKey := os.Getenv("SESSION_KEY")
	if sessionKey == "" {
		log.Fatal("session key missing from environment variables")
	}

	auth, err := auth.New(auth.Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		RedirectURL:  redirectURL,
		SessionKey:   sessionKey,
	})
	if err != nil {
		log.Fatal(err)
	}

	r := web.Routes(web.Config{
		PackingStore: packingStore,
		Auth:         auth,
	})

	r.Post("/add-packing-item", func(w http.ResponseWriter, r *http.Request) {
		var req packing.AddPackingItem
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}

		item := packing.NewItem(req.Name, req.Category)

		err := packingStore.AddItem(r.Context(), req.PackingID, packing.DemoUserID, item)
		if err != nil {
			log.Printf("add item to packing list: %v", err)
			http.Error(w, "Storing item on packing list failed", http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusCreated)
		fmt.Fprintln(w, "add item to packing list")
	})

	r.Post("/remove-packing-item", func(w http.ResponseWriter, r *http.Request) {
		var req packing.RemovePackingItem

		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}

		err := packingStore.RemoveItem(r.Context(), req.PackingID, packing.DemoUserID, req.ItemID)
		if err != nil {
			log.Printf("remove item from packing list: %v", err)
			http.Error(w, "Removing item off packing list failed", http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusOK)
		fmt.Fprintln(w, "item removed")
	})

	csrfKey := os.Getenv("CSRF_KEY")

	if csrfKey == "" {
		log.Fatal("CSRF_KEY is required")
	}

	csrfMiddleware := csrf.Protect(
		[]byte(csrfKey),
		csrf.Secure(false), // dev only: allow the CSRF cookie over http://localhost
		csrf.TrustedOrigins([]string{
			"localhost:3000",
		}),
		csrf.FieldName("_csrf"),
	)
	http.ListenAndServe(":3000", csrfMiddleware(r))

}
