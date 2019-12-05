test:
	go test -race .
cover:
	go test . -coverprofile=cover.out
	go tool cover -html=cover.out
bench:
	go test -bench=.
