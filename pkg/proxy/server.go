package proxy

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"os/signal"
	"time"

	"github.com/rochacon/bastrd/pkg/auth"

	jwt "github.com/dgrijalva/jwt-go"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Server implements a simple reverse proxy server authenticating on AWS IAM
type Server struct {
	Addr              string
	SecretKey         []byte
	SessionCookieName string
	Upstream          *url.URL
}

// ListenAndServer starts the HTTP server.
// This server respects SIGINT and will gracefully shutdown.
func (s *Server) ListenAndServe() error {
	mux := http.NewServeMux()
	mux.HandleFunc("/", s.ServeHTTP)
	mux.HandleFunc("/healthz", s.Health)
	mux.HandleFunc("/login", s.Login)
	mux.HandleFunc("/logout", s.Logout)
	mux.Handle("/metrics", promhttp.Handler())
	log.Println("Listening on", s.Addr)
	drained := make(chan error)
	sigint := make(chan os.Signal)
	signal.Notify(sigint, os.Interrupt)
	srv := &http.Server{
		Addr:    s.Addr,
		Handler: mux,
	}
	go func() {
		<-sigint
		log.Println("Received SIGINT, draining connection")
		ctx, cancel := context.WithTimeout(context.Background(), time.Second*30)
		defer cancel()
		drained <- srv.Shutdown(ctx)
	}()
	err := srv.ListenAndServe()
	if err != nil && err != http.ErrServerClosed {
		return err
	}
	err = <-drained
	log.Printf("Done")
	return err
}

// Health returns a successful health check
func (s *Server) Health(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte("ok"))
}

// write mainServeHTTP that validates token and route to appropriate serve method
// valid token: proxy to upstream.
// invalid token redirect to login
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// check jwt in cookie, if good call proxy
	sessionCookie, err := r.Cookie(s.SessionCookieName)
	if err != nil {
		http.Redirect(w, r, "/login?error=invalid_cookie", 302)
		return
	}
	tkn, err := s.jwtParse(sessionCookie.Value)
	if err != nil {
		http.Redirect(w, r, "/login?error=invalid_token", 302)
		return
	}
	log.Printf("Proxying user %q %q %q", tkn["username"], r.Method, r.URL)
	s.Proxy(w, r)
}

// proxy request to upstream with net/http/httputil.SingleHostReverseProxy
func (s *Server) Proxy(w http.ResponseWriter, r *http.Request) {
	p := httputil.NewSingleHostReverseProxy(s.Upstream)
	r.URL = s.Upstream
	p.ServeHTTP(w, r)
}

// login validates basic auth of username and secret+mfa on AWS IAM and sets cookie with session jwt
func (s *Server) Login(w http.ResponseWriter, r *http.Request) {
	username, password, ok := r.BasicAuth()
	if !ok {
		w.Header().Set("WWW-Authenticate", "Basic realm=\"Provide your credentials\"")
		http.Error(w, "Unauthorized", 401)
		return
	}
	lenPassword := len(password)
	if lenPassword < 7 {
		w.Header().Set("WWW-Authenticate", "Basic realm=\"Invalid credentials\"")
		http.Error(w, "Unauthorized", 401)
		return
	}
	expiration := time.Duration(time.Hour * 2)
	secretKey, mfaToken := password[:lenPassword-6], password[lenPassword-6:]
	_, err := auth.NewSessionCredentials(username, secretKey, mfaToken, expiration)
	if err != nil {
		log.Printf("Failed authentication for %q", username)
		w.Header().Set("WWW-Authenticate", "Basic realm=\"Invalid credentials\"")
		http.Error(w, "Unauthorized", 401)
		return
	}
	jwtToken, err := s.jwtNew(username, expiration)
	if err != nil {
		http.Error(w, fmt.Sprintf("Unexpected error: %s", err), 500)
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name:     s.SessionCookieName,
		Value:    jwtToken,
		Path:     "/",
		MaxAge:   int(expiration.Seconds()),
		HttpOnly: true,
		Secure:   true,
	})
	http.Redirect(w, r, "/", 302)
}

// logout kills cookie and redirect to /
func (s *Server) Logout(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{
		Name:     s.SessionCookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   true,
	})
	http.Redirect(w, r, "/", 302)
}

// jwtNew create a new JWT for a user
func (s *Server) jwtNew(username string, expires time.Duration) (string, error) {
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"username": username,
		"exp":      time.Now().Add(expires).Unix(),
	})
	tokenString, err := token.SignedString(s.SecretKey)
	if err != nil {
		return "", err
	}
	return tokenString, nil
}

// jwtParse takes a token string and a function for looking up the key. The latter is especially
// useful if you use multiple keys for your application.  The standard is to use 'kid' in the
// head of the token to identify which key to use, but the parsed token (head and claims) is provided
// to the callback, providing flexibility.
func (s *Server) jwtParse(jwtToken string) (jwt.MapClaims, error) {
	token, err := jwt.Parse(jwtToken, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("Unexpected signing method: %v", token.Header["alg"])
		}
		return s.SecretKey, nil
	})
	if err != nil {
		return nil, fmt.Errorf("Invalid token: %s", err)
	}
	if !token.Valid {
		return nil, fmt.Errorf("Invalid token")
	}
	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok || claims.Valid() != nil {
		return nil, fmt.Errorf("Invalid token contents")
	}
	return claims, nil
}
