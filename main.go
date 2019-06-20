package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"

	firebase "firebase.google.com/go"
	"firebase.google.com/go/auth"
	"github.com/improbable-eng/grpc-web/go/grpcweb"
	"github.com/seankhliao/authed/authed"
	log "github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

var (
	Version = "set with -ldflags \"-X main.Verions=$VERSION\""

	Headers = []string{"*"}
	Origins = make(map[string]struct{})
	Port    = os.Getenv("PORT")
)

func init() {
	switch strings.ToLower(os.Getenv("LOG_LEVEL")) {
	case "debug":
		log.SetLevel(log.DebugLevel)
	case "info":
		log.SetLevel(log.InfoLevel)
	case "error":
		fallthrough
	default:
		log.SetLevel(log.ErrorLevel)
	}

	switch strings.ToLower(os.Getenv("LOG_FORMAT")) {
	case "json":
		log.SetFormatter(&log.JSONFormatter{})
	default:
		log.SetFormatter(&log.TextFormatter{})
	}

	for _, o := range strings.Split(os.Getenv("ORIGINS"), ",") {
		Origins[strings.TrimSpace(o)] = struct{}{}
	}

	if Port == "" {
		Port = ":8080"
	}
}

func allowOrigin(o string) bool {
	_, ok := Origins[o]
	log.Infoln("allowOrigin: ", o, ok)
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

	log.Infoln("Starting on", Port)
	http.ListenAndServe(Port, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		wsvr.ServeHTTP(w, r)
	}))
}

type Server struct {
	app        *firebase.App
	authClient *auth.Client
}

func NewServer(ctx context.Context) *Server {
	log.Infoln("NewServer firebase NewApp")
	app, err := firebase.NewApp(ctx, nil)
	if err != nil {
		log.Fatalln("NewServer firebase NewApp", err)
	}

	log.Infoln("NewServer firebase authClient")
	authClient, err := app.Auth(ctx)
	if err != nil {
		log.Fatalln("NewServer firebase authClient", err)
	}

	return &Server{
		app:        app,
		authClient: authClient,
	}
}

func (s *Server) Echo(ctx context.Context, r *authed.Msg) (*authed.Msg, error) {
	log.Infoln("Echo get metadata")
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		log.Errorln("Echo no metadata")
		return nil, fmt.Errorf("Echo: no metadata found")
	}

	log.Infoln("Echo get authorization header")
	authHeader, ok := md["authorization"]
	if !ok {
		log.Errorln("Echo no authorization header")
		return nil, fmt.Errorf("Echo: no authorization header found")
	}

	log.Infoln("Echo VerifyIDTokenAndCheckRevoked")
	token, err := s.authClient.VerifyIDTokenAndCheckRevoked(ctx, authHeader[0])
	if err != nil {
		log.Errorln("Echo VerifyIDTokenAndCheckRevoked", err)
		return nil, err
	}

	log.Infoln("Echo GetUser")
	userRecord, err := s.authClient.GetUser(ctx, token.UID)
	if err != nil {
		log.Errorln("Echo GetUser", err)
		return nil, err
	}
	return &authed.Msg{
		Msg: fmt.Sprintf("hello %v, your email is %v\n", userRecord.DisplayName, userRecord.Email),
	}, nil
}

func (s *Server) authInterceptor(ctx context.Context, r interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
	log.Infoln("authInterceptor authorizing")

	log.Infoln("authorize get metadata")
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		log.Errorln("authorize no metadata")
		return nil, errors.New("authorize: no metadata found")
	}

	log.Infoln("authorize get authHeader")
	authHeader, ok := md["authorization"]
	if !ok {
		log.Errorln("authorize no authHeader")
		return nil, errors.New("authorize: authorization header not found")
	}

	log.Infoln("authorize VerifyIDToken")
	_, err := s.authClient.VerifyIDToken(context.Background(), authHeader[0])
	if err != nil {
		log.Errorln("authorize VerifyIDToken", err)
		return nil, fmt.Errorf("authorize: VerifyIDToken: %v\n", err)
	}

	log.Infoln("authInterceptor authorized")
	return handler(ctx, r)
}
