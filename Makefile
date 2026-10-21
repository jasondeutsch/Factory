.PHONY: install update start verify

install:
	brew install herdr git go node
	curl -fsSL https://pi.dev/install.sh | sh
	cd orchestrator && npm ci --omit=dev
	cd comms && go mod download

update:
	brew upgrade herdr git go node
	pi update self
	cd orchestrator && npm ci --omit=dev
	cd comms && go mod download

verify:
	cd orchestrator && npm ci
	cd orchestrator && npm run check
	cd orchestrator && npm test
	cd comms && go test ./...

start:
	./orchestrator/start.sh $(if $(PROJECT),--project "$(PROJECT)") $(if $(FACTORY),--factory "$(FACTORY)")

.PHONY: monitor
monitor:
	./orchestrator/monitor.sh $(if $(PROJECT),-project "$(PROJECT)")
