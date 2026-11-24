package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"sync"
)

// Store holds the movies in memory and persists them to a JSON file
// (movies.json by default) so they survive a restart.
//
// JSON-file persistence is a deliberate choice for this small project:
// it needs no database server, no CGO driver and no extra dependency,
// and it is easy to inspect and back up. For a multi-user service,
// replace Store with a database-backed implementation.
type Store struct {
	mu     sync.Mutex
	path   string
	movies []Movie
	nextID int
}

// NewStore loads movies from path, or seeds the two original movies
// when the file does not exist yet.
func NewStore(path string) (*Store, error) {
	s := &Store{path: path, nextID: 1}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			s.movies = seedMovies()
			s.nextID = 3
			return s, s.saveLocked()
		}
		return nil, fmt.Errorf("read store file: %w", err)
	}
	if len(data) == 0 {
		s.movies = seedMovies()
		s.nextID = 3
		return s, s.saveLocked()
	}

	var movies []Movie
	if err := json.Unmarshal(data, &movies); err != nil {
		return nil, fmt.Errorf("parse store file: %w", err)
	}
	s.movies = movies
	for _, m := range movies {
		if n, err := strconv.Atoi(m.ID); err == nil && n >= s.nextID {
			s.nextID = n + 1
		}
	}
	return s, nil
}

func seedMovies() []Movie {
	return []Movie{
		{ID: "1", Isbn: "438227", Title: "Movie One", Year: 2020, Director: &Director{FirstName: "Abe", LastName: "Kebe"}},
		{ID: "2", Isbn: "454555", Title: "Movie Two", Year: 2021, Director: &Director{FirstName: "Abebe", LastName: "Kebede"}},
	}
}

// saveLocked writes the current movies to disk. Callers must hold s.mu.
func (s *Store) saveLocked() error {
	data, err := json.MarshalIndent(s.movies, "", "  ")
	if err != nil {
		return fmt.Errorf("encode movies: %w", err)
	}
	if err := os.WriteFile(s.path, data, 0o644); err != nil {
		return fmt.Errorf("write store file: %w", err)
	}
	return nil
}

func (s *Store) List() []Movie {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]Movie, len(s.movies))
	copy(out, s.movies)
	return out
}

func (s *Store) Get(id string) (Movie, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, m := range s.movies {
		if m.ID == id {
			return m, true
		}
	}
	return Movie{}, false
}

// Create validates the movie, assigns the next stable sequential ID
// and persists it.
func (s *Store) Create(m Movie) (Movie, error) {
	if err := validateMovie(m); err != nil {
		return Movie{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	m.ID = strconv.Itoa(s.nextID)
	s.nextID++
	s.movies = append(s.movies, m)
	if err := s.saveLocked(); err != nil {
		return Movie{}, err
	}
	return m, nil
}

// Update replaces the movie with the given id, keeping its ID.
// It returns false when no movie with that id exists.
func (s *Store) Update(id string, m Movie) (Movie, bool, error) {
	if err := validateMovie(m); err != nil {
		return Movie{}, false, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for i, item := range s.movies {
		if item.ID == id {
			m.ID = id
			s.movies[i] = m
			if err := s.saveLocked(); err != nil {
				return Movie{}, false, err
			}
			return m, true, nil
		}
	}
	return Movie{}, false, nil
}

// Delete removes the movie with the given id.
// It returns false when no movie with that id exists.
func (s *Store) Delete(id string) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i, item := range s.movies {
		if item.ID == id {
			s.movies = append(s.movies[:i], s.movies[i+1:]...)
			return true, s.saveLocked()
		}
	}
	return false, nil
}
