# Database test dibuat di container db (docker compose up -d db).
TEST_DATABASE_URL ?= postgres://compass:compass@localhost:5433/compass_test?sslmode=disable

.PHONY: test test-db vet

test-db:
	@docker compose -f ../docker-compose.yml exec -T db sh -c \
		'psql -U $$POSTGRES_USER -d $$POSTGRES_DB -tAc "SELECT 1 FROM pg_database WHERE datname = '"'"'compass_test'"'"'" | grep -q 1 || createdb -U $$POSTGRES_USER compass_test'

test: test-db
	TEST_DATABASE_URL='$(TEST_DATABASE_URL)' go test -count=1 ./...

vet:
	go vet ./...
