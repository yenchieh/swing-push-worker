FROM golang:1.7

ADD . /go/src/github.com/swing-push-worker


ENTRYPOINT["/swing-push-worker"]


