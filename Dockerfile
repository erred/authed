FROM golang:alpine AS build

WORKDIR /app
COPY . .
RUN GO111MODULE=on CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
        go build -mod=vendor -o ggwt-server github.com/seankhliao/go-grpc-web-tmpl/server

FROM scratch

COPY --from=build /app/ggwt-server /bin/

ENTRYPOINT ["/bin/ggwt-server"]
