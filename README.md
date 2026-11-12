# api-changelog

Compares the Swagger 2.0 `swagger.json` in the working tree with the same file at the merge base with a base ref, and writes a markdown changelog of the endpoint changes. Run it after `swag init` (or any generator that writes `swagger.json`).

It reports per endpoint (method and path): added or removed endpoints, deprecation, `consumes`/`produces`, parameters (type, `in`, required, enum), request body fields and response fields (type, required, enum, nested fields, `$ref`s inlined). A pure definition rename produces no change. With no changes the changelog file is deleted.

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
| `-base` | `origin/HEAD` | git ref the branch is compared with |
| `-out` | `<dir of spec>/changelog` | directory the changelog file is written to |
| `-name` | current branch | name used in the header and the file name (`/` becomes `-`); required on a detached HEAD, e.g. in CI |

The file is written to `<out>/<name>.md`.

## Makefile example

A repo with two swag specs:

```make
changelog:
	swag init -g cmd/api/main.go --output docs/api
	swag init -g cmd/partner_api/main.go --output docs/partner_api
	go tool api-changelog -spec docs/api/swagger.json -base origin/stage
	go tool api-changelog -spec docs/partner_api/swagger.json -base origin/stage
```
