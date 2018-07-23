package main

import (
	"context"
	"log"
	"os"

	"./builder"
)

func main() {
	log.Printf("Starting Windows builder")
	ctx := context.Background()
	var server builder.Server

	//Parse environment variables.
	hostname := os.Getenv("HOST")
	username := os.Getenv("USERNAME")
	password := os.Getenv("PASSWORD")
	container := os.Getenv("NAME")
	args := os.Getenv("ARGS")
	projectID := os.Getenv("PROJECT_ID")

	//Start a Windows VM on GCE in the background if required.
	if (hostname == "") || (username == "") || (password == "") {
		server := builder.NewServer(ctx, projectID)
	} else {
		server := builder.Server{
			Hostname: hostname,
			Username: username,
			Password: password,
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
