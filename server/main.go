package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/improbable-eng/grpc-web/go/grpcweb"
	"github.com/seankhliao/authed/authed"
	"google.golang.org/grpc"
)

var (
	// grpc stuff
	Debug   = false
	Headers = strings.Split(os.Getenv("HEADERS"), ",")
	Origins = make(map[string]struct{})
	Port    = os.Getenv("PORT")
)

func init() {
	// grpc stuff
	if os.Getenv("DEBUG") == "1" {
		Debug = true
	}

	for i, h := range Headers {
		Headers[i] = strings.TrimSpace(h)
	}

	for _, o := range strings.Split(os.Getenv("ORIGINS"), ",") {
		Origins[strings.TrimSpace(o)] = struct{}{}
	}

	if Port == "" {
		Port = ":8090"
	}
	if Port[0] != ':' {
		Port = ":" + Port
	}
}

func allowOrigin(o string) bool {
	_, ok := Origins[o]
	if Debug {
		log.Printf("origin filter %v allowed: %v\n", o, ok)
	}
	return ok
}

func main() {
	svr := NewServer()
	gsvr := grpc.NewServer()
	authed.RegisterAuthedServer(gsvr, svr)
	wsvr := grpcweb.WrapServer(gsvr,
		grpcweb.WithOriginFunc(allowOrigin),
		grpcweb.WithAllowedRequestHeaders(Headers),
	)

	if Debug {
		log.Printf("starting on %v\nallowing headers: %v\nallowing origins: %v\n",
			Port, Headers, Origins)
	}
	http.ListenAndServe(Port, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		wsvr.ServeHTTP(w, r)
	}))
}

type Server struct{}

func NewServer() *Server {
	return &Server{}
}

func (s *Server) Echo(ctx context.Context, r *authed.Msg) (*authed.Msg, error) {
	return r, nil
}
