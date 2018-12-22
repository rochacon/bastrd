package main

import (
	"fmt"
	"log"
	"os"
	"runtime"

	"github.com/rochacon/bastrd/cmd"
	"github.com/urfave/cli"
)

// App version, injected on build time
var VERSION string = "dev"

func main() {
	app := cli.NewApp()
	app.Name = "bastrd"
	app.Usage = "bastion server for secure environments"
	app.Version = fmt.Sprintf("%s %s", VERSION, runtime.Version())
	app.Commands = []cli.Command{
		cmd.AuthorizedKeys,
		cmd.CreateUser,
		cmd.Sync,
		cmd.Toolbox,
	}
	if err := app.Run(os.Args); err != nil {
		log.Fatal(err)
	}
}
