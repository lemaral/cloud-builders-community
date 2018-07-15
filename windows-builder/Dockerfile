FROM golang:1.10 AS build-env

ADD src/main.go /go/src
ADD src/builder /go/src/builder

ENV GOBIN=/go/bin
RUN go get -d builder
RUN go install /go/src/main.go

FROM gcr.io/distroless/base
COPY --from=build-env /go/bin/main /usr/bin/main
ENTRYPOINT ["/usr/bin/main"]