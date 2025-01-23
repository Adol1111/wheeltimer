test:
	go test --tags=assert ./...

lint: 
	golangci-lint run -v