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
	var startVM bool

	//Parse environment variables.
	hostname := os.Getenv("HOST")
	username := os.Getenv("USERNAME")
	password := os.Getenv("PASSWORD")
	container := os.Getenv("NAME")
	//args := os.Getenv("ARGS")
	projectID := os.Getenv("PROJECT_ID")

	//Start a Windows VM on GCE in the background if required.
	if (hostname == "") || (username == "") || (password == "") {
		startVM = true
		server = builder.NewServer(ctx, projectID)
	} else {
		server = builder.Server{
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
	bucketname, filename, err := builder.ZipUploadLinux(ctx, client, projectID)

	//Connect to Windows host, download from GCS, and unzip.
	err = server.PullContainer(container)
	if err != nil {
		log.Fatalf("Failed to pull container: %+v", err)
	}

	err = server.DownloadUnzipWindows(bucketname, filename)
	if err != nil {
		log.Fatalf("Failed to download from GCS to Windows: %+v", err)
	}

	//Execute build step container and stream results to stdout.
	err = server.RunContainer(container)
	if err != nil {
		log.Fatalf("Failed to run Windows container: %+v", err)
	}

	//Upload results to GCS.
	err = server.ZipUploadWindows(bucketname, filename)
	if err != nil {
		log.Fatalf("Failed to zip and upload workspace back to GCS after build: %+v", err)
	}

	//Shutdown Windows VM in background if required.
	if startVM {
		err = builder.StopWindowsVM(ctx, projectID)
	}

	//Write GCS results to workspace and exit.
	err = builder.DownloadUnzipLinux(ctx, client, bucketname, filename)
	if err != nil {
		log.Fatalf("Failed to download and unzip results from GCS: %+v", err)
	}
}
