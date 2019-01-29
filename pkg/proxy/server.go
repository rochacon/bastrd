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
	"strings"
	"sync"
	"time"

	"github.com/rochacon/bastrd/pkg/auth"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/iam"
	jwt "github.com/dgrijalva/jwt-go"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Server implements a simple reverse proxy server authenticating on AWS IAM
type Server struct {
	Addr              string
	AllowedGroups     []string
	IAM               *iam.IAM
	SecretKey         []byte
	SessionCookieName string
	Upstream          *url.URL
	upstreamProxy     *httputil.ReverseProxy
	groupCache        map[string]*iam.GetGroupOutput
	GroupCachePeriod  time.Duration
	groupCacheMutex   *sync.RWMutex
}

// New instantiates a default server
func New(addr string, secretKey []byte, upstream *url.URL) *Server {
	s := &Server{
		Addr:              addr,
		SecretKey:         secretKey,
		SessionCookieName: "sessionToken",
		Upstream:          upstream,
	}
	s.groupCacheMutex = &sync.RWMutex{}
	s.upstreamProxy = s.buildProxy()
	return s
}

// groupCacheManager manages the allowed groups cache
func (s *Server) groupCacheManager() error {
	if s.groupCache == nil {
		s.groupCache = map[string]*iam.GetGroupOutput{}
	}
	if len(s.AllowedGroups) == 0 {
		return fmt.Errorf("empty list of allowed groups, disabling group cache")
	}
	err := make(chan error)
	go func() {
		for {
			log.Printf("group cache sync started")
			s.groupCacheMutex.Lock()
			for _, group := range s.AllowedGroups {
				grp, err := s.IAM.GetGroup(&iam.GetGroupInput{GroupName: aws.String(group)})
				if err != nil {
					log.Printf("failed to sync group %q: %s", group, err)
					continue
				}
				s.groupCache[*grp.Group.GroupName] = grp
			}
			s.groupCacheMutex.Unlock()
			log.Printf("group cache sync finished")
			<-time.After(s.GroupCachePeriod)
		}
	}()
	return <-err
}

// ListenAndServer starts the HTTP server.
// This server respects SIGINT and will gracefully shutdown.
func (s *Server) ListenAndServe() error {
	go func() {
		log.Printf("groupCacheManager exit: %s", s.groupCacheManager())
	}()
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
		http.Redirect(w, r, "/login?error=invalid_cookie", http.StatusFound)
		return
	}
	tkn, err := s.jwtParse(sessionCookie.Value)
	if err != nil {
		http.Redirect(w, r, "/login?error=invalid_token", http.StatusFound)
		return
	}
	log.Printf("Proxying user %q %q %q", tkn["username"], r.Method, r.URL)
	s.Proxy(w, r)
}

// proxy request to upstream with net/http/httputil.SingleHostReverseProxy
func (s *Server) Proxy(w http.ResponseWriter, r *http.Request) {
	if strings.HasPrefix(r.URL.Path, s.Upstream.Path) == false {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}
	s.upstreamProxy.ServeHTTP(w, r)
}

// buildProxy sets up a simple reverse proxy server
func (s *Server) buildProxy() *httputil.ReverseProxy {
	// director based on httputil.NewSingleHostReverseProxy without path joining
	// and dropping Authorization and Cookie headers
	director := func(r *http.Request) {
		r.URL.Scheme = s.Upstream.Scheme
		r.URL.Host = s.Upstream.Host
		if s.Upstream.RawQuery == "" || r.URL.RawQuery == "" {
			r.URL.RawQuery = s.Upstream.RawQuery + r.URL.RawQuery
		} else {
			r.URL.RawQuery = s.Upstream.RawQuery + "&" + r.URL.RawQuery
		}
		r.Header.Del("Authorization")
		r.Header.Del("Cookie")
		if _, ok := r.Header["User-Agent"]; !ok {
			// explicitly disable User-Agent so it's not set to default value
			r.Header.Set("User-Agent", "")
		}
	}
	return &httputil.ReverseProxy{Director: director}
}

// login validates basic auth of username and secret+mfa on AWS IAM and sets cookie with session jwt
func (s *Server) Login(w http.ResponseWriter, r *http.Request) {
	username, password, ok := r.BasicAuth()
	if !ok {
		w.Header().Set("WWW-Authenticate", "Basic realm=\"Provide your credentials\"")
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	lenPassword := len(password)
	if lenPassword < 7 {
		w.Header().Set("WWW-Authenticate", "Basic realm=\"Invalid credentials\"")
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	expiration := time.Duration(time.Hour * 2)
	secretKey, mfaToken := password[:lenPassword-6], password[lenPassword-6:]
	_, err := auth.NewSessionCredentials(username, secretKey, mfaToken, expiration)
	if err != nil {
		log.Printf("Failed authentication for %q: %s", username, err)
		w.Header().Set("WWW-Authenticate", "Basic realm=\"Invalid credentials\"")
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	if s.userInAllowedGroups(username) == false {
		log.Printf("Failed authentication for %q: user does not belong to allowed groups", username)
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}
	jwtToken, err := s.jwtNew(username, expiration)
	if err != nil {
		log.Printf("Unexpected error while authenticating %q: %s", username, err)
		http.Error(w, fmt.Sprintf("Unexpected error: %s", err), http.StatusInternalServerError)
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
	http.Redirect(w, r, s.Upstream.Path, http.StatusFound)
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

// userInAllowedGroups checks wether the user belongs to the AllowedGroups
// if allowed groups list is empty all users are allowed
func (s *Server) userInAllowedGroups(username string) bool {
	if len(s.AllowedGroups) == 0 {
		return true
	}
	s.groupCacheMutex.RLock()
	defer s.groupCacheMutex.RUnlock()
	for _, group := range s.groupCache {
		for _, user := range group.Users {
			if *user.UserName == username {
				return true
			}
		}
	}
	return false
}
