.PHONY: backend frontend openapi run  tidy lint clean

BACKEND_SRC := $(shell find web/backend -name "*.go")
FRONTEND_SRC := $(shell find web/frontend -type f ! -path "web/frontend/dist/*" ! -path "web/frontend/node_modules/*")
FRONTEND_STAMP := build/frontend/.stamp

all: backend frontend

build/fru-lab: $(BACKEND_SRC)
	@echo "[+] Building backend..."
	mkdir -p build
	cd web/backend && go build -o ../../build/fru-lab .
	@echo "[✔] Backend build finished"

build/frontend: $(FRONTEND_SRC)
	@echo "[+] Installing frontend deps..."
	cd web/frontend && yarn install

	@echo "[+] Building frontend..."
	cd web/frontend && yarn build

	@echo "[✔] Frontend build finished"
	@mkdir -p build/frontend
	@cp -r web/frontend/dist/. build/frontend/
	@touch $(FRONTEND_STAMP)

backend:
	@if [ -f build/fru-lab ]; then \
		if [ -z "$$(find web/backend -name '*.go' -newer build/fru-lab)" ]; then \
			echo "[✔] backend is up-to-date, no build needed"; \
			exit 0; \
		fi; \
	fi; \
	$(MAKE) build/fru-lab

frontend:
	@if [ -f $(FRONTEND_STAMP) ]; then \
		if [ -z "$$(find web/frontend -type f -newer $(FRONTEND_STAMP))" ]; then \
			echo "[✔] frontend is up-to-date, no build needed"; \
			exit 0; \
		fi; \
	fi; \
	$(MAKE) build/frontend

openapi:
	@echo "[+] Generating OpenAPI client..."
	cd web && ./openapi-generator-docker.sh
	@echo "[✔] OpenAPI client generated"

run:
	./build/fru-lab -c config.yaml

tidy:
	cd web/backend && go mod tidy

lint:
	cd web/backend && golangci-lint run

clean:
	rm -rf build
	rm /tmp/frulab.db