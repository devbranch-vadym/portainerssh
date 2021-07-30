package portainer

import (
	"bytes"
	"encoding/json"
	"net/http"
	"strconv"
)

// TerminalDimensions is a simple struct containing current user's terminal width and height.
type TerminalDimensions struct {
	Width  int
	Height int
}

func (r *API) resizeTerminal(execEndpointId string, dimensions TerminalDimensions) (map[string]interface{}, error) {
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
