.PHONY: install update

install:
	$(MAKE) -C ../.. install
	brew install golangci-lint
	go install golang.org/x/tools/gopls@latest
	python3 ../../orchestrator/scripts/install_extensions.py golang coder_golang

update:
	$(MAKE) -C ../.. update
	brew upgrade golangci-lint
	go install golang.org/x/tools/gopls@latest
	python3 ../../orchestrator/scripts/install_extensions.py golang coder_golang
