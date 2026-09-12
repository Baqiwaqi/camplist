package web

import (
	"github.com/gorilla/csrf"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"testing"
)

func TestIdentityTokenCanBeUsedAtSignoutPath(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/identity", func(w http.ResponseWriter, r *http.Request) { io.WriteString(w, csrf.Token(r)) })
	mux.HandleFunc("POST /auth/signout", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(204) })
	server := httptest.NewUnstartedServer(nil)
	host := server.Listener.Addr().String()
	server.Config.Handler = CSRFProtection([]byte("01234567890123456789012345678901"), false, host)(mux)
	server.Start()
	defer server.Close()
	jar, _ := cookiejar.New(nil)
	legacyURL, _ := url.Parse(server.URL)
	jar.SetCookies(legacyURL, []*http.Cookie{{Name: "_gorilla_csrf", Value: "legacy-scoped-cookie", Path: "/auth"}})
	client := &http.Client{Jar: jar}
	response, err := client.Get(server.URL + "/api/identity")
	if err != nil {
		t.Fatal(err)
	}
	token, err := io.ReadAll(response.Body)
	response.Body.Close()
	if err != nil {
		t.Fatal(err)
	}
	request, _ := http.NewRequest("POST", server.URL+"/auth/signout", nil)
	request.Header.Set("X-CSRF-Token", string(token))
	request.Header.Set("Origin", server.URL)
	response, err = client.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != 204 {
		t.Fatalf("identity token rejected on signout path: %d", response.StatusCode)
	}
	target, _ := url.Parse(server.URL + "/auth/signout")
	found := false
	for _, cookie := range jar.Cookies(target) {
		if cookie.Name == "camplist-csrf" {
			found = true
		}
	}
	if !found {
		t.Fatal("CSRF cookie missing at signout path")
	}
}
