.PHONY: install update start

install:
	brew install herdr git go python
	curl -fsSL https://pi.dev/install.sh | sh
	cd orchestrator/comms && go mod download

update:
	brew upgrade herdr git go python
	pi update self
	cd orchestrator/comms && go mod download

start:
	./orchestrator/start.sh --factory $(FACTORY)
