package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/gorilla/mux"
)

func newTestServer(t *testing.T) (*server, string) {
	t.Helper()
	dir := t.TempDir()
	store, err := NewStore(filepath.Join(dir, "movies.json"))
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	return &server{store: store}, dir
}

func doRequest(t *testing.T, s *server, method, path string, body []byte) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, bytes.NewReader(body))
	rec := httptest.NewRecorder()
	newRouter(s).ServeHTTP(rec, req)
	return rec
}

func decodeMovies(t *testing.T, rec *httptest.ResponseRecorder) []Movie {
	t.Helper()
	var movies []Movie
	if err := json.NewDecoder(rec.Body).Decode(&movies); err != nil {
		t.Fatalf("decode movies: %v", err)
	}
	return movies
}

func TestListMoviesReturnsSeedMovies(t *testing.T) {
	s, _ := newTestServer(t)
	rec := doRequest(t, s, "GET", "/movies", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if got := len(decodeMovies(t, rec)); got != 2 {
		t.Fatalf("expected 2 seed movies, got %d", got)
	}
}

func TestGetMovie(t *testing.T) {
	s, _ := newTestServer(t)

	rec := doRequest(t, s, "GET", "/movies/1", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	var movie Movie
	if err := json.NewDecoder(rec.Body).Decode(&movie); err != nil {
		t.Fatalf("decode movie: %v", err)
	}
	if movie.Title != "Movie One" {
		t.Fatalf("expected Movie One, got %q", movie.Title)
	}

	rec = doRequest(t, s, "GET", "/movies/999", nil)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for missing movie, got %d", rec.Code)
	}
}

func TestCreateMovie(t *testing.T) {
	s, dir := newTestServer(t)
	body := []byte(`{"isbn":"123","title":"New Film","year":2024,"director":{"firstName":"Jane","lastName":"Doe"}}`)
	rec := doRequest(t, s, "POST", "/movies", body)
	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rec.Code, rec.Body.String())
	}
	var created Movie
	if err := json.NewDecoder(rec.Body).Decode(&created); err != nil {
		t.Fatalf("decode created movie: %v", err)
	}
	if created.ID == "" || created.ID == "1" || created.ID == "2" {
		t.Fatalf("expected a new stable ID, got %q", created.ID)
	}

	// It survives a reload from the same file: persistence works.
	store, err := NewStore(filepath.Join(dir, "movies.json"))
	if err != nil {
		t.Fatalf("reload store: %v", err)
	}
	if _, ok := store.Get(created.ID); !ok {
		t.Fatalf("created movie %s was not persisted", created.ID)
	}
}

func TestCreateMovieValidation(t *testing.T) {
	s, _ := newTestServer(t)

	cases := []struct {
		name string
		body string
	}{
		{"missing title", `{"isbn":"1","director":{"firstName":"Jane","lastName":"Doe"}}`},
		{"missing director", `{"isbn":"1","title":"No Director"}`},
		{"missing director last name", `{"isbn":"1","title":"X","director":{"firstName":"Jane"}}`},
		{"year too early", `{"isbn":"1","title":"X","year":1500,"director":{"firstName":"Jane","lastName":"Doe"}}`},
		{"year too late", `{"isbn":"1","title":"X","year":3000,"director":{"firstName":"Jane","lastName":"Doe"}}`},
	}
	for _, tc := range cases {
		rec := doRequest(t, s, "POST", "/movies", []byte(tc.body))
		if rec.Code != http.StatusBadRequest {
			t.Errorf("%s: expected 400, got %d", tc.name, rec.Code)
		}
	}

	// Bad JSON is a 400, not a silent empty movie.
	rec := doRequest(t, s, "POST", "/movies", []byte(`{not json`))
	if rec.Code != http.StatusBadRequest {
		t.Errorf("bad JSON: expected 400, got %d", rec.Code)
	}
}

func TestUpdateMovie(t *testing.T) {
	s, _ := newTestServer(t)
	body := []byte(`{"isbn":"999","title":"Movie One Updated","year":2022,"director":{"firstName":"Abe","lastName":"Kebe"}}`)
	rec := doRequest(t, s, "PUT", "/movies/1", body)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	var updated Movie
	if err := json.NewDecoder(rec.Body).Decode(&updated); err != nil {
		t.Fatalf("decode updated movie: %v", err)
	}
	if updated.ID != "1" || updated.Title != "Movie One Updated" {
		t.Fatalf("unexpected updated movie: %+v", updated)
	}

	rec = doRequest(t, s, "PUT", "/movies/999", body)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for missing movie, got %d", rec.Code)
	}
}

func TestDeleteMovie(t *testing.T) {
	s, _ := newTestServer(t)
	rec := doRequest(t, s, "DELETE", "/movies/1", nil)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", rec.Code)
	}
	rec = doRequest(t, s, "GET", "/movies/1", nil)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404 after delete, got %d", rec.Code)
	}
	rec = doRequest(t, s, "DELETE", "/movies/999", nil)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for missing movie, got %d", rec.Code)
	}
}

func TestHealth(t *testing.T) {
	s, _ := newTestServer(t)
	rec := doRequest(t, s, "GET", "/health", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
}

func TestRouterUsesMuxVars(t *testing.T) {
	// Guard against the old behaviour where a missing movie returned
	// an empty 200: the router must reach the handler with the id.
	s, _ := newTestServer(t)
	req := httptest.NewRequest("GET", "/movies/abc", nil)
	rec := httptest.NewRecorder()
	r := mux.NewRouter()
	r.HandleFunc("/movies/{id}", s.getMovie).Methods("GET")
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
}
