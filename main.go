package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/gorilla/mux"
)

// Movie is a single film. Year is optional in the request, but when
// given it must be a sensible year (see validateMovie).
type Movie struct {
	ID       string    `json:"id"`
	Isbn     string    `json:"isbn"`
	Title    string    `json:"title"`
	Year     int       `json:"year,omitempty"`
	Director *Director `json:"director"`
}

// Director is the person who directed the film.
type Director struct {
	FirstName string `json:"firstName"`
	LastName  string `json:"lastName"`
}

type errorResponse struct {
	Error string `json:"error"`
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, errorResponse{Error: msg})
}

// validateMovie checks the fields a client must supply.
// Title and director (first and last name) are required.
// Year, when given, must be between 1888 (the first films) and 2100.
func validateMovie(m Movie) error {
	if m.Title == "" {
		return errors.New("title is required")
	}
	if m.Director == nil {
		return errors.New("director is required")
	}
	if m.Director.FirstName == "" {
		return errors.New("director firstName is required")
	}
	if m.Director.LastName == "" {
		return errors.New("director lastName is required")
	}
	if m.Year != 0 && (m.Year < 1888 || m.Year > 2100) {
		return errors.New("year must be between 1888 and 2100")
	}
	return nil
}

// decodeMovie reads a movie from the request body. Bad JSON is a
// 400 error, not a silent empty movie.
func decodeMovie(w http.ResponseWriter, r *http.Request) (Movie, bool) {
	var movie Movie
	if err := json.NewDecoder(r.Body).Decode(&movie); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return Movie{}, false
	}
	return movie, true
}

type server struct {
	store *Store
}

func (s *server) getMovies(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, s.store.List())
}

func (s *server) getMovie(w http.ResponseWriter, r *http.Request) {
	movie, ok := s.store.Get(mux.Vars(r)["id"])
	if !ok {
		writeError(w, http.StatusNotFound, "movie not found")
		return
	}
	writeJSON(w, http.StatusOK, movie)
}

func (s *server) createMovie(w http.ResponseWriter, r *http.Request) {
	movie, ok := decodeMovie(w, r)
	if !ok {
		return
	}
	created, err := s.store.Create(movie)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, created)
}

func (s *server) updateMovie(w http.ResponseWriter, r *http.Request) {
	movie, ok := decodeMovie(w, r)
	if !ok {
		return
	}
	updated, found, err := s.store.Update(mux.Vars(r)["id"], movie)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if !found {
		writeError(w, http.StatusNotFound, "movie not found")
		return
	}
	writeJSON(w, http.StatusOK, updated)
}

func (s *server) deleteMovie(w http.ResponseWriter, r *http.Request) {
	deleted, err := s.store.Delete(mux.Vars(r)["id"])
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not save movies")
		return
	}
	if !deleted {
		writeError(w, http.StatusNotFound, "movie not found")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *server) health(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func newRouter(s *server) *mux.Router {
	r := mux.NewRouter()
	r.HandleFunc("/health", s.health).Methods("GET")
	r.HandleFunc("/movies", s.getMovies).Methods("GET")
	r.HandleFunc("/movies/{id}", s.getMovie).Methods("GET")
	r.HandleFunc("/movies", s.createMovie).Methods("POST")
	r.HandleFunc("/movies/{id}", s.updateMovie).Methods("PUT")
	r.HandleFunc("/movies/{id}", s.deleteMovie).Methods("DELETE")
	return r
}

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8000"
	}
	file := os.Getenv("MOVIES_FILE")
	if file == "" {
		file = "movies.json"
	}

	store, err := NewStore(file)
	if err != nil {
		log.Fatalf("could not load movies: %v", err)
	}

	srv := &http.Server{
		Addr:         ":" + port,
		Handler:      newRouter(&server{store: store}),
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}
	fmt.Printf("Starting server at port %s\n", port)
	log.Fatal(srv.ListenAndServe())
}
