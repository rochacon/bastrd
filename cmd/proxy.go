package cmd

import (
	"fmt"
	"log"
	"net/url"

	"github.com/rochacon/bastrd/pkg/proxy"

	"github.com/urfave/cli"
)

var Proxy = cli.Command{
	Name:   "proxy",
	Usage:  "AWS IAM authenticated HTTP proxy.",
	Action: proxyMain,
	Flags: []cli.Flag{
		cli.StringFlag{
			Name:   "bind",
			Usage:  "Address to listen for HTTP requests.",
			EnvVar: "BIND",
			Value:  "0.0.0.0:8080",
		},
		cli.StringFlag{
			Name:   "secret-key",
			Usage:  "Cookie/JWT secret key.",
			EnvVar: "SECRET_KEY",
		},
		cli.StringFlag{
			Name:   "session-cookie-name",
			Usage:  "Cookie/JWT secret key.",
			EnvVar: "SESSION_COOKIE_NAME",
			Value:  "sessionToken",
		},
		cli.StringFlag{
			Name:   "upstream",
			Usage:  "Upstream URL, may include path.",
			EnvVar: "UPSTREAM_URL",
		},
	},
}

func proxyMain(ctx *cli.Context) error {
	secretKey := ctx.String("secret-key")
	if secretKey == "" {
		return fmt.Errorf("Secret key is required.")
	}
	sessionCookieName := ctx.String("session-cookie-name")
	if sessionCookieName == "" {
		return fmt.Errorf("Session cookie name cant be empty.")
	}
	upstreamUrl := ctx.String("upstream")
	upstream, err := url.Parse(upstreamUrl)
	if err != nil {
		return fmt.Errorf("Could not parse upstream: %s", err)
	}
	log.Printf("Upstream: %s", upstream)
	srv := &proxy.Server{
		Addr:              ctx.String("bind"),
		SecretKey:         []byte(secretKey),
		SessionCookieName: sessionCookieName,
		Upstream:          upstream,
	}
	return srv.ListenAndServe()
}
