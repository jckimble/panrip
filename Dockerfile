FROM golang:latest as gobuild
RUN go get github.com/cellofellow/gopiano
RUN go get github.com/bogem/id3v2
RUN go get github.com/spf13/cobra
RUN go get github.com/spf13/viper
ADD ./main.go /main.go
WORKDIR /
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o panrip ./main.go

FROM alpine:latest
WORKDIR /
RUN apk add --update ffmpeg ca-certificates && rm -rf /var/cache/apk/*
RUN mkdir /download
COPY --from=gobuild /panrip .

VOLUME ["/download"]
ENTRYPOINT ["/panrip"]
