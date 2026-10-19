.PHONY: install update

install:
	$(MAKE) -C ../.. install
	brew install golangci-lint
	go install golang.org/x/tools/gopls@latest
	PI_CODING_AGENT_DIR="$(CURDIR)/agents/coder_golang/pi" pi update --extensions --no-approve

update:
	$(MAKE) -C ../.. update
	brew upgrade golangci-lint
	go install golang.org/x/tools/gopls@latest
	PI_CODING_AGENT_DIR="$(CURDIR)/agents/coder_golang/pi" pi update --extensions --no-approve
