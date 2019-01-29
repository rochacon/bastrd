package cmd

import (
	"fmt"
	"log"
	"net/url"
	"time"

	"github.com/rochacon/bastrd/pkg/proxy"

	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/iam"
	"github.com/urfave/cli"
)

var Proxy = cli.Command{
	Name:   "proxy",
	Usage:  "AWS IAM authenticated HTTP proxy.",
	Action: proxyMain,
	Flags: []cli.Flag{
		cli.StringSliceFlag{
			Name:  "allowed-groups",
			Usage: "Comma separated list of AWS IAM Groups allowed to authenticate. (defaults to empty, which allows all)",
		},
		cli.DurationFlag{
			Name:  "group-cache-period",
			Usage: "Duration of the allowed group cache.",
			Value: 5 * time.Minute,
		},
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
	if upstreamUrl == "" {
		return fmt.Errorf("Upstream URL is required.")
	}
	upstream, err := url.Parse(upstreamUrl)
	if err != nil {
		return fmt.Errorf("Could not parse upstream: %s", err)
	}
	log.Printf("Forwarding requests to: %s", upstream)
	srv := proxy.New(ctx.String("bind"), []byte(secretKey), upstream)
	srv.AllowedGroups = ctx.StringSlice("allowed-groups")
	srv.GroupCachePeriod = ctx.Duration("group-cache-period")
	srv.IAM = iam.New(session.New())
	srv.SessionCookieName = sessionCookieName
	return srv.ListenAndServe()
}
