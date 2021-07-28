package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"strconv"
	"strings"

	"gopkg.in/alecthomas/kingpin.v2"

	"golang.org/x/crypto/ssh/terminal"

	"github.com/gorilla/websocket"
	"github.com/spf13/viper"
)

const (
	VERSION = "0.0.2"
	AUTHOR  = "Vadym Abramchuk <vadym+portainerssh@dev-branch.com>"
	// TODO: Implement fuzzy match and update description.
	USAGE = `
Example:
    portainerssh my-server-1
    portainerssh "my-server*"  (equals to) portainerssh my-server%
    portainerssh %proxy%
    portainerssh "projectA-app-*" (equals to) portainerssh projectA-app-%

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
	// TODO: change Command type to []string, just like Docker does
	Command string
}

type PortainerAPI struct {
	ApiUrl   string
	Endpoint int
	User     string
	Password string
	Jwt      string
}

type WebTerm struct {
	SocketConn *websocket.Conn
	ttyState   *terminal.State
	errChn     chan error
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

func (r *PortainerAPI) getContainerId(name string) string {
	req, _ := http.NewRequest("GET", r.formatHttpApiUrl()+"/endpoints/"+strconv.Itoa(r.Endpoint)+"/docker/containers/json", nil)
	resp, err := r.makeArrReq(req, true)
	if err != nil {
		fmt.Println("Failed to communicate with Portainer API: " + err.Error())
		os.Exit(1)
	}

	var data []map[string]interface{}
	for _, row := range resp {
		if strings.Contains(row["Names"].([]interface{})[0].(string), name) {
			data = append(data, row)
		}
	}

	var choice = 1
	if len(data) == 0 {
		fmt.Println("Container " + name + " not existed in system, not running, or you don't have access permissions.")
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

func (r *PortainerAPI) getExecEndpointId(containerId string, command string) (string, error) {
	jsonBodyData := map[string]interface{}{
		"AttachStdin":  true,
		"AttachStdout": true,
		"AttachStderr": true,
		"Cmd":          []string{command},
		"Tty":          true,
		"id":           containerId,
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

func (r *PortainerAPI) getWsUrl(containerId string, command string) string {
	endpointId, err := r.getExecEndpointId(containerId, command)
	if err != nil {
		fmt.Println("Failed to run exec on container: ", err.Error())
		os.Exit(1)
	}

	// TODO: Implement terminal resize request as well as resizing in runtime.
	//cols, rows, _ := terminal.GetSize(int(os.Stdin.Fd()))
	//req, _ := http.NewRequest("POST", containerId+"?action=execute",
	//	strings.NewReader(fmt.Sprintf(
	//		`{"attachStdin":true, "attachStdout":true,`+
	//			`"command":["/bin/sh", "-c", "TERM=xterm-256color; export TERM; `+
	//			`stty cols %d rows %d; `+
	//			`[ -x /bin/bash ] && ([ -x /usr/bin/script ] && /usr/bin/script -q -c \"/bin/bash\" /dev/null || exec /bin/bash) || exec /bin/sh"], "tty":true}`, cols, rows)))
	//resp, err := r.makeObjReq(req, true)
	jwt, _ := r.getJwt()

	return r.formatWsApiUrl() + "/websocket/exec?token=" + jwt + "&endpointId=1&id=" + endpointId
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

func (r *PortainerAPI) GetContainerConn(name string, command string) *websocket.Conn {
	fmt.Println("Searching for container " + name)
	containerId := r.getContainerId(name)
	fmt.Println("Getting access token")
	wsurl := r.getWsUrl(containerId, command)
	fmt.Println("Connecting to a shell ...")
	return r.getWSConn(wsurl)
}

func ReadConfig() *Config {
	app := kingpin.New("portainerssh", USAGE)
	app.Author(AUTHOR)
	app.Version(VERSION)
	app.HelpFlag.Short('h')

	viper.SetDefault("api_url", "")
	viper.SetDefault("endpoint", "1")
	viper.SetDefault("user", "")
	viper.SetDefault("password", "")
	viper.SetDefault("command", "bash")

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
	var command = app.Flag("command", "Command to execute inside container.").Default(viper.GetString("command")).Short('c').String()
	// TODO: Implement fuzzy match
	var container = app.Arg("container", "Container name, fuzzy match").Required().String()

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
		Command:   *command,
	}

}

func main() {
	config := ReadConfig()
	portainer := PortainerAPI{
		ApiUrl:   config.ApiUrl,
		Endpoint: config.Endpoint,
		User:     config.User,
		Password: config.Password,
	}
	conn := portainer.GetContainerConn(config.Container, config.Command)

	wt := WebTerm{
		SocketConn: conn,
	}
	wt.Run()

	fmt.Println("Good bye.")
}
