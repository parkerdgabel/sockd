package message

import "encoding/gob"

func init() {
	gob.Register(CreateResponse{})
	gob.Register(ListResponse{})
	gob.Register(InspectResponse{})
	gob.Register(LogsResponse{})
}

type ResponsePayload interface{}

type CreateResponse struct {
	Id string `json:"id"`
}

type ListResponse struct {
	Ids []string `json:"ids"`
}

type InspectResponse struct {
	Id     string `json:"id"`
	Status string `json:"status"`
}

type LogsResponse struct {
	Id    string `json:"id"`
	Logs  string `json:"logs"`
	Error string `json:"error"`
}

type ForkResponse struct {
	Id string `json:"id"`
}

type Response struct {
	Success bool
	Message string
	Payload ResponsePayload
}
