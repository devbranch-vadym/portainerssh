package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	tsize "github.com/kopoli/go-terminal-size"
	"io/ioutil"
	"net/http"
	"os"
	"strconv"
	"strings"

	"gopkg.in/alecthomas/kingpin.v2"

	"golang.org/x/crypto/ssh/terminal"

	"github.com/gorilla/websocket"
	"github.com/minio/pkg/wildcard"
	"github.com/spf13/viper"

	_ "embed"
)

//go:embed version.txt
var VERSION string

const (
	AUTHOR = "Vadym Abramchuk <vadym+portainerssh@dev-branch.com>"
	USAGE  = `
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
            "password": "your_access_password"
        }

    If you want to use YAML format, create a config.yml with content:
        api_url: https://your.portainer.server/api
        user: your_access_key
        password: your_access_password

    We accept environment variables as well:
        PORTAINER_API_URL=https://your.portainer.server/api
        PORTAINER_USER=your_access_key
        PORTAINER_PASSWORD=your_access_password
`
)

type Config struct {
	Container string
	ApiUrl    string
	Endpoint  int
	User      string
	Password  string
}

type PortainerAPI struct {
	ApiUrl   string
	Endpoint int
	User     string
	Password string
	Jwt      string
}

type ContainerExecParams struct {
	ContainerName string
	// TODO: change Command type to []string, just like Docker does
	Command string
	User    string
}

type WebTerm struct {
	SocketConn *websocket.Conn
	ttyState   *terminal.State
	errChn     chan error
}

type TerminalDimensions struct {
	Width  int
	Height int
}

func (w *WebTerm) wsWrite() {
	var err error
	var keybuf [1]byte
	for {
		_, err = os.Stdin.Read(keybuf[0:1])
		if err != nil {
			w.errChn <- err
			return
		}

		err = w.SocketConn.WriteMessage(websocket.BinaryMessage, keybuf[0:1])
		if err != nil {
			if websocket.IsCloseError(err, websocket.CloseNormalClosure) {
				w.errChn <- nil
			} else {
				w.errChn <- err
			}
			return
		}
	}
}

func (w *WebTerm) wsRead() {
	var err error
	var raw []byte
	for {
		_, raw, err = w.SocketConn.ReadMessage()
		if err != nil {
			if websocket.IsCloseError(err, websocket.CloseNormalClosure) {
				w.errChn <- nil
			} else {
				w.errChn <- err
			}
			return
		}
		os.Stdout.Write(raw)
	}
}

func (w *WebTerm) SetRawtty(isRaw bool) {
	if isRaw {
		state, err := terminal.MakeRaw(int(os.Stdin.Fd()))
		if err != nil {
			panic(err)
		}
		w.ttyState = state
	} else {
		terminal.Restore(int(os.Stdin.Fd()), w.ttyState)
	}
}

func (w *WebTerm) Run() {
	w.errChn = make(chan error)
	w.SetRawtty(true)

	go w.wsRead()
	go w.wsWrite()

	err := <-w.errChn
	w.SetRawtty(false)

	if err != nil {
		panic(err)
	}
}

func (r *PortainerAPI) formatHttpApiUrl() string {
	if r.ApiUrl[len(r.ApiUrl)-1:len(r.ApiUrl)] == "/" {
		return r.ApiUrl[0 : len(r.ApiUrl)-1]
	} else {
		return r.ApiUrl
	}
}

func (r *PortainerAPI) formatWsApiUrl() string {
	return "ws" + strings.TrimPrefix(r.formatHttpApiUrl(), "http")
}

func (r *PortainerAPI) makeObjReq(req *http.Request, useAuth bool) (map[string]interface{}, error) {
	body, err := r.makeReq(req, useAuth)
	if err != nil {
		return nil, err
	}

	var apiResp map[string]interface{}
	if err = json.Unmarshal(body, &apiResp); err != nil {
		return nil, err
	}
	return apiResp, nil
}

func (r *PortainerAPI) makeArrReq(req *http.Request, useAuth bool) ([]map[string]interface{}, error) {
	body, err := r.makeReq(req, useAuth)
	if err != nil {
		return nil, err
	}

	var apiResp []map[string]interface{}
	if err = json.Unmarshal(body, &apiResp); err != nil {
		return nil, err
	}
	return apiResp, nil
}

