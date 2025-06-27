FROM golang:1.14-alpine AS builder
RUN apk --no-cache add gcc g++ make git
WORKDIR /go/src/app
COPY . .

ENV GOOS=linux \
    GOBIN=$GOPATH/bin
RUN go mod download
RUN go build -ldflags="-s -w" -o ./bin/main-bin ./*.go

# creating an alpine image from scratch (lightweight)
FROM alpine:3.9

# adding ffmpeg. used for creation thumbnail from video
# without this, image size : ~12 MB ; with this, image size : ~65 MB
# there are lighter images for ffmpeg, JIC needed : https://hub.docker.com/r/jrottenberg/ffmpeg
RUN apk add --update ffmpeg

# copying binary built from previous stage
WORKDIR /usr/bin
COPY --from=builder /go/src/app/bin /go/bin
EXPOSE 4499
ENTRYPOINT /go/bin/main-bin 