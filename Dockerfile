FROM golang:alpine AS build

WORKDIR /app
RUN apk add --no-cache ca-certificates
COPY . .
RUN GO111MODULE=on CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
        go build -mod=vendor -o authed-server github.com/seankhliao/authed/server

FROM scratch

COPY --from=build /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY --from=build /app/authed-server /bin/

ENTRYPOINT ["/bin/authed-server"]
