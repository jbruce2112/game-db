.PHONY: server web test tidy

server:
	cd server && go run ./cmd/gamedb

web:
	cd web && npm run dev

test:
	cd server && go test ./...

tidy:
	cd server && go mod tidy
