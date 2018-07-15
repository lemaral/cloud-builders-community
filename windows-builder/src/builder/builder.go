package builder

import (
	"archive/zip"
	"bytes"
	"context"
	"io"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"cloud.google.com/go/storage"
	"golang.org/x/oauth2/google"
	compute "google.golang.org/api/compute/v1"
)

const (
	prefix       = "https://www.googleapis.com/compute/v1/projects/"
	imageURL     = prefix + "windows-cloud/global/images/windows-server-1709-dc-core-for-containers-v20180508"
	zone         = "us-central1-f"
	instanceName = "windows-builder"
)

//GCEService returns a Compute Engine service.
func GCEService(ctx context.Context) (*compute.Service, error) {
	client, err := google.DefaultClient(ctx, compute.ComputeScope)
	if err != nil {
		log.Printf("Failed to create Google Default Client: %v", err)
		return nil, err
	}
	service, err := compute.New(client)
	if err != nil {
		log.Printf("Failed to create Compute Service: %v", err)
		return nil, err
	}

	return service, nil
}

//StartWindowsVM starts a Windows VM on GCE and returns host, username, password.
func StartWindowsVM(ctx context.Context, service *compute.Service, projectID string) (*compute.Instance, error) {
	startupCmd := `winrm set winrm/config/Service/Auth @{Basic="true”} & winrm set winrm/config/Service @{AllowUnencrypted="true”}`
	instance := &compute.Instance{
		Name:        instanceName,
		MachineType: prefix + projectID + "/zones/" + zone + "/machineTypes/n1-standard-1",
		Disks: []*compute.AttachedDisk{
			{
				AutoDelete: true,
				Boot:       true,
				Type:       "PERSISTENT",
				InitializeParams: &compute.AttachedDiskInitializeParams{
					DiskName:    "windows-pd",
					SourceImage: imageURL,
				},
			},
		},
		Metadata: &compute.Metadata{
			Items: []*compute.MetadataItems{
				&compute.MetadataItems{
					Key:   "windows-startup-script-bat",
					Value: &startupCmd,
				},
			},
		},
		NetworkInterfaces: []*compute.NetworkInterface{
			&compute.NetworkInterface{
				AccessConfigs: []*compute.AccessConfig{
					&compute.AccessConfig{
						Type: "ONE_TO_ONE_NAT",
						Name: "External NAT",
					},
				},
				Network: prefix + projectID + "/global/networks/default",
			},
		},
		ServiceAccounts: []*compute.ServiceAccount{
			{
				Email: "default",
				Scopes: []string{
					compute.DevstorageFullControlScope,
					compute.ComputeScope,
				},
			},
		},
	}

	op, err := service.Instances.Insert(projectID, zone, instance).Do()
	if err != nil {
		log.Printf("GCE Instances insert call failed: %v", err)
		return nil, err
	}
	etag := op.Header.Get("Etag")
	inst, err := service.Instances.Get(projectID, zone, instanceName).IfNoneMatch(etag).Do()
	if err != nil {
		log.Printf("Could not get GCE Instance details after creation: %v", err)
		return nil, err
	}
	log.Printf("Successfully created instance %#v", inst.Name)

	return inst, nil
}

//RefreshWindowsVM refreshes latest info from GCE on a VM.
func RefreshWindowsVM(ctx context.Context, service *compute.Service, projectID string) (*compute.Instance, error) {
	inst, err := service.Instances.Get(projectID, zone, instanceName).Do()
	if err != nil {
		log.Printf("Could not refresh instance: %v", err)
		return nil, err
	}
	return inst, nil
}

//StopWindowsVM stops a Windows VM on GCE.
func StopWindowsVM(ctx context.Context, service *compute.Service, projectID string) error {
	_, err := service.Instances.Delete(projectID, zone, instanceName).Do()
	if err != nil {
		log.Printf("Could not delete instance: %v", err)
		return err
	}
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

//ZipUploadDir zips and uploads a dir to GCS.
func ZipUploadDir(ctx context.Context, client *storage.Client, projectID string) (string, string, error) {
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

//DownloadUnzipDir downloads and unzips a dir from GCS.
func DownloadUnzipDir() {
}
