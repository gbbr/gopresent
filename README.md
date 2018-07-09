To run:

```bash
go get github.com/gbbr/gopresent/...
cd $GOPATH/src/github.com/gbbr/gopresent
go run ./cmd/gopresent/main.go
```

If developing, use the following command to live-reload the server when editing templates or changing static files:

```bash
rego -extra-watches=static/*,templates/* github.com/gbbr/gopresent/cmd/gopresent
```

If `rego` is not installed, install it by running `go get github.com/sqs/rego/...`
