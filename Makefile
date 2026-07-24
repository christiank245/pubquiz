APP_NAME=pubquiz
BIN_DIR=bin

.PHONY: deps css run build test clean

deps:
	go mod tidy
	npm install

css:
	npm run build:css

run: css
	go run . serve

build: css
	mkdir -p $(BIN_DIR)
	CGO_ENABLED=0 go build -o $(BIN_DIR)/$(APP_NAME) .

test:
	go tool ginkgo -r ./...

clean:
	rm -rf $(BIN_DIR)
