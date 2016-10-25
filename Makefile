PACKAGES := $(shell glide novendor)

export GO15VENDOREXPERIMENT=1

.DEFAULT_GOAL:=build


.PHONY: build
build:
	go build -i $(PACKAGES)
	go build -i .

.PHONY: install
install:
	glide --version || go get github.com/Masterminds/glide
	glide install

.PHONY: test
test:
	go test -cover -race $(PACKAGES)

.PHONY: cover
cover:
	go test . -coverprofile=cover.out
	go tool cover -html=cover.out

.PHONY: install_ci
install_ci: install
		go get github.com/wadey/gocovmerge
		go get github.com/mattn/goveralls
		go get golang.org/x/tools/cmd/cover

.PHONY: test_ci
test_ci: install_ci build
	./scripts/cover.sh $(shell go list $(PACKAGES))
