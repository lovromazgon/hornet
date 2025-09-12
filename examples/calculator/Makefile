.PHONY: generate-proto
generate-proto:
	cd sdk/proto && buf generate

.PHONY: build-plugin
build-plugin:
	GOOS=wasip1 GOARCH=wasm go build -buildmode=c-shared -o plugin/main.wasm plugin/main.go

.PHONY: run
run: build-plugin
	cd host && go run main.go

.PHONY: clean
clean:
	rm -f plugin/main.wasm