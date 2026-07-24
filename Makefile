APP_NAME=pubquiz
BIN_DIR=bin
TAILWIND_BIN:=$(shell mise which tailwindcss 2>/dev/null)

.PHONY: deps css run build test clean

deps:
	mise install
	go mod tidy

css:
	test -n "$(TAILWIND_BIN)" || (echo "tailwindcss not found. Run 'make deps' first." && exit 1)
	$(TAILWIND_BIN) -c ./tailwind.config.js -i ./web/assets/css/input.css -o ./web/public/css/styles.css --minify

run: css
	go run . serve

build: css
	mkdir -p $(BIN_DIR)
	CGO_ENABLED=0 go build -o $(BIN_DIR)/$(APP_NAME) .

test:
	go tool ginkgo -r ./...

clean:
	rm -rf $(BIN_DIR)
