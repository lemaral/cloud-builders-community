package builder

import (
	"context"
	"log"

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
