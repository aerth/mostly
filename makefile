tests: *.go */*.go */*/*.go
	go test -c -o ./bin/ ./...
clean:
	rm -rf bin
