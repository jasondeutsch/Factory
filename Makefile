.PHONY: install update start

install:
	brew install herdr git go python golangci-lint
	curl -fsSL https://pi.dev/install.sh | sh
	go install golang.org/x/tools/gopls@latest
	PI_CODING_AGENT_DIR="$(CURDIR)/factories/golang/agents/coder_golang/pi" pi update --extensions --no-approve
	cd orchestrator/comms && go mod download

update:
	brew upgrade herdr git go python golangci-lint
	pi update self
	go install golang.org/x/tools/gopls@latest
	PI_CODING_AGENT_DIR="$(CURDIR)/factories/golang/agents/coder_golang/pi" pi update --extensions --no-approve
	cd orchestrator/comms && go mod download

start:
	docker-compose -f orchestrator/comms/compose.yaml up --build -d
	./orchestrator/start.sh
