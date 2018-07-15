package main

import (
	"log"
	"os"
)

func main() {
	log.Printf("Starting Windows builder")
	//Parse environment variables.
	host := os.Getenv("HOST")
	username := os.Getenv("USERNAME")
	password := os.Getenv("PASSWORD")
	container := os.Getenv("NAME")
	args := os.Getenv("ARGS")

	//Start a Windows VM on GCE in the background if required.

	//Sync workspace to GCS.

	//Connect to Windows host and download from GCS.

	//Execute build step container and stream results to stdout.

	//Upload results to GCS.

	//Shutdown Windows VM in background if required.

	//Write GCS results to workspace and exit.
}
