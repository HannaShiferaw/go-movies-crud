# go-movies-crud

A small movies API in Go, using gorilla/mux. It stores films, lets you
list, view, add, change and delete them, and keeps them saved in a file
so they are still there after a restart.

## Run it

```sh
go run .
```

The server listens on port 8000 by default.

```sh
PORT=8080 go run .
```

Movies are saved to `movies.json` in the folder you run it from.
Change that with `MOVIES_FILE`:

```sh
MOVIES_FILE=/tmp/movies.json go run .
```

`movies.json` is local data and is not committed to git.

## API

| Method | Path          | What it does              | Success |
| ------ | ------------- | ------------------------- | ------- |
| GET    | `/health`     | Is the server up?         | 200     |
| GET    | `/movies`     | List all movies           | 200     |
| GET    | `/movies/{id}`| One movie                 | 200, or 404 |
| POST   | `/movies`     | Add a movie               | 201     |
| PUT    | `/movies/{id}`| Replace a movie           | 200, or 404 |
| DELETE | `/movies/{id}`| Delete a movie            | 204, or 404 |

A movie looks like this:

```json
{
  "id": "3",
  "isbn": "123456",
  "title": "New Film",
  "year": 2024,
  "director": {"firstName": "Jane", "lastName": "Doe"}
}
```

Rules:

- `title` is required.
- `director`, with `firstName` and `lastName`, is required.
- `year`, when given, must be between 1888 and 2100.
- Bad JSON gets a 400. A missing movie gets a 404 with a JSON error.
- IDs are stable, sequential numbers (`1`, `2`, `3`, …), assigned by
  the server — never random.

Example:

```sh
curl http://localhost:8000/movies
curl -X POST http://localhost:8000/movies \
  -H 'Content-Type: application/json' \
  -d '{"isbn":"123456","title":"New Film","year":2024,"director":{"firstName":"Jane","lastName":"Doe"}}'
curl -X DELETE http://localhost:8000/movies/3
```

The API starts with two seed movies, "Movie One" (id 1) and
"Movie Two" (id 2), when there is no saved file yet.

## Persistence

Movies are saved as JSON in `movies.json`. That is a deliberate
choice for a small project: no database server, no CGO driver and no
extra dependency. For a multi-user service, swap the `Store` in
`store.go` for a database-backed one.

## Tests

```sh
go test ./...
go vet ./...
gofmt -l .
```

## Files

- `main.go` — the HTTP handlers, validation and the server.
- `store.go` — loading, saving and changing movies, with a lock
  so two requests cannot corrupt the file.
- `main_test.go` — tests for listing, viewing, creating, updating
  and deleting movies, plus validation, bad JSON and 404s.
