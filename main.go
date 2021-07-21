package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"strings"

	"gopkg.in/alecthomas/kingpin.v2"

	"golang.org/x/crypto/ssh/terminal"

	"github.com/gorilla/websocket"
	"github.com/spf13/viper"
)

const (
	// TODO: Fix author/license
	VERSION = "0.0.1"
	AUTHOR  = "Vadym Abramchuk <vadym+portainerssh@dev-branch.com>"
	// TODO: Implement fuzzy match and update description.
	USAGE   = `
Example:
    portainerssh my-server-1
    portainerssh "my-server*"  (equals to) portainerssh my-server%
    portainerssh %proxy%
    portainerssh "projectA-app-*" (equals to) portainerssh projectA-app-%

Configuration:
    We read configuration from config.json or config.yml in ./, /etc/portainerssh/ and ~/.portainerssh/ folders.

    If you want to use JSON format, create a config.json in the folders with content:
        {
            "endpoint": "https://portainerssh.server/api",
            "user": "your_access_key",
            "password": "your_access_password"
        }

    If you want to use YAML format, create a config.yml with content:
        endpoint: https://your.portainer.server/api
        user: your_access_key
        password: your_access_password

    We accept environment variables as well:
        PORTAINER_ENDPOINT=https://your.portainer.server/api
        PORTAINER_USER=your_access_key
        PORTAINER_PASSWORD=your_access_password
`
)

type Config struct {
	Container string
	Endpoint  string
	User      string
	Password  string
}

type PortainerAPI struct {
	Endpoint string
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

func (r *PortainerAPI) formatEndpoint() string {
	if r.Endpoint[len(r.Endpoint)-1:len(r.Endpoint)] == "/" {
		return r.Endpoint[0 : len(r.Endpoint)-1]
	} else {
		return r.Endpoint
	}
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
		req, _ := http.NewRequest("POST", r.formatEndpoint()+"/auth", bytes.NewReader(body))

		resp, err := r.makeObjReq(req, false)
		if err != nil {
			return "", err
		}

		r.Jwt = resp["jwt"].(string)
	}

	return r.Jwt, nil
}

func (r *PortainerAPI) getContainerId(name string) string {
	req, _ := http.NewRequest("GET", r.formatEndpoint()+"/endpoints/1/docker/containers/json", nil)
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

func (r *PortainerAPI) getExecEndpointId(containerId string) (string, error) {
	jsonBodyData := map[string]interface{}{
		"AttachStdin":  true,
		"AttachStdout": true,
		"AttachStderr": true,
		"Cmd":          []string{"bash"},
		"Tty":          true,
		"id":           containerId,
	}
	body, err := json.Marshal(jsonBodyData)
	if err != nil {
		return "", err
	}
	req, _ := http.NewRequest("POST", r.formatEndpoint()+"/endpoints/1/docker/containers/"+containerId+"/exec", bytes.NewReader(body))
	resp, err := r.makeObjReq(req, true)

	if err != nil {
		return "", err
	}

	return resp["Id"].(string), nil
}

func (r *PortainerAPI) getWsUrl(containerId string) string {
	endpointId, err := r.getExecEndpointId(containerId)
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

	// TODO: Fix API URL
	return "ws://localhost:9000/api/websocket/exec?token=" + jwt + "&endpointId=1&id=" + endpointId
}

func (r *PortainerAPI) getWSConn(wsUrl string) *websocket.Conn {
	endpoint := r.formatEndpoint()
	header := http.Header{}
	header.Add("Origin", endpoint)
	conn, _, err := websocket.DefaultDialer.Dial(wsUrl, header)
	if err != nil {
		fmt.Println("We couldn't connect to this container: ", err.Error())
		os.Exit(1)
	}
	return conn
}

func (r *PortainerAPI) GetContainerConn(name string) *websocket.Conn {
	fmt.Println("Searching for container " + name)
	containerId := r.getContainerId(name)
	fmt.Println("Getting access token")
	wsurl := r.getWsUrl(containerId)
	fmt.Println("Connecting to a shell ...")
	return r.getWSConn(wsurl)
}

func ReadConfig() *Config {
	app := kingpin.New("portainerssh", USAGE)
	app.Author(AUTHOR)
	app.Version(VERSION)
	app.HelpFlag.Short('h')

	viper.SetDefault("endpoint", "")
	viper.SetDefault("user", "")
	viper.SetDefault("password", "")

	viper.SetConfigName("config")              // name of config file (without extension)
	viper.AddConfigPath(".")                   // call multiple times to add many search paths
	viper.AddConfigPath("$HOME/.portainerssh") // call multiple times to add many search paths
	viper.AddConfigPath("/etc/portainerssh/")  // path to look for the config file in
	viper.ReadInConfig()

	viper.SetEnvPrefix("portainer")
	viper.AutomaticEnv()

	var endpoint = app.Flag("endpoint", "Portainer server endpoint, https://your.portainer.server/api .").Default(viper.GetString("endpoint")).String()
	var user = app.Flag("user", "Portainer API user/accesskey.").Default(viper.GetString("user")).String()
	var password = app.Flag("password", "Portainer API password/secret.").Default(viper.GetString("password")).String()
	// TODO: Implement fuzzy match
	var container = app.Arg("container", "Container name, fuzzy match").Required().String()

	app.Parse(os.Args[1:])

	if *endpoint == "" || *user == "" || *password == "" || *container == "" {
		app.Usage(os.Args[1:])
		os.Exit(1)
	}

	return &Config{
		Container: *container,
		Endpoint:  *endpoint,
		User:      *user,
		Password:  *password,
	}

}

func main() {
	config := ReadConfig()
	portainer := PortainerAPI{
		Endpoint: config.Endpoint,
		User:     config.User,
		Password: config.Password,
	}
	conn := portainer.GetContainerConn(config.Container)

	wt := WebTerm{
		SocketConn: conn,
	}
	wt.Run()

	fmt.Println("Good bye.")
}
