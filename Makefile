.PHONY: run
run: templ
	go run main.go

.PHONY: db-local
db-local:
	docker compose -f docker-compose.dev.yaml up -d

.PHONY: templ
templ: internal/ui/*.templ
	templ generate

