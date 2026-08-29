.PHONY: server web test test-stress tidy

server:
	cd server && go run ./cmd/gamedb

web:
	cd web && npm run dev

test:
	cd server && go test ./...

# Extreme concurrency / volume suite. Default scale is already heavy.
# STRESS=1 expands to thousands of rows and more devices (~minutes).
test-stress:
	cd server && go test -count=1 -timeout 20m -run 'TestExtremeStress|TestStress' ./internal/api ./internal/store ./internal/barcode

tidy:
	cd server && go mod tidy
