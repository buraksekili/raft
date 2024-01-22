server:
	go build -o bin/server cmd/server/main.go && ./bin/server --replica=3
client:
	go build -o bin/client cmd/client/main.go && ./bin/client
