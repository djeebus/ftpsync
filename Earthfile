VERSION 0.7

build:
    ARG GOLANG_VERSION="1.25.3"
    ARG ALPINE_VERSION="3.22"

    FROM golang:${GOLANG_VERSION}-alpine${ALPINE_VERSION}

    WORKDIR /src
    COPY . /src

    RUN apk add --no-cache gcc musl-dev

    ENV CGO_ENABLED=1
    RUN go build -o ftpsync ./main.go
    SAVE ARTIFACT ./ftpsync AS LOCAL ./dist/ftpsync

tests:
    ARG GOLANG_VERSION="1.25.3"
    ARG ALPINE_VERSION="3.22"

    FROM golang:${GOLANG_VERSION}-alpine${ALPINE_VERSION}

    WORKDIR /src
    COPY . /src

    RUN apk add --no-cache gcc musl-dev

    ENV CGO_ENABLED=1
    RUN go test ./...

image:
    ARG ALPINE_VERSION="3.22"
    ARG image="ftpsync:dev"

    FROM alpine:${ALPINE_VERSION}

    COPY +build/ftpsync /bin
    ENTRYPOINT /bin/ftpsync

    SAVE IMAGE --push ${image}
