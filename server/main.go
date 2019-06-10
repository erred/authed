package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"strings"

	firebase "firebase.google.com/go"
	"firebase.google.com/go/auth"
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
	ctx := context.Background()
	svr := NewServer(ctx)
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
		if Debug {
			log.Printf("got %v request for: %v with headers: %v\n", r.Method, r.URL.Path, r.Header)
		}
		if b := r.Header.Get("Authorization"); !strings.HasPrefix(b, "Bearer: ") {
			w.WriteHeader(http.StatusUnauthorized)
			return
		} else if _, err := svr.authClient.VerifyIDToken(context.Background(), strings.TrimPrefix(b, "Bearer: ")); err != nil {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		wsvr.ServeHTTP(w, r)
	}))
}

type Server struct {
	app        *firebase.App
	authClient *auth.Client
}

func NewServer(ctx context.Context) *Server {
	// uses GOOGLE_APPLICATION_CREDENTIALS
	app, err := firebase.NewApp(ctx, nil)
	if err != nil {
		log.Fatalf("NewServer firebase.NewApp: %v\n", err)
	}
	authClient, err := app.Auth(ctx)
	if err != nil {
		log.Fatalf("NewServer app.Auth: %v\n", err)
	}
	return &Server{
		app:        app,
		authClient: authClient,
	}
}

func (s *Server) Echo(ctx context.Context, r *authed.Msg) (*authed.Msg, error) {
	return r, nil
}
