.PHONY: install update start

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

start:
	./orchestrator/start.sh $(if $(PROJECT),--project "$(PROJECT)") $(if $(FACTORY),--factory "$(FACTORY)")
