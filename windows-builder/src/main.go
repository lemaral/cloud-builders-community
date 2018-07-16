package main

import (
	"context"
	"log"
	"os"

	"./builder"
	compute "google.golang.org/api/compute/v1"
)

var (
	inst      *compute.Instance
	host      string
	username  string
	password  string
	container string
	args      string
	projectID string
)

func main() {
	log.Printf("Starting Windows builder")
	ctx := context.Background()

	//Parse environment variables.
	host = os.Getenv("HOST")
	username = os.Getenv("USERNAME")
	password = os.Getenv("PASSWORD")
	container = os.Getenv("NAME")
	args = os.Getenv("ARGS")
	projectID = os.Getenv("PROJECT_ID")

	//Start a Windows VM on GCE in the background if required.
	if (host == "") || (username == "") || (password == "") {
		log.Print("Starting Windows VM")
		svc, err := builder.GCEService(ctx)
		if err != nil {
			log.Fatalf("Failed to start GCE service: %v", err)
		}
		inst, err = builder.StartWindowsVM(ctx, svc, projectID)
		if err != nil {
			log.Fatalf("Failed to start Windows VM: %v", err)
		}
		//TODO: set host, username
		password, err = builder.ResetWindowsPassword(projectID, svc, inst, username)

		//Set firewall rule.
		err = builder.SetFirewallRule(ctx, svc, projectID)
		if err != nil {
			log.Fatalf("Failed to set ingress firewall rule: %v", err)
		}
	}

	//Sync workspace to GCS.
	client, err := builder.NewGCSClient(ctx)
	if err != nil {
		log.Fatalf("Failed to start GCS client")
	}
	_, _, err = builder.ZipUploadDir(ctx, client, projectID)

	//Connect to Windows host, download from GCS, and unzip.
	//powershell.exe -nologo -noprofile -command "& { Add-Type -A 'System.IO.Compression.FileSystem'; [IO.Compression.ZipFile]::ExtractToDirectory('foo.zip', 'bar'); }"

	//Execute build step container and stream results to stdout.

	//Upload results to GCS.

	//Shutdown Windows VM in background if required.

	//Write GCS results to workspace and exit.
}
