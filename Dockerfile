FROM golang:1.13.5-alpine as builder

ENV PROJECT_NAME=admission-server
ENV BUILD_PATH=$GOPATH/src/$PROJECT_NAME/


RUN mkdir -p $GOPATH/src/$PROJECT_NAME
WORKDIR $BUILD_PATH

COPY main.go .
COPY go.mod .
COPY go.sum .

RUN go mod download

RUN go build -o /go/bin/$PROJECT_NAME $BUILD_PATH/main.go

# ===========================
FROM alpine:3.9 AS final

ENV PROJECT_NAME=admission-server

COPY --from=builder /go/bin/$PROJECT_NAME /usr/local/bin/$PROJECT_NAME

RUN mkdir -p /etc/webhook/certs.d/

ENTRYPOINT ["/usr/local/bin/admission-server"]