package config

import (
	"os"
	"strings"

	"github.com/spf13/viper"
	"gopkg.in/alecthomas/kingpin.v2"

	"github.com/devbranch-vadym/portainerssh/pkg/portainer"
	"github.com/google/shlex"
)

const (
	author = "Vadym Abramchuk <vadym+portainerssh@dev-branch.com>"
	usage  = `
Connect to container by it's name:
	portainerssh my-server-1
Substitute single character:
	portainerssh "my-server-?"
Connect any container matching pattern:
    portainerssh "%server%"

Wildcards matching:
   "?" matches any single character. "%" matches zero or more characters.

Configuration:
    We read configuration from config.json or config.yml in ./, /etc/portainerssh/ and ~/.portainerssh/ folders.

    If you want to use JSON format, create a config.json in the folders with content:
        {
            "api_url": "https://portainerssh.server/api",
            "user": "your_access_key",
            "password": "your_access_password",
            "endpoint": 1,
            "api_key": "my-api-key-here"
        }

    If you want to use YAML format, create a config.yml with content:
        api_url: https://your.portainer.server/api
        user: your_access_key
        password: your_access_password
        endpoint: 1
        api_key: my-api-key-here

    We accept environment variables as well:
        PORTAINER_API_URL=https://your.portainer.server/api
        PORTAINER_USER=your_access_key
        PORTAINER_PASSWORD=your_access_password
`
)

// Config is a runtime configuration.
type Config struct {
	ApiUrl   string
	Endpoint int
	User     string
	Password string
	ApiKey   string
}

// ReadConfig gathers configuration values from all available sources and returns everything required to connect
// to Portainer API and execute a command in container.
func ReadConfig(version string) (*Config, *portainer.ContainerExecParams) {
	app := kingpin.New("portainerssh", usage)
	app.Author(author)
	app.Version(strings.TrimSpace(version))
	app.HelpFlag.Short('h')

	viper.SetDefault("api_url", "")
	viper.SetDefault("endpoint", "1")
	viper.SetDefault("user", "")
	viper.SetDefault("password", "")
	viper.SetDefault("api_key", "")

	viper.SetConfigName("config")              // name of config file (without extension)
	viper.AddConfigPath(".")                   // call multiple times to add many search paths
	viper.AddConfigPath("$HOME/.portainerssh") // call multiple times to add many search paths
	viper.AddConfigPath("/etc/portainerssh/")  // path to look for the config file in
	viper.ReadInConfig()

	viper.SetEnvPrefix("portainer")
	viper.AutomaticEnv()

	var apiUrl = app.Flag("api_url", "Portainer server API URL, https://your.portainer.server/api .").Default(viper.GetString("api_url")).String()
	var endpoint = app.Flag("endpoint", "Portainer endpoint ID. Default is 1.").Default(viper.GetString("endpoint")).Int()
	var user = app.Flag("user", "Portainer API user/accesskey.").Default(viper.GetString("user")).String()
	var password = app.Flag("password", "Portainer API password/secret.").Default(viper.GetString("password")).String()
	var apiKey = app.Flag("api_key", "Portainer API key.").Default(viper.GetString("api_key")).String()

	var container = app.Arg("container", "Container name, wildcards allowed").Required().String()
	var command = app.Flag("command", "Command to execute inside container.").Default("bash").Short('c').String()
	var runAs = app.Flag("run_as_user", "User to execute container command as.").Default("").Short('u').String()
	var workdir = app.Flag("workdir", "Working directory to execute command in.").Default("").Short('w').String()

	app.Parse(os.Args[1:])

	var hasAuthCredentials = (*user != "" && *password != "") || *apiKey != ""

	if *apiUrl == "" || *endpoint == 0 || !hasAuthCredentials || *container == "" {
		app.Usage(os.Args[1:])
		os.Exit(1)
	}

	// TODO: Handle shlex.Split errors
	commandParts, _ := shlex.Split(*command)

	return &Config{
			ApiUrl:   *apiUrl,
			Endpoint: *endpoint,
			User:     *user,
			Password: *password,
			ApiKey:   *apiKey,
		}, &portainer.ContainerExecParams{
			ContainerName: *container,
			Command:       commandParts,
			User:          *runAs,
			WorkingDir:    *workdir,
		}

}
