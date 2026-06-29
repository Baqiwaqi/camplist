package main

import (
	"bytes"
	db "camplist/internal"
	"camplist/internal/packing"
	"camplist/internal/views"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"text/tabwriter"

	"github.com/go-chi/chi/middleware"
	"github.com/go-chi/chi/v5"
	"github.com/gorilla/csrf"
	"github.com/gorilla/schema"
	"github.com/joho/godotenv"
)

type Item struct {
	ID        int
	Name      string
	Catergory string
	Checked   bool
}

func addItem(items []Item, item Item) []Item {
	return append(items, item)
}

func toggleItemByID(items []Item, id int) error {
	for i, item := range items {
		// return when we find the item
		if item.ID == id {
			items[i].Checked = !items[i].Checked
			return nil
		}
	}
	return fmt.Errorf("item id not found ")
}

func renameItemByID(items []Item, id int, name string) error {
	for i, item := range items {
		if item.ID == id {
			items[i].Name = name
			return nil
		}
	}

	return fmt.Errorf("error renaming ")
}

func removeItem(items []Item, id int) ([]Item, error) {
	for i, item := range items {
		if item.ID == id {
			return append(items[:i], items[i+1:]...), nil
		}
	}
	return items, fmt.Errorf("item id not found")
}

func printItems(items []Item) {
	var buf bytes.Buffer
	tw := tabwriter.NewWriter(&buf, 0, 0, 1, ' ', 0)
	fmt.Fprintln(tw, "ID\tName\tCategory\tChecked")
	for _, item := range items {
		fmt.Fprintf(tw, "%d\t%s\t%s\t%t\n", item.ID, item.Name, item.Catergory, item.Checked)
	}
	tw.Flush()
	fmt.Print(buf.String())
}

func filterByCategory(items []Item, category string) []Item {
	var filteredItems []Item
	for _, item := range items {
		if item.Catergory == category {
			filteredItems = append(filteredItems, item)
		}
	}
	return filteredItems
}

func printItemsByCategory(items []Item, category string) {
	filteredItems := filterByCategory(items, category)
	printItems(filteredItems)
}

func nextID(items []Item) int {
	maxID := 0

	// loop though the items
	for _, item := range items {
		if item.ID > maxID {
			maxID = item.ID
		}
	}

	return maxID + 1
}

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

	r := chi.NewRouter()
	r.Use(middleware.Logger)

	fileServer := http.FileServer(http.Dir("static"))
	r.Handle("/static/*", http.StripPrefix("/static/", fileServer))

	r.Get("/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("Hello, World!"))
	})

	r.Get("/ui", func(w http.ResponseWriter, r *http.Request) {
		lists, err := packingStore.GetPackingLists(r.Context(), packing.DemoUserID)
		if err != nil {
			log.Printf("render ui: %v", err)
			http.Error(w, "render failed", http.StatusInternalServerError)
			return
		}

		if err := views.PackingListPage("Camping", lists).Render(r.Context(), w); err != nil {
			log.Printf("render ui: %v", err)
		}
	})

	r.Get("/list/{id}", func(w http.ResponseWriter, r *http.Request) {
		id := chi.URLParam(r, "id")
		fmt.Printf("id %s", id)
		list, err := packingStore.GetPackingList(r.Context(), id, packing.DemoUserID)
		if err != nil {
			log.Printf("render ui: %v", err)
			http.Error(w, "getting the list failed", http.StatusInternalServerError)
			return
		}

		if err := views.PackingDetails("Packing details", list).Render(r.Context(), w); err != nil {
			log.Printf("render details: %v", err)
		}
	})

	r.Get("/new-list", func(w http.ResponseWriter, r *http.Request) {
		form := packing.NewCreatePackingListForm()
		if err := views.NewPackingListPage("New Packing List", form, csrf.Token(r)).Render(r.Context(), w); err != nil {
			log.Printf("render new list: %v", err)
		}
	})

	r.Post("/new-list", func(w http.ResponseWriter, r *http.Request) {
		err := r.ParseForm()
		if err != nil {
			http.Error(w, "invalid form data", http.StatusBadRequest)
			return
		}

		var form packing.CreatePackingListForm

		// decode the form
		dec := schema.NewDecoder()
		dec.IgnoreUnknownKeys(true)
		err = dec.Decode(&form, r.PostForm)
		if err != nil {
			http.Error(w, "invalid form data", http.StatusBadRequest)
			return
		}

		form.Initial = false

		// validate the input
		if errs := form.Validate(); len(errs) > 0 {
			form.Error = errs
			if err := views.NewPackingListPage("New Packing List", form, csrf.Token(r)).Render(r.Context(), w); err != nil {
				log.Printf("render new list: %v", err)
			}
			return
		}

		list := packing.NewList(packing.DemoUserID, form.Name, form.Description)

		err = packingStore.SavePackingList(r.Context(), list)
		if err != nil {
			form.Error = []string{"Storing packing list failed"}
			http.Error(w, "Storing packing list failed", http.StatusInternalServerError)
			return
		}

		http.Redirect(w, r, "/ui", http.StatusSeeOther)
	})

	r.Post("/packing-list", func(w http.ResponseWriter, r *http.Request) {
		var req packing.CreatePackingList
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}

		list := packing.NewList(packing.DemoUserID, req.Name, req.Description)

		err := packingStore.SavePackingList(r.Context(), list)
		if err != nil {
			log.Printf("save packing list: %v", err)
			http.Error(w, "Storing packing list failed", http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusCreated)
		fmt.Fprintln(w, "packing list created")
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
