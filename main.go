package main

import (
	_ "embed"
	"fmt"

	"github.com/devbranch-vadym/portainerssh/internal/config"
	"github.com/devbranch-vadym/portainerssh/pkg/portainer_api"
	"github.com/devbranch-vadym/portainerssh/pkg/wsterm"
)

//go:embed version.txt
var version string

func main() {
	config, params := config.ReadConfig(version)
	portainer := portainer_api.PortainerAPI{
		ApiUrl:   config.ApiUrl,
		Endpoint: config.Endpoint,
		User:     config.User,
		Password: config.Password,
	}
	conn := portainer.GetContainerConn(params)

	wt := wsterm.NewWebTerm(conn)
	wt.Run()

	fmt.Println("Good bye.")
}
