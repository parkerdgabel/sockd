package main

import (
	"fmt"
	"log"
	"net"
	"os"
	"parkerdgabel/sockd/pkg/client"
	"parkerdgabel/sockd/pkg/container"
	"parkerdgabel/sockd/pkg/message"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	connectionType string
	connectionAddr string
)

var cfgFile string
var socketPath string

var rootCmd = &cobra.Command{
	Use:   "sockctl",
	Short: "sockctl is a CLI to interact with the sockd daemon",
}

func init() {
	cobra.OnInitialize(initConfig)
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.sockctl.yaml)")
	rootCmd.PersistentFlags().StringVar(&socketPath, "socket", "/var/run/sockd.sock", "Unix socket path")
	rootCmd.PersistentFlags().StringVar(&connectionType, "connection-type", "unix", "Connection type (unix or tcp)")
	rootCmd.PersistentFlags().StringVar(&connectionAddr, "connection-addr", socketPath, "Connection address (socket path for unix or host:port for tcp)")
}

func initConfig() {
	if cfgFile != "" {
		viper.SetConfigFile(cfgFile)
	} else {
		home, err := os.UserHomeDir()
		cobra.CheckErr(err)

		viper.AddConfigPath(home)
		viper.SetConfigName(".sockctl")
	}

	viper.BindPFlag("connection-type", rootCmd.PersistentFlags().Lookup("connection-type"))
	viper.BindPFlag("connection-addr", rootCmd.PersistentFlags().Lookup("connection-addr"))

	viper.AutomaticEnv()

	if err := viper.ReadInConfig(); err == nil {
		fmt.Println("Using config file:", viper.ConfigFileUsed())
	}
}

func main() {
	rootCmd.AddCommand(
		newCreateCmd(),
		newDeleteCmd(),
		newStartCmd(),
		newStopCmd(),
		newListCmd(),
		newInspectCmd(),
		newLogsCmd(),
		newForkCmd(),
		newPauseCmd(),
		newUnpauseCmd(),
	)
	cobra.CheckErr(rootCmd.Execute())
}

func newClient() *client.Client {
	connType := viper.GetString("connection-type")
	connAddr := viper.GetString("connection-addr")

	conn, err := net.Dial(connType, connAddr)
	if err != nil {
		log.Fatalf("Failed to connect to socket: %v", err)
	}
	return client.NewClient(client.WithConn(conn))
}
func newCreateCmd() *cobra.Command {
	var meta container.Meta
	var name string

	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a new container",
		Run: func(cmd *cobra.Command, args []string) {
			c := newClient()
			defer c.Close()
			res, err := c.Create(meta, name)
			if err != nil {
				log.Fatalf("Failed to create container: %v", err)
			}
			fmt.Printf("Created container: %s\n", res.Payload.(message.CreateResponse).Id)
		},
	}

	cmd.Flags().StringVar(&name, "name", "", "Container name")
	cmd.Flags().StringVar(&meta.ParentID, "parent-id", "", "Parent container ID")
	cmd.Flags().BoolVar(&meta.IsLeaf, "is-leaf", false, "Is this a leaf container")
	cmd.Flags().StringSliceVar(&meta.Installs, "installs", nil, "List of installs")
	cmd.Flags().StringSliceVar(&meta.Imports, "imports", nil, "List of imports")
	cmd.Flags().StringVar((*string)(&meta.Runtime), "runtime", "", "Container runtime")
	cmd.Flags().IntVar(&meta.MemLimitMB, "mem-limit-mb", 0, "Memory limit in MB")
	cmd.Flags().IntVar(&meta.CPUPercent, "cpu-percent", 0, "CPU percentage limit")
	cmd.Flags().StringVar(&meta.BaseImageName, "base-image-name", "", "Base image name")
	cmd.Flags().StringVar(&meta.BaseImageVersion, "base-image-version", "latest", "Base image version")

	cmd.MarkFlagRequired("runtime")
	cmd.MarkFlagRequired("base-image-name")
	cmd.MarkFlagRequired("name")

	return cmd
}

func newDeleteCmd() *cobra.Command {
	var id string

	cmd := &cobra.Command{
		Use:   "delete",
		Short: "Delete a container",
		Run: func(cmd *cobra.Command, args []string) {
			c := newClient()
			defer c.Close()
			_, err := c.Delete(id)
			if err != nil {
				log.Fatalf("Failed to delete container: %v", err)
			}
			fmt.Printf("Deleted container: %s\n", id)
		},
	}

	cmd.Flags().StringVar(&id, "id", "", "Container ID")
	cmd.MarkFlagRequired("id")

	return cmd
}

