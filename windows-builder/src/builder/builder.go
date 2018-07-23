package builder

import (
	"archive/zip"
	"bytes"
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"cloud.google.com/go/storage"
	"github.com/masterzen/winrm"
)

const (
	prefix    = "https://www.googleapis.com/compute/v1/projects/"
	imageURL  = prefix + "windows-cloud/global/images/windows-server-1709-dc-core-for-containers-v20180508"
	winrmport = 5986
)

//Server represents a remote Windows server.
type Server struct {
	Hostname string
	Username string
	Password string
	Client   Clientlike
}

//Clientlike allows mocking of winrm.Client for test purposes.
type Clientlike interface {
	Run(string, io.Writer, io.Writer) (int, error)
}

func (s *Server) getClient() Clientlike {
	if s.Client == nil {
		endpoint := winrm.NewEndpoint(s.Hostname, winrmport, true, true, nil, nil, nil, 0)
		client, err := winrm.NewClient(endpoint, s.Username, s.Password)
		if err != nil {
			log.Printf("Error opening client connection to Windows host: %v", err)
			return nil
		}
		s.Client = client
	}
	return s.Client
}

//RunRemoteCommand runs a command on a Windows server.
func (s *Server) RunRemoteCommand(stdout io.Writer, cmd string) error {
	var stderr bytes.Buffer
	client := s.getClient()
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

//PullContainer configures GCR and pulls container.
func (s *Server) PullContainer(name string) error {
	cmd := "gcloud --quiet auth configure-docker"
	out := bytes.NewBuffer([]byte{})
	err := s.RunRemoteCommand(out, cmd)
	if err != nil {
		log.Printf("Unable to configure gcloud as Docker credential helper: %s", out)
		return err
	}
	log.Println(out.String())
	cmd = fmt.Sprintf("docker pull %s", name)
	out = bytes.NewBuffer([]byte{})
	err = s.RunRemoteCommand(out, cmd)
	if err != nil {
		log.Printf("Unable to pull container: %s", out)
		return err
	}
	log.Println(out.String())
	return nil
}

//NewGCSClient creates a new GCS client.
func NewGCSClient(ctx context.Context) (*storage.Client, error) {
	client, err := storage.NewClient(ctx)
	if err != nil {
		log.Printf("Failed to create client: %v", err)
		return nil, err
	}
	return client, nil
}

//WriteFileToGCS writes a stream to an existing GCS bucket.
func WriteFileToGCS(ctx context.Context, client *storage.Client, bucketname string, filename string, reader io.Reader) error {
	//TODO: Create bucket if it doesn't exist
	bucket := client.Bucket(bucketname)
	obj := bucket.Object(filename)

	w := obj.NewWriter(ctx)
	if _, err := io.Copy(w, reader); err != nil {
		log.Printf("Failed to write to GCS object: %v", err)
		return err
	}

	if err := w.Close(); err != nil {
		log.Printf("Failed to close GCS object: %v", err)
		return err
	}
	return nil
}

//ReadFileFromGCS reads a file from an existing GCS bucket and returns bytes
func ReadFileFromGCS(ctx context.Context, client *storage.Client, bucketname string, filename string) ([]byte, error) {
	bucket := client.Bucket(bucketname)
	obj := bucket.Object(filename)

	r, err := obj.NewReader(ctx)
	if err != nil {
		log.Printf("Failed to open GCS object: %v", err)
		return nil, err
	}
	defer r.Close()

	bytes, err := ioutil.ReadAll(r)
	if err != nil {
		log.Printf("Failed to read from GCS object: %v", err)
		return nil, err
	}
	return bytes, nil
}

//ZipUploadLinux zips and uploads a dir to GCS.
func ZipUploadLinux(ctx context.Context, client *storage.Client, projectID string) (string, string, error) {
	buf := new(bytes.Buffer)
	w := zip.NewWriter(buf)

	//Step through files in workspace directory
	err := filepath.Walk("/workspace", func(path string, info os.FileInfo, err error) error {
		if err != nil {
			log.Printf("Error accessing file %s: %v", path, err)
			return err
		}

		if !info.IsDir() {
			//Add file to in-memory zip
			filename := strings.Replace(path, "/workspace/", "", 1)
			f, err := w.Create(filename)
			if err != nil {
				log.Printf("Error adding file %s to ZIP", filename)
				return err
			}
			bytes, err := ioutil.ReadFile(path)
			if err != nil {
				log.Printf("Error reading file %s", path)
				return err
			}
			_, err = f.Write(bytes)
			if err != nil {
				log.Printf("Error writing file %s to ZIP", path)
				return err
			}
		}
		return nil
	})
	if err != nil {
		log.Printf("Error walking workspace directory: %v", err)
		return "", "", err
	}
	err = w.Close()
	if err != nil {
		log.Printf("Error closing zip file: %v", err)
		return "", "", err
	}

	//Write ZIP to GCS
	bucketname := "cloudbuild-windows-" + projectID
	timestamp := time.Now().Format(time.RFC3339)
	filename := "cloudbuild-windows-" + timestamp + ".zip"
	log.Printf("Writing file %s to GCS bucket %s", filename, bucketname)
	err = WriteFileToGCS(ctx, client, bucketname, filename, buf)

	return bucketname, filename, nil
}

//DownloadUnzipWindows downloads workspace from GCS and unzips.
func (s *Server) DownloadUnzipWindows(bucketname string, filename string) error {
	//Strip directories from filename
	var stripped string
	fparts := strings.Split(filename, "/")
	l := len(fparts)
	if l > 1 {
		stripped = fparts[l-1]
	} else {
		stripped = filename
	}
	cmd := fmt.Sprintf("gsutil cp gs://%s/%s C:\\%s", bucketname, filename, stripped)
	out := bytes.NewBuffer([]byte{})
	err := s.RunRemoteCommand(out, cmd)
	if err != nil {
		log.Printf("Unable to download workspace zip from GCS: %s", out)
		return err
	}
	log.Println(out.String())
	//TODO: Check this gives the correct behavior on Windows
	cmd = fmt.Sprintf("cd C:\\ & unzip C:\\%s C:\\workspace", stripped)
	out = bytes.NewBuffer([]byte{})
	err = s.RunRemoteCommand(out, cmd)
	if err != nil {
		log.Printf("Unable to unzip on remote Windows machine: %s", out)
		return err
	}
	log.Println(out.String())
	return nil
}

//RunContainer starts the container on the Windows server.
func (s *Server) RunContainer(name string) error {
	//TODO: Check this is the correct Docker mount command.
	cmd := fmt.Sprintf("docker run -it --mount C:\\workspace %s", name)
	out := os.Stdout //redirect output straight to tty
	err := s.RunRemoteCommand(out, cmd)
	if err != nil {
		log.Printf("Unable to run container remotely: %+v", err)
		return err
	}
	return nil
}

//ZipUploadWindows zips the workspace and uploads back to GCS, overwriting the previous one.
func (s *Server) ZipUploadWindows(bucketname string, filename string) error {
	//Strip directories from filename
	var stripped string
	fparts := strings.Split(filename, "/")
	l := len(fparts)
	if l > 1 {
		stripped = fparts[l-1]
	} else {
		stripped = filename
	}
	//TODO: Check this gives the correct behavior on Windows
	cmd := fmt.Sprintf("cd C:\\ & zip C:\\%s C:\\workspace", stripped)
	out := bytes.NewBuffer([]byte{})
	err := s.RunRemoteCommand(out, cmd)
	if err != nil {
		log.Printf("Unable to zip on remote Windows machine: %s", out)
		return err
	}
	log.Println(out.String())

	cmd = fmt.Sprintf("gsutil cp C:\\%s gs://%s/%s", stripped, bucketname, filename)
	out = bytes.NewBuffer([]byte{})
	err = s.RunRemoteCommand(out, cmd)
	if err != nil {
		log.Printf("Unable to upload workspace zip to GCS: %s", out)
		return err
	}
	log.Println(out.String())
	return nil
}
