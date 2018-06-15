FROM golang:latest

RUN go get golang.org/x/net/http2

COPY main.go /go/src/github.com/ernado/go-issue-25009/

RUN go install github.com/ernado/go-issue-25009

ENTRYPOINT ["/go/bin/go-issue-25009"]