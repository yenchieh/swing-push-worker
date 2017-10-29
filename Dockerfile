FROM golang:1.9
RUN mkdir -p /go/src/github.com/kidsdynamic/swing-push-worker/cert
ADD ./build /go/src/github.com/kidsdynamic/swing-push-worker/
ADD ./cert /go/src/github.com/kidsdynamic/swing-push-worker/cert
WORKDIR /go/src/github.com/kidsdynamic/swing-push-worker/
CMD ["/go/src/github.com/kidsdynamic/swing-push-worker/main"]
