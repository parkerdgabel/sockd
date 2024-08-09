package main

import (
	"encoding/gob"
	"fmt"
	"log"
	"net"
	"os"
	"parkerdgabel/sockd/internal/manager"
	"parkerdgabel/sockd/pkg/container"
	"parkerdgabel/sockd/pkg/message"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var m *manager.Manager = manager.NewManager()

const socketPath = "/var/run/sockd.sock"

var cfgFile string
var tcpAddr string

var rootCmd = &cobra.Command{
	Use:   "sockd",
	Short: "sockd is a daemon to manage SOCK containers",
	Run: func(cmd *cobra.Command, args []string) {
		startDaemon()
	},
}

func init() {
	cobra.OnInitialize(initConfig)
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.sockd.yaml)")
	rootCmd.PersistentFlags().StringVar(&tcpAddr, "tcp", "", "TCP address to listen on (e.g., :8080)")
}

func initConfig() {
	if cfgFile != "" {
		viper.SetConfigFile(cfgFile)
	} else {
		home, err := os.UserHomeDir()
		cobra.CheckErr(err)

		viper.AddConfigPath(home)
		viper.SetConfigName(".sockd")
	}

	viper.AutomaticEnv()

	if err := viper.ReadInConfig(); err == nil {
		fmt.Println("Using config file:", viper.ConfigFileUsed())
	}
}

func startDaemon() {
	listeners := []net.Listener{}

	// Listen on Unix socket
	unixListener, err := net.Listen("unix", socketPath)
	if err != nil {
		log.Fatalf("Failed to listen on %s: %v", socketPath, err)
	}
	listeners = append(listeners, unixListener)
	fmt.Printf("Listening on Unix socket %s\n", socketPath)

	// Listen on TCP address if provided
	if tcpAddr != "" {
		tcpListener, err := net.Listen("tcp", tcpAddr)
		if err != nil {
			log.Fatalf("Failed to listen on TCP address %s: %v", tcpAddr, err)
		}
		listeners = append(listeners, tcpListener)
		fmt.Printf("Listening on TCP address %s\n", tcpAddr)
	}

	for _, listener := range listeners {
		go func(l net.Listener) {
			for {
				conn, err := l.Accept()
				if err != nil {
					log.Printf("Failed to accept connection: %v", err)
					continue
				}
				go handleConnection(conn)
			}
		}(listener)
	}

	// Block forever
	select {}
}

