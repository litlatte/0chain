package interestpoolsc

import (
	"github.com/goccy/go-json"
)

type transferResponses struct {
	Responses []string `json:"responses"`
}

func (tr *transferResponses) addResponse(response string) {
	tr.Responses = append(tr.Responses, response)
}

func (tr *transferResponses) encode() []byte {
	buff, _ := json.Marshal(tr)
	return buff
}

func (tr *transferResponses) decode(input []byte) error {
	err := json.Unmarshal(input, tr)
	return err
}
