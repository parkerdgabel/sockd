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
	Id string
}

type ListResponse struct {
	Ids []string
}

type InspectResponse struct {
	Id     string
	Status string
}

type LogsResponse struct {
	Id    string
	Logs  string
	Error string
}

type ForkResponse struct {
	Id string
}

type Response struct {
	Success bool
	Message string
	Payload ResponsePayload
}