func handleConnection(conn net.Conn) {
	defer conn.Close()
	// Handle the connection
	// Create a new decoder and encoder
	decoder := gob.NewDecoder(conn)
	encoder := gob.NewEncoder(conn)

	for {
		var msg message.Request
		// Decode the incoming message
		if err := decoder.Decode(&msg); err != nil {
			log.Printf("Failed to decode message: %v", err)
			return
		}

		// Process the message (this is just an example)
		log.Printf("Received command: %s, payload: %s", msg.Command, msg.Payload)
		switch msg.Command {
		case message.CommandCreate:
			payload, ok := msg.Payload.(message.PayloadCreate)
			if !ok {
				log.Printf("Invalid payload type: %T", msg.Payload)
				return
			}
			log.Printf("Creating container: %s", payload.Name)
			c, err := createContainer(payload)
			if err != nil {
				log.Printf("Failed to create container: %v", err)
				response := message.Response{
					Success: false,
					Message: err.Error(),
				}
				if err := encoder.Encode(&response); err != nil {
					log.Printf("Failed to encode response: %v", err)
					return
				}
			}
			response := message.Response{
				Success: true,
				Message: fmt.Sprintf("Created container: %s", c.ID()),
				Payload: message.CreateResponse{
					Id: c.ID(),
				},
			}
			if err := encoder.Encode(&response); err != nil {
				log.Printf("Failed to encode response: %v", err)
				return
			}
		case message.CommandDelete:
			payload, ok := msg.Payload.(message.PayloadDelete)
			if !ok {
				log.Printf("Invalid payload type: %T", msg.Payload)
				return
			}
			log.Printf("Deleting container: %s", payload.Id)
			if err := m.DestroyContainer(payload.Id); err != nil {
				log.Printf("Failed to delete container: %v", err)
				response := message.Response{
					Success: false,
					Message: err.Error(),
				}
				if err := encoder.Encode(&response); err != nil {
					log.Printf("Failed to encode response: %v", err)
					return
				}
			}
			response := message.Response{
				Success: true,
				Message: fmt.Sprintf("Deleted container: %s", payload.Id),
			}
			if err := encoder.Encode(&response); err != nil {
				log.Printf("Failed to encode response: %v", err)
				return
			}
		case message.CommandStart:
			payload, ok := msg.Payload.(message.PayloadStart)
			if !ok {
				log.Printf("Invalid payload type: %T", msg.Payload)
				return
			}
			log.Printf("Starting container: %s", payload.Id)
			if err := startContainer(payload); err != nil {
				log.Printf("Failed to start container: %v", err)
				response := message.Response{
					Success: false,
					Message: err.Error(),
				}
				if err := encoder.Encode(&response); err != nil {
					log.Printf("Failed to encode response: %v", err)
					return
				}
			}
			response := message.Response{
				Success: true,
				Message: fmt.Sprintf("Started container: %s", payload.Id),
			}
			if err := encoder.Encode(&response); err != nil {
				log.Printf("Failed to encode response: %v", err)
				return
			}
		case message.CommandStop:
			payload, ok := msg.Payload.(message.PayloadStop)
			if !ok {
				log.Printf("Invalid payload type: %T", msg.Payload)
				return
			}
			log.Printf("Stopping container: %s", payload.Id)
			if err := m.StopContainer(payload.Id); err != nil {
				log.Printf("Failed to stop container: %v", err)
				response := message.Response{
					Success: false,
					Message: err.Error(),
				}
				if err := encoder.Encode(&response); err != nil {
					log.Printf("Failed to encode response: %v", err)
					return
				}
			}
			response := message.Response{
				Success: true,
				Message: fmt.Sprintf("Stopped container: %s", payload.Id),
			}
			if err := encoder.Encode(&response); err != nil {
				log.Printf("Failed to encode response: %v", err)
				return
			}
		case message.CommandList:
			log.Printf("Listing containers")
			ids := m.ListContainers()
			response := message.Response{
				Success: true,
				Message: fmt.Sprintf("Listed %d containers", len(ids)),
				Payload: message.ListResponse{
					Ids: ids,
				},
			}
			if err := encoder.Encode(&response); err != nil {
				log.Printf("Failed to encode response: %v", err)
				return
			}
		case message.CommandInspect:
			payload, ok := msg.Payload.(message.PayloadInspect)
			if !ok {
				log.Printf("Invalid payload type: %T", msg.Payload)
				return
			}
			log.Printf("Inspecting container: %s", payload.Id)
		case message.CommandLogs:
			payload, ok := msg.Payload.(message.PayloadLogs)
			if !ok {
				log.Printf("Invalid payload type: %T", msg.Payload)
				return
			}
			log.Printf("Getting logs for container: %s", payload.Id)
		case message.CommandFork:
			payload, ok := msg.Payload.(message.PayloadFork)
			if !ok {
				log.Printf("Invalid payload type: %T", msg.Payload)
				return
			}
			log.Printf("Forking container: %s", payload.Id)
			if err := m.ForkContainer(payload.Id); err != nil {
				log.Printf("Failed to fork container: %v", err)
				response := message.Response{
					Success: false,
					Message: err.Error(),
				}
				if err := encoder.Encode(&response); err != nil {
					log.Printf("Failed to encode response: %v", err)
					return
				}
			}
			response := message.Response{
				Success: true,
				Message: fmt.Sprintf("Forked container: %s", payload.Id),
			}
			if err := encoder.Encode(&response); err != nil {
				log.Printf("Failed to encode response: %v", err)
				return
			}
		case message.CommandPause:
			payload, ok := msg.Payload.(message.PayloadPause)
			if !ok {
				log.Printf("Invalid payload type: %T", msg.Payload)
				return
			}
			log.Printf("Pausing container: %s", payload.Id)
			if err := m.PauseContainer(payload.Id); err != nil {
				log.Printf("Failed to pause container: %v", err)
				response := message.Response{
					Success: false,
					Message: err.Error(),
				}
				if err := encoder.Encode(&response); err != nil {
					log.Printf("Failed to encode response: %v", err)
					return
				}
			}
			response := message.Response{
				Success: true,
				Message: fmt.Sprintf("Paused container: %s", payload.Id),
			}
			if err := encoder.Encode(&response); err != nil {
				log.Printf("Failed to encode response: %v", err)
				return
			}
		case message.CommandUnpause:
			payload, ok := msg.Payload.(message.PayloadUnpause)
			if !ok {
				log.Printf("Invalid payload type: %T", msg.Payload)
				return
			}
			log.Printf("Unpausing container: %s", payload.Id)
			if err := m.UnpauseContainer(payload.Id); err != nil {
				log.Printf("Failed to unpause container: %v", err)
				response := message.Response{
					Success: false,
					Message: err.Error(),
				}
				if err := encoder.Encode(&response); err != nil {
					log.Printf("Failed to encode response: %v", err)
					return
				}
			}
			response := message.Response{
				Success: true,
				Message: fmt.Sprintf("Unpaused container: %s", payload.Id),
			}
			if err := encoder.Encode(&response); err != nil {
				log.Printf("Failed to encode response: %v", err)
				return
			}
		default:
			log.Printf("Unknown command: %s", msg.Command)
		}
	}
}

func createContainer(payload message.PayloadCreate) (*container.Container, error) {
	c, err := m.CreateContainer(&payload.Meta, payload.Name)
	if err != nil {
		return nil, err
	}
	return c, nil
}

func startContainer(payload message.PayloadStart) error {
	if err := m.StartContainer(payload.Id); err != nil {
		return err
	}
	return nil
}

func main() {
	cobra.CheckErr(rootCmd.Execute())
}
