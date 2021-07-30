package portainer_api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"strconv"
)

type TerminalDimensions struct {
	Width  int
	Height int
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
