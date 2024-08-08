package message

import (
	"encoding/gob"
	"parkerdgabel/sockd/pkg/container"
)

func init() {
	gob.Register(Request{})
	gob.Register(Response{})
	gob.Register(PayloadCreate{})
	gob.Register(PayloadDelete{})
	gob.Register(PayloadStart{})
	gob.Register(PayloadStop{})
	gob.Register(PayloadInspect{})
	gob.Register(PayloadLogs{})
	gob.Register(PayloadFork{})
	gob.Register(PayloadList{})
	gob.Register(PayloadPause{})
	gob.Register(PayloadUnpause{})
	gob.Register(container.Meta{})
}

type Command string

const (
	// CommandCreate is used to create a new container
	CommandCreate Command = "create"
	// CommandDelete is used to delete a container
	CommandDelete Command = "delete"
	// CommandStart is used to start a container
	CommandStart Command = "start"
	// CommandStop is used to stop a container
	CommandStop Command = "stop"
	// CommandList is used to list all containers
	CommandList Command = "list"
	// CommandInspect is used to inspect a container
	CommandInspect Command = "inspect"
	// CommandLogs is used to get logs from a container
	CommandLogs Command = "logs"
	// CommandFork is used to fork a container
	CommandFork Command = "fork"
	// CommandPause is used to pause a container
	CommandPause Command = "pause"
	// CommandUnpause is used to unpause a container
	CommandUnpause Command = "unpause"
)

type RequestPayload interface{}

type PayloadCreate struct {
	BaseImage        string         `json:"base_image"`
	BaseImageVersion string         `json:"base_image_version"`
	Meta             container.Meta `json:"meta"`
	Name             string         `json:"name"`
}

type PayloadDelete struct {
	Id string `json:"id"`
}

type PayloadStart struct {
	Id string `json:"id"`
}

type PayloadStop struct {
	Id string `json:"id"`
}

type PayloadInspect struct {
	Id string `json:"id"`
}

type PayloadLogs struct {
	Id string `json:"id"`
}

type PayloadFork struct {
	Id string `json:"id"`
}

type PayloadList struct{}

type PayloadPause struct {
	Id string `json:"id"`
}

type PayloadUnpause struct {
	Id string `json:"id"`
}

type Request struct {
	Command Command
	Payload RequestPayload
}
