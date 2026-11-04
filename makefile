all: tests cmds
tests: *.go */*.go */*/*.go
	go test -trimpath -ldflags '-w -s' -c -o ./bin/ ./...
cmds: *.go */*.go */*/*.go
	go build -trimpath -ldflags '-w -s' -o ./bin/ ./...

clean:
	rm -rf bin
