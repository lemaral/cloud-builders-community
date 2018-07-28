package builder

import (
	"bufio"
	"bytes"
	"io"
	"log"

	"github.com/masterzen/winrm"
	"github.com/packer-community/winrmcp/winrmcp"
)

//Remote represents a remote Windows server.
type Remote struct {
	Hostname string
	Username string
	Password string
	Client   Clientlike
}

//Clientlike is something which allows us to execute commands on a remote server.
type Clientlike interface {
	Run(string, io.Writer, io.Writer) (int, error)
}

func (r *Remote) getClient() Clientlike {
	if r.Client == nil {
		endpoint := winrm.NewEndpoint(r.Hostname, winrmport, true, true, nil, nil, nil, 0)
		client, err := winrm.NewClient(endpoint, r.Username, r.Password)
		if err != nil {
			log.Printf("Error opening client connection to Windows host: %v", err)
			return nil
		}
		r.Client = client
	}
	return r.Client
}

//RunRemoteCommand runs a command on a Windows server.
func (r *Remote) RunRemoteCommand(stdout io.Writer, cmd string) error {
	var stderr bytes.Buffer
	client := r.getClient()
	exit, err := client.Run(cmd, stdout, &stderr)
	if err != nil {
		log.Printf("Client experienced error running command: %+v", err)
		return err
	}
	if exit != 0 {
		log.Printf("Client received non-zero error code %d, stderr %s", exit, stderr.String())
	}
	return nil
}

//CopyWorkspace copies a workspace from GCB to the remote server.
func (r *Remote) CopyWorkspace(direction string) error {
	//Create workspace dir on server if it doesn't already exist.
	var buf bytes.Buffer
	err := r.RunRemoteCommand(bufio.NewWriter(&buf), "mkdir C:\\workspace")
	if err != nil {
		log.Printf("Failed to create directory C:\\workspace: %+v", err)
		return err
	}

	//Copy files.
	winrmcp, err := winrmcp.New(r.Hostname, &winrmcp.Config{
		Auth: winrmcp.Auth{
			User:     r.Username,
			Password: r.Password,
		},
		Https:    true,
		Insecure: true,
	})
	if err != nil {
		log.Printf("Failed to create winrmcp client: %+v", err)
		return err
	}
	if direction == "to" {
		err = winrmcp.Copy("/workspace/*", "C:\\workspace")
	} else {
		err = winrmcp.Copy("C:\\workspace", "/workspace")
	}
	if err != nil {
		log.Printf("Failed to copy files: %+v", err)
		return err
	}
	return nil
}