func (r *PortainerAPI) makeReq(req *http.Request, useAuth bool) ([]byte, error) {
	req.Header.Add("Accept", "application/json")
	req.Header.Add("Content-Type", "application/json")
	if useAuth {
		jwt, err := r.getJwt()
		if err != nil {
			return nil, err
		}
		req.Header.Add("Authorization", "Bearer "+jwt)
	}

	cli := http.Client{}
	resp, err := cli.Do(req)
	if err != nil {
		return nil, err
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	resp.Body.Close()

	return body, nil
}

func (r *PortainerAPI) getJwt() (string, error) {
	if r.Jwt == "" {
		jsonBodyData := map[string]interface{}{
			"username": r.User,
			"password": r.Password,
		}
		body, err := json.Marshal(jsonBodyData)
		if err != nil {
			return "", err
		}
		req, _ := http.NewRequest("POST", r.formatHttpApiUrl()+"/auth", bytes.NewReader(body))

		resp, err := r.makeObjReq(req, false)
		if err != nil {
			return "", err
		}

		r.Jwt = resp["jwt"].(string)
	}

	return r.Jwt, nil
}

func (r *PortainerAPI) getContainerId(params *ContainerExecParams) string {
	req, _ := http.NewRequest("GET", r.formatHttpApiUrl()+"/endpoints/"+strconv.Itoa(r.Endpoint)+"/docker/containers/json", nil)
	resp, err := r.makeArrReq(req, true)
	if err != nil {
		fmt.Println("Failed to communicate with Portainer API: " + err.Error())
		os.Exit(1)
	}

	var data []map[string]interface{}
	for _, row := range resp {
		if wildcard.Match(
			strings.Replace(params.ContainerName, "%", "*", -1),
			strings.TrimLeft(row["Names"].([]interface{})[0].(string), "/"),
		) {
			data = append(data, row)
		}
	}

	var choice = 1
	if len(data) == 0 {
		fmt.Println("Container " + params.ContainerName + " not existed in system, not running, or you don't have access permissions.")
		os.Exit(1)
	}
	if len(data) > 1 {
		fmt.Println("We found more than one containers in system:")
		for i, ctn := range data {
			fmt.Println(fmt.Sprintf("[%d] Container: %s, ID %s", i+1, ctn["Names"].([]interface{})[0].(string), ctn["Id"].(string)))
		}
		fmt.Println("--------------------------------------------")
		fmt.Print("Which one you want to connect: ")
		fmt.Scan(&choice)
	}
	ctn := data[choice-1]
	fmt.Println(fmt.Sprintf("Target Container: %s, ID %s", ctn["Names"].([]interface{})[0].(string), ctn["Id"].(string)))
	return ctn["Id"].(string)
}

func (r *PortainerAPI) getExecEndpointId(containerId string, params *ContainerExecParams) (string, error) {
	jsonBodyData := map[string]interface{}{
		"AttachStdin":  true,
		"AttachStdout": true,
		"AttachStderr": true,
		"Cmd":          []string{params.Command},
		"Tty":          true,
		"id":           containerId,
	}
	if params.User != "" {
		jsonBodyData["User"] = params.User
	}
	body, err := json.Marshal(jsonBodyData)
	if err != nil {
		return "", err
	}
	req, _ := http.NewRequest("POST", r.formatHttpApiUrl()+"/endpoints/"+strconv.Itoa(r.Endpoint)+"/docker/containers/"+containerId+"/exec", bytes.NewReader(body))
	resp, err := r.makeObjReq(req, true)

	if err != nil {
		return "", err
	}

	return resp["Id"].(string), nil
}

func (r *PortainerAPI) getWsUrl(containerId string, params *ContainerExecParams) string {
	endpointId, err := r.getExecEndpointId(containerId, params)
	if err != nil {
		fmt.Println("Failed to run exec on container: ", err.Error())
		os.Exit(1)
	}

	// TODO: Connect to a tsize channel.
	s, err := tsize.GetSize()
	if err != nil {
		fmt.Println("GetSize failed: ", err.Error())
	} else {
		// TODO: That's not really a correct approach; Portainer is expecting resize request _after_ WS connection
		//  is established.
		go r.resizeTerminal(endpointId, TerminalDimensions{Height: s.Height, Width: s.Width})
	}

	jwt, _ := r.getJwt()

	return r.formatWsApiUrl() + "/websocket/exec?token=" + jwt + "&endpointId=1&id=" + endpointId
}

func (r *PortainerAPI) resizeTerminal(execEndpointId string, dimensions TerminalDimensions) (map[string]interface{}, error) {
	jsonBodyData := map[string]interface{}{
		"Height": dimensions.Height,
		"Width":  dimensions.Width,
		"id":     execEndpointId,
	}
	body, err := json.Marshal(jsonBodyData)
	if err != nil {
		return nil, err
	}
	req, _ := http.NewRequest("POST", r.formatHttpApiUrl()+"/endpoints/"+strconv.Itoa(r.Endpoint)+"/docker/exec/"+execEndpointId+"/resize?h="+strconv.Itoa(dimensions.Height)+"&w="+strconv.Itoa(dimensions.Width), bytes.NewReader(body))

	return r.makeObjReq(req, true)
}

func (r *PortainerAPI) getWSConn(wsUrl string) *websocket.Conn {
	apiUrl := r.formatHttpApiUrl()
	header := http.Header{}
	header.Add("Origin", apiUrl)
	conn, _, err := websocket.DefaultDialer.Dial(wsUrl, header)
	if err != nil {
		fmt.Println("We couldn't connect to this container: ", err.Error())
		os.Exit(1)
	}
	return conn
}

func (r *PortainerAPI) GetContainerConn(params *ContainerExecParams) *websocket.Conn {
	fmt.Println("Searching for container " + params.ContainerName)
	containerId := r.getContainerId(params)
	fmt.Println("Getting access token")
	wsurl := r.getWsUrl(containerId, params)
	fmt.Println("Connecting to a shell ...")
	return r.getWSConn(wsurl)
}

func ReadConfig() (*Config, *ContainerExecParams) {
	app := kingpin.New("portainerssh", USAGE)
	app.Author(AUTHOR)
	app.Version(strings.TrimSpace(VERSION))
	app.HelpFlag.Short('h')

	viper.SetDefault("api_url", "")
	viper.SetDefault("endpoint", "1")
	viper.SetDefault("user", "")
	viper.SetDefault("password", "")

	viper.SetConfigName("config")              // name of config file (without extension)
	viper.AddConfigPath(".")                   // call multiple times to add many search paths
	viper.AddConfigPath("$HOME/.portainerssh") // call multiple times to add many search paths
	viper.AddConfigPath("/etc/portainerssh/")  // path to look for the config file in
	viper.ReadInConfig()

	viper.SetEnvPrefix("portainer")
	viper.AutomaticEnv()

	var api_url = app.Flag("api_url", "Portainer server API URL, https://your.portainer.server/api .").Default(viper.GetString("api_url")).String()
	var endpoint = app.Flag("endpoint", "Portainer endpoint ID. Default is 1.").Default(viper.GetString("endpoint")).Int()
	var user = app.Flag("user", "Portainer API user/accesskey.").Default(viper.GetString("user")).String()
	var password = app.Flag("password", "Portainer API password/secret.").Default(viper.GetString("password")).String()

	var container = app.Arg("container", "Container name, wildcards allowed").Required().String()
	var command = app.Flag("command", "Command to execute inside container.").Default("bash").Short('c').String()
	var runAs = app.Flag("run_as_user", "User to execute container command as.").Default("").Short('u').String()

	app.Parse(os.Args[1:])

	if *api_url == "" || *endpoint == 0 || *user == "" || *password == "" || *container == "" {
		app.Usage(os.Args[1:])
		os.Exit(1)
	}

	return &Config{
			Container: *container,
			ApiUrl:    *api_url,
			Endpoint:  *endpoint,
			User:      *user,
			Password:  *password,
		}, &ContainerExecParams{
			ContainerName: *container,
			Command:       *command,
			User:          *runAs,
		}

}

func main() {
	config, params := ReadConfig()
	portainer := PortainerAPI{
		ApiUrl:   config.ApiUrl,
		Endpoint: config.Endpoint,
		User:     config.User,
		Password: config.Password,
	}
	conn := portainer.GetContainerConn(params)

	wt := WebTerm{
		SocketConn: conn,
	}
	wt.Run()

	fmt.Println("Good bye.")
}
