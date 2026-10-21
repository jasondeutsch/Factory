.PHONY: install update start verify

install:
	brew install herdr git go node
	curl -fsSL https://pi.dev/install.sh | sh
	npm ci --omit=dev
	cd orchestrator/comms && go mod download

update:
	brew upgrade herdr git go node
	pi update self
	npm ci --omit=dev
	cd orchestrator/comms && go mod download

verify:
	npm ci
	npm run check
	npm test
	cd orchestrator/comms && go test ./...

start:
	./orchestrator/start.sh $(if $(PROJECT),--project "$(PROJECT)") $(if $(FACTORY),--factory "$(FACTORY)")

.PHONY: monitor
monitor:
	./orchestrator/monitor.sh $(if $(PROJECT),-project "$(PROJECT)")
