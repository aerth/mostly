all: tests cmds
tests: *.go */*.go */*/*.go
	go test -c -o ./bin/ -trimpath -ldflags '-w -s' ./...
cmds: *.go */*.go */*/*.go
	go build -o ./bin/ -trimpath -ldflags '-w -s' ./...
clean:
	rm -rf bin
