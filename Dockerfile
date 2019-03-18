FROM golang:alpine AS builder
RUN apk update && apk add --no-cache git ca-certificates
RUN apk add --no-cache --virtual .build-deps bash gcc musl-dev openssl
COPY ./src/main.go $GOPATH/src/main.go
WORKDIR $GOPATH/src/
RUN go get -d -v
RUN CGO_ENABLED=1 GOOS=linux GOARCH=amd64 go build -a -installsuffix cgo -ldflags="-w -s" -o /nodehealthcheck

FROM alpine
RUN apk update && apk add ca-certificates
COPY --from=builder /nodehealthcheck /
ENTRYPOINT ["/nodehealthcheck"]