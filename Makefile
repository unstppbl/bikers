.PHONY: help config build db migrate up down logs deploy-jarvis gis-network check-web check-architecture check-backend check-integration

DEPLOY_ARGS ?=

help:
	@echo "config              Validate Compose configuration without printing secrets"
	@echo "build               Build the single web/API application image"
	@echo "db                  Start the private PostgreSQL service"
	@echo "migrate             Apply explicit database migrations (starts db)"
	@echo "up                  Start the app and db; run migrate first"
	@echo "down                Stop services; preserve database and media volumes"
	@echo "logs                Follow application logs"
	@echo "gis-network         Create/validate the private shared GIS network for local Compose"
	@echo "deploy-jarvis       Build on Jarvis; DEPLOY_ARGS='--migrate-to 10 --configure-gis' opts into reviewed GIS rollout"
	@echo "check-web           Typecheck, lint, test and build the web app"
	@echo "check-architecture  Check all backend internal package imports"
	@echo "check-backend       Run Go unit tests and vet"
	@echo "check-integration   Run Go tests with required TEST_DATABASE_URL"

config:
	docker compose config --quiet

build:
	docker compose build app

db:
	docker compose up -d db

migrate: db gis-network
	docker compose run --rm app migrate

up: gis-network
	docker compose up -d app

gis-network:
	@if docker network inspect bikerskz_gis >/dev/null 2>&1; then \
		test "$$(docker network inspect --format '{{.Internal}}' bikerskz_gis)" = true || \
		{ echo "bikerskz_gis must be an internal network" >&2; exit 1; }; \
	else \
		docker network create --internal --label com.docker.compose.project=bkgis \
			--label com.docker.compose.network=private bikerskz_gis; \
	fi

down:
	docker compose down

logs:
	docker compose logs --follow app

deploy-jarvis:
	python3 scripts/deploy-jarvis.py $(DEPLOY_ARGS)

check-web:
	cd apps/web && npm run typecheck && npm run lint && npm test && npm run build

check-architecture:
	cd apps/backend && go run ./cmd/architecture

check-backend: check-architecture
	cd apps/backend && go test ./... && go vet ./...

check-integration: check-architecture
	@test -n "$$TEST_DATABASE_URL" || (echo "Set TEST_DATABASE_URL to a disposable PostgreSQL database" >&2; exit 1)
	cd apps/backend && go test -count=1 ./...
