FROM golang:1.14-alpine as builder
RUN apk --no-cache add gcc g++ make git
WORKDIR /go/src/app
COPY . .

ENV GOOS=linux \
    GOARCH=amd64 \
    GOBIN=$GOPATH/bin
RUN go get ./...
RUN go build -ldflags="-s -w" -o ./bin/main-bin ./main.go

FROM alpine:3.9
WORKDIR /usr/bin
COPY --from=builder /go/src/app/bin /go/bin
EXPOSE 2712
ENTRYPOINT /go/bin/main-bin 