func newStartCmd() *cobra.Command {
	var id string

	cmd := &cobra.Command{
		Use:   "start",
		Short: "Start a container",
		Run: func(cmd *cobra.Command, args []string) {
			c := newClient()
			defer c.Close()
			_, err := c.Start(id)
			if err != nil {
				log.Fatalf("Failed to start container: %v", err)
			}
			fmt.Printf("Started container: %s\n", id)
		},
	}

	cmd.Flags().StringVar(&id, "id", "", "Container ID")
	cmd.MarkFlagRequired("id")

	return cmd
}

func newStopCmd() *cobra.Command {
	var id string

	cmd := &cobra.Command{
		Use:   "stop",
		Short: "Stop a container",
		Run: func(cmd *cobra.Command, args []string) {
			c := newClient()
			defer c.Close()
			_, err := c.Stop(id)
			if err != nil {
				log.Fatalf("Failed to stop container: %v", err)
			}
			fmt.Printf("Stopped container: %s\n", id)
		},
	}

	cmd.Flags().StringVar(&id, "id", "", "Container ID")
	cmd.MarkFlagRequired("id")

	return cmd
}

func newListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List all containers",
		Run: func(cmd *cobra.Command, args []string) {
			c := newClient()
			defer c.Close()
			res, err := c.List()
			if err != nil {
				log.Fatalf("Failed to list containers: %v", err)
			}
			fmt.Printf("Containers: %v\n", res.Payload.(message.ListResponse).Ids)
		},
	}

	return cmd
}

func newInspectCmd() *cobra.Command {
	var id string

	cmd := &cobra.Command{
		Use:   "inspect",
		Short: "Inspect a container",
		Run: func(cmd *cobra.Command, args []string) {
			c := newClient()
			defer c.Close()
			res, err := c.Inspect(id)
			if err != nil {
				log.Fatalf("Failed to inspect container: %v", err)
			}
			fmt.Printf("Container details: %v\n", res.Payload)
		},
	}

	cmd.Flags().StringVar(&id, "id", "", "Container ID")
	cmd.MarkFlagRequired("id")

	return cmd
}

func newLogsCmd() *cobra.Command {
	var id string

	cmd := &cobra.Command{
		Use:   "logs",
		Short: "Get logs of a container",
		Run: func(cmd *cobra.Command, args []string) {
			c := newClient()
			defer c.Close()
			res, err := c.Logs(id)
			if err != nil {
				log.Fatalf("Failed to get logs: %v", err)
			}
			fmt.Printf("Logs: %v\n", res.Payload)
		},
	}

	cmd.Flags().StringVar(&id, "id", "", "Container ID")
	cmd.MarkFlagRequired("id")

	return cmd
}

func newForkCmd() *cobra.Command {
	var id string

	cmd := &cobra.Command{
		Use:   "fork",
		Short: "Fork a container",
		Run: func(cmd *cobra.Command, args []string) {
			c := newClient()
			defer c.Close()
			_, err := c.Fork(id)
			if err != nil {
				log.Fatalf("Failed to fork container: %v", err)
			}
			fmt.Printf("Forked container: %s\n", id)
		},
	}

	cmd.Flags().StringVar(&id, "id", "", "Container ID")
	cmd.MarkFlagRequired("id")

	return cmd
}

func newPauseCmd() *cobra.Command {
	var id string

	cmd := &cobra.Command{
		Use:   "pause",
		Short: "Pause a container",
		Run: func(cmd *cobra.Command, args []string) {
			c := newClient()
			defer c.Close()
			_, err := c.Pause(id)
			if err != nil {
				log.Fatalf("Failed to pause container: %v", err)
			}
			fmt.Printf("Paused container: %s\n", id)
		},
	}

	cmd.Flags().StringVar(&id, "id", "", "Container ID")
	cmd.MarkFlagRequired("id")

	return cmd
}

func newUnpauseCmd() *cobra.Command {
	var id string

	cmd := &cobra.Command{
		Use:   "unpause",
		Short: "Unpause a container",
		Run: func(cmd *cobra.Command, args []string) {
			c := newClient()
			defer c.Close()
			_, err := c.Unpause(id)
			if err != nil {
				log.Fatalf("Failed to unpause container: %v", err)
			}
			fmt.Printf("Unpaused container: %s\n", id)
		},
	}

	cmd.Flags().StringVar(&id, "id", "", "Container ID")
	cmd.MarkFlagRequired("id")

	return cmd
}
