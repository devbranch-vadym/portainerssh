package portainer

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/gorilla/websocket"
	"github.com/minio/pkg/wildcard"
)

// API handles remote API calls to Portainer.
type API struct {
	ApiUrl   string
	Endpoint int
	User     string
	Password string
	Jwt      string
	ApiKey   string
}

// ContainerExecParams contains details required for connecting to a specific container.
type ContainerExecParams struct {
	ContainerName string
	Command       []string
	User          string
}

// ShellSession contains details about remote shell connected via WebSocket.
type ShellSession struct {
	InstanceId      string
	WsUrl           string
	PortainerApi    *API
	ShellConnection *websocket.Conn
}

func (r *API) formatHttpApiUrl() string {
	if r.ApiUrl[len(r.ApiUrl)-1:len(r.ApiUrl)] == "/" {
		return r.ApiUrl[0 : len(r.ApiUrl)-1]
	}
	return r.ApiUrl
}

func (r *API) formatWsApiUrl() string {
	return "ws" + strings.TrimPrefix(r.formatHttpApiUrl(), "http")
}

func (r *API) makeObjReq(req *http.Request, useAuth bool) (map[string]interface{}, error) {
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

func (r *API) makeArrReq(req *http.Request, useAuth bool) ([]map[string]interface{}, error) {
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

func (r *API) makeReq(req *http.Request, useAuth bool) ([]byte, error) {
	req.Header.Add("Accept", "application/json")
	req.Header.Add("Content-Type", "application/json")
	if useAuth {
		if r.ApiKey != "" {
			req.Header.Add("X-API-Key", r.ApiKey)
		} else {
			jwt, err := r.getJwt()
			if err != nil {
				return nil, err
			}
			req.Header.Add("Authorization", "Bearer "+jwt)
		}
	}

	cli := http.Client{}
	resp, err := cli.Do(req)
	if err != nil {
		return nil, err
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	resp.Body.Close()

	return body, nil
}

func (r *API) getJwt() (string, error) {
	if r.Jwt == "" {
		if r.User == "" || r.Password == "" {
			fmt.Println("Both username and password should be provided for JWT auth.")
			os.Exit(1)
		}

		fmt.Println("Retrieving access token")
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

func (r *API) getContainerId(params *ContainerExecParams) string {
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

func (r *API) spawnExecInstance(containerId string, params *ContainerExecParams) (string, error) {
	jsonBodyData := map[string]interface{}{
		"AttachStdin":  true,
		"AttachStdout": true,
		"AttachStderr": true,
		"Cmd":          params.Command,
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

// GetExecSessionExitCode retrieves exec instance exit code.
func (r *API) GetExecSessionExitCode(execInstanceId string) (int, error) {
	req, _ := http.NewRequest("GET", r.formatHttpApiUrl()+"/endpoints/"+strconv.Itoa(r.Endpoint)+"/docker/exec/"+execInstanceId+"/json", bytes.NewReader(nil))
	resp, err := r.makeObjReq(req, true)

	if err != nil {
		return 0, err
	}

	return int(resp["ExitCode"].(float64)), nil
}

func (r *API) getWsUrl(execInstanceId string, endpointId int) string {
	// Newer versions of Portainer uses headers to pass authentication parameters.
	if r.ApiKey != "" {
		// The token parameter is not used by Portainer, but it is still required.
		return r.formatWsApiUrl() + "/websocket/exec?token=&endpointId=" + strconv.Itoa(endpointId) + "&id=" + execInstanceId
	}

	// Older versions uses token query parameter to pass in the jwt token.
	jwt, _ := r.getJwt()
	return r.formatWsApiUrl() + "/websocket/exec?token=" + jwt + "&endpointId=" + strconv.Itoa(endpointId) + "&id=" + execInstanceId
}

func (r *API) getWSConn(wsUrl string) *websocket.Conn {
	apiUrl := r.formatHttpApiUrl()
	header := http.Header{}
	header.Add("Origin", apiUrl)

	// Use API key if available, otherwise fall back to JWT.
	if r.ApiKey != "" {
		header.Add("X-API-Key", r.ApiKey)
	} else {
		jwt, err := r.getJwt()
		if err != nil {
			fmt.Println("Failed to communicate with Portainer API: " + err.Error())
			os.Exit(1)
		}
		header.Add("Authorization", "Bearer "+jwt)
	}

	conn, _, err := websocket.DefaultDialer.Dial(wsUrl, header)
	if err != nil {
		fmt.Println("We couldn't connect to this container: ", err.Error())
		os.Exit(1)
	}
	return conn
}

// GetContainerConn finds a container to connect, executes a command in it and returns spawned websocket connection.
func (r *API) GetContainerConn(params *ContainerExecParams) ShellSession {
	fmt.Println("Searching for container " + params.ContainerName)
	containerId := r.getContainerId(params)
	execInstanceId, err := r.spawnExecInstance(containerId, params)
	if err != nil {
		fmt.Println("Failed to run exec on container: ", err.Error())
		os.Exit(1)
	}

	wsurl := r.getWsUrl(execInstanceId, r.Endpoint)
	resize, _, _ := r.handleTerminalResize(execInstanceId)

	fmt.Println("Connecting to a shell ...")
	conn := r.getWSConn(wsurl)

	// Trigger terminal resize after connection is established.
	resize <- TriggerResize{}

	return ShellSession{
		InstanceId:      execInstanceId,
		WsUrl:           wsurl,
		PortainerApi:    r,
		ShellConnection: conn,
	}
}
