.PHONY: backend frontend tester openapi openapi-webconsole run run-tester test tidy lint clean docker

BACKEND_SRC := $(shell find web/backend -name "*.go")
TESTER_SRC := $(shell find tester -name "*.go") tester/go.mod tester/go.sum
FRONTEND_SRC := $(shell find web/frontend -type f ! -path "web/frontend/dist/*" ! -path "web/frontend/node_modules/*")
FRONTEND_STAMP := build/frontend/.stamp

all: backend frontend tester

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

build/fru-tester: $(TESTER_SRC)
	@echo "[+] Building fru-tester..."
	mkdir -p build
	cd tester && go build -o ../build/fru-tester .
	@echo "[✔] fru-tester build finished"

tester: build/fru-tester

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

openapi-webconsole:
	@echo "[+] Generating webconsole OpenAPI client..."
	cd web && ./webconsole-openapi-generator-docker.sh
	@echo "[✔] webconsole OpenAPI client generated"

run:
	./build/fru-lab -c config.yaml

# needs root for netlink (gNB IPs) and the sctp kernel module
run-tester:
	sudo modprobe sctp
	sudo ./build/fru-tester -c tester.yaml

test:
	cd tester && go test -race ./...
	cd web/backend && go test ./...

tidy:
	cd web/backend && go mod tidy
	cd tester && go mod tidy

lint:
	cd web/backend && golangci-lint run
	cd tester && golangci-lint run

clean:
	rm -rf build
	rm /tmp/frulab.db

docker:
	./docker/build_image.sh