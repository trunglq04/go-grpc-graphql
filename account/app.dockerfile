FROM golang:1.26.4 AS build 
RUN apk --no-cache add gcc g++ make ca-certificates
WORKDIR /go/src/github.com/trunglq04/go-grpc-graphql
COPY go.mod go.sum ./
COPY vendor vendor
COPY account account
RUN GO111MODULE=on go build -mod vendor -o /go/bin/app ./account/cmd/account

FROM alpine:3.11
WORKDIR /usr/bin
COPY -- from=build /go/bin .
EXPOSE 8080
CMD ["app"]