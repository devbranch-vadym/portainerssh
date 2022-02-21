package portainer

import (
	"bytes"
	"encoding/json"
	"net/http"
	"strconv"

	tsize "github.com/kopoli/go-terminal-size"
)

// TerminalDimensions is a simple struct containing current user's terminal width and height.
type TerminalDimensions struct {
	Width  int
	Height int
}

func (r *API) resizeTerminal(execEndpointId string, size tsize.Size) error {
	jsonBodyData := map[string]interface{}{
		"Height": size.Height,
		"Width":  size.Width,
		"id":     execEndpointId,
	}
	body, err := json.Marshal(jsonBodyData)
	if err != nil {
		return err
	}
	req, _ := http.NewRequest("POST", r.formatHttpApiUrl()+"/endpoints/"+strconv.Itoa(r.Endpoint)+"/docker/exec/"+execEndpointId+"/resize?h="+strconv.Itoa(size.Height)+"&w="+strconv.Itoa(size.Width), bytes.NewReader(body))
	_, err = r.makeObjReq(req, true)

	return err
}

// TriggerResize is a simple Resize event trigger.
type TriggerResize struct{}

func (r *API) handleTerminalResize(execInstanceId string) (chan<- TriggerResize, <-chan error, error) {
	sizeListener, err := tsize.NewSizeListener()
	if err != nil {
		// Error creating SizeListener
		return nil, nil, err
	}
	resize := make(chan TriggerResize)
	errChan := make(chan error)

	go func() {
		for {
			select {
			case <-resize:
				size, err := tsize.GetSize()
				if err != nil {
					errChan <- err
				}
				r.resizeTerminal(execInstanceId, size)

			case newSize := <-sizeListener.Change:
				r.resizeTerminal(execInstanceId, newSize)
			}
		}
	}()

	return resize, errChan, nil
}
