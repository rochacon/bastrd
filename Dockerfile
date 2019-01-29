FROM golang:1.11-alpine as builder
ENV GO111MODULE=on
RUN apk add --no-cache ca-certificates git
RUN grep nobody /etc/passwd > /etc/passwd.nobody \
 && grep nobody /etc/group > /etc/group.nobody
COPY go.mod /go/src/github.com/rochacon/bastrd/go.mod
COPY go.sum /go/src/github.com/rochacon/bastrd/go.sum
WORKDIR /go/src/github.com/rochacon/bastrd
RUN go mod download
COPY . /go/src/github.com/rochacon/bastrd
RUN go install -v -ldflags "-X main.VERSION=$(git describe --abbrev=10 --always --dirty --tags)" -tags "netgo osusergo"

FROM scratch
COPY --from=builder /etc/passwd.nobody /etc/passwd
COPY --from=builder /etc/group.nobody /etc/group
COPY --from=builder /etc/ssl/certs /etc/ssl/certs
COPY --from=builder /go/bin/bastrd /bastrd
USER nobody
ENTRYPOINT ["/bastrd"]
