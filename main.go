package main

import (
	_ "embed"
	"fmt"

	"github.com/devbranch-vadym/portainerssh/internal/config"
	"github.com/devbranch-vadym/portainerssh/pkg/portainer"
	"github.com/devbranch-vadym/portainerssh/pkg/wsterm"
)

//go:embed version.txt
var version string

func main() {
	config, params := config.ReadConfig(version)
	portainer := portainer.API{
		ApiUrl:   config.ApiUrl,
		Endpoint: config.Endpoint,
		User:     config.User,
		Password: config.Password,
	}
	conn := portainer.GetContainerConn(params)

	wt := wsterm.WebTerm{
		SocketConn: conn,
	}
	wt.Run()

	fmt.Println("Good bye.")
}
