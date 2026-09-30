tests: *.go */*.go */*/*.go
	go test -trimpath -ldflags '-w -s' -c -o ./bin/ ./...
clean:
	rm -rf bin
