package main

import (
	_ "embed"
	"os"

	"github.com/devbranch-vadym/portainerssh/internal/config"
	"github.com/devbranch-vadym/portainerssh/pkg/portainer"
	"github.com/devbranch-vadym/portainerssh/pkg/wsterm"
)

//go:embed version.txt
var version string

func main() {
	config, params := config.ReadConfig(version)
	api := portainer.API{
		ApiUrl:   config.ApiUrl,
		Endpoint: config.Endpoint,
		User:     config.User,
		Password: config.Password,
	}
	conn := api.GetContainerConn(params)

	wt := wsterm.NewWebTerm(conn.ShellConnection)
	wt.Run()

	exitCode, err := conn.PortainerApi.GetExecSessionExitCode(conn.InstanceId)
	if err != nil {
		panic(err)
	}

	os.Exit(exitCode)
}
