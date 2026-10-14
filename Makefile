.PHONY: generate test test-race dbcheck dev-social
generate:
	go run github.com/sqlc-dev/sqlc/cmd/sqlc@v1.30.0 generate
test:
	go test ./...
test-race:
	go test -race ./server ./api/websocketmsg ./service/social ./service/direct ./service/group ./service/invitation
dbcheck:
	go run ./cmd/dbcheck
dev-social:
	go run ./cmd/dev-social

# Explicit DATABASE_URL only; defaults to read-only version check.
migration-status:
	go run ./cmd/migrate
social-check:
	go run ./cmd/social-check
