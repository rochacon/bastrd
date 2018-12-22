FROM golang:1.11-alpine as build
ENV GO111MODULE=on
RUN apk add --no-cache ca-certificates git
COPY go.mod /go/src/github.com/rochacon/bastrd/go.mod
COPY go.sum /go/src/github.com/rochacon/bastrd/go.sum
WORKDIR /go/src/github.com/rochacon/bastrd
RUN go mod download
COPY . /go/src/github.com/rochacon/bastrd
RUN go install -v -ldflags "-X main.VERSION=$(git describe --abbrev=10 --always --dirty --tags)" -tags "netgo osusergo"

FROM alpine
COPY --from=build /etc/ssl/certs /etc/ssl/certs
COPY --from=build /go/bin/bastrd /bastrd
ENTRYPOINT ["/bastrd"]
