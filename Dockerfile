FROM golang:latest as builder
RUN mkdir /app
ADD . /app/
WORKDIR /app
RUN go get -d
RUN  CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -ldflags '-extldflags "-static"' -o main .


# Final image.
FROM alpine

RUN apk update && apk add ca-certificates && rm -rf /var/cache/apk/*
COPY --from=builder /app/main .
EXPOSE 2112
ENTRYPOINT ["./main"]
