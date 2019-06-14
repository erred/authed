package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"

	firebase "firebase.google.com/go"
	"firebase.google.com/go/auth"
	"github.com/improbable-eng/grpc-web/go/grpcweb"
	"github.com/seankhliao/authed/authed"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
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
	gsvr := grpc.NewServer(grpc.UnaryInterceptor(svr.authInterceptor))
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
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return nil, fmt.Errorf("Echo: no metadata found")
	}
	authHeader, ok := md["authorization"]
	if !ok {
		return nil, fmt.Errorf("Echo: no authorization header found")
	}
	token, err := s.authClient.VerifyIDToken(ctx, authHeader[0])
	if err != nil {
		return nil, fmt.Errorf("Echo: verification: %v", err)
	}
	userRecord, err := s.authClient.GetUser(ctx, token.UID)
	if err != nil {
		return nil, fmt.Errorf("Echo: GetUser: %v", err)
	}
	return &authed.Msg{
		Msg: fmt.Sprintf("hello %v, your email is %v\n", userRecord.DisplayName, userRecord.Email),
	}, nil
}

func (s *Server) authInterceptor(ctx context.Context, r interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
	if err := s.authorize(ctx); err != nil {
		if Debug {
			log.Printf("authInterceptor not authorized: %v\n", err)
		}
		return nil, err
	}

	return handler(ctx, r)
}

func (s *Server) authorize(ctx context.Context) error {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return errors.New("authorize: no metadata found")
	}

	authHeader, ok := md["authorization"]
	if !ok {
		return errors.New("authorize: authorization header not found")
	}

	_, err := s.authClient.VerifyIDToken(context.Background(), authHeader[0])
	if err != nil {
		return fmt.Errorf("authorize: VerifyIDToken: %v\n", err)
	}
	return nil
}
