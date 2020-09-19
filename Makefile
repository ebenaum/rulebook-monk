
rulebook: build/rulebook-monk
ifndef SRC
	$(error Please provide rulebook source path: `make $@ SRC=XXX`)
endif
	 cat $(SRC) | ./build/rulebook-monk

build/rulebook-monk: builder.go lex.go roman.go cmd/rulebook/main.go
	go build -o $@ ./cmd/rulebook
