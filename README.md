# api-changelog

Compares the Swagger 2.0 `swagger.json` at a head ref (by default the working tree) with the same file at its merge base with a base ref, and writes a markdown changelog of the endpoint changes. Run it after `swag init` (or any generator that writes `swagger.json`).

It reports per endpoint (method and path): added or removed endpoints, deprecation, `consumes`/`produces`, parameters (type, `in`, required, enum, validation keywords), request body fields and response fields (type, required, enum, validation keywords, nested fields, `$ref`s inlined). A pure definition rename produces no change. With no changes the changelog file is deleted.

## Install

```
go get -tool github.com/amomama/api-changelog/cmd/api-changelog
go tool api-changelog
```

Requires Go 1.24 for the `tool` directive.

## Flags

| Flag | Default | Meaning |
| --- | --- | --- |
| `-spec` | `docs/swagger.json` | swagger.json to compare |
| `-base` | `origin/HEAD` | git ref the head is compared with |
| `-head` | `HEAD` | git ref compared with the base; `HEAD` reads the spec from the working tree, so uncommitted `swag init` output counts, any other ref reads it from that commit |
| `-out` | `<dir of spec>/<name>.md` | changelog file path |
| `-name` | head branch | name used in the header and the default file name (`/` becomes `-`); required when the head is not a branch, e.g. a commit or a detached HEAD in CI |

## Makefile example

Regenerating the spec and writing the changelog to a `changelog` directory next to it:

```make
BRANCH = $(shell git rev-parse --abbrev-ref HEAD | tr / -)

swag:
	swag init -g cmd/server/main.go --output docs/server
	go tool api-changelog -spec docs/server/swagger.json -base origin/main -out docs/server/changelog/$(BRANCH).md
```

## Example

Branch `feature/book-isbn` adds `POST /books`, a `status` filter and limit on `GET /books`, an `isbn` field and an `archived` status on `Book`, raises the `title` limit, and drops `POST /books/legacy-import`. `go tool api-changelog -base main` writes `docs/feature-book-isbn.md`:

````markdown
# API changes: feature/book-isbn

Compared with `main` at merge base `485494e`.

### GET /books · changed

- Parameter `limit` maximum `100` added
- Parameter `status` added (query, string, one of draft, published, archived): Filter by status
- Response `200`: field `[].isbn` added (string): ISBN-13
- Response `200`: field `[].status` value `archived` added
- Response `200`: field `[].title` maxLength changed from `200` to `500`

### POST /books · added

**Create book**

Consumes `application/json` · Produces `application/json`

**Request body** `CreateBookRequest` (required)

```json
{
  "title": "string",
  "isbn": "string"
}
```

**Response** `IDResponse`

- `200` OK

```json
{
  "id": 0
}
```

**Response** `Error`

- `422` Validation error

```json
{
  "message": "string"
}
```

### POST /books/legacy-import · removed

**Import books from the legacy catalog**

### GET /books/{id} · changed

- Response `200`: field `isbn` added (string): ISBN-13
- Response `200`: field `status` value `archived` added
- Response `200`: field `title` maxLength changed from `200` to `500`
````
