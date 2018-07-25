package builder

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha1"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	"golang.org/x/oauth2/google"
	compute "google.golang.org/api/compute/v1"
)

const (
	zone           = "us-central1-f"
	instanceName   = "windows-builder"
	computeTimeout = 120 * time.Second
)

//NewServer creates a new Windows server on GCE.
func NewServer(ctx context.Context, projectID string) Server {
	log.Print("Starting Windows VM server")
	svc, err := GCEService(ctx)
	if err != nil {
		log.Fatalf("Failed to start GCE service: %v", err)
	}
	inst, err := StartWindowsVM(ctx, svc, projectID)
	if err != nil {
		log.Fatalf("Failed to start Windows VM: %v", err)
	}
	//TODO: get host IP from inst, set the username, reset password
	hostname := ""
	username := ""
	password, err := ResetWindowsPassword(projectID, svc, inst, username)

	//Set firewall rule.
	err = SetFirewallRule(ctx, svc, projectID)
	if err != nil {
		log.Fatalf("Failed to set ingress firewall rule: %v", err)
	}

	return Server{
		Hostname: hostname,
		Username: username,
		Password: password,
	}
}

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

//SetFirewallRule allows ingress on WinRM port.
func SetFirewallRule(ctx context.Context, service *compute.Service, projectID string) error {
	firewallRule := &compute.Firewall{
		Allowed: []*compute.FirewallAllowed{
			&compute.FirewallAllowed{
				IPProtocol: "tcp",
				Ports:      []string{"5986"},
			},
		},
		Direction:    "INGRESS",
		Name:         "allow-winrm-ingress",
		SourceRanges: []string{"0.0.0.0/0"},
	}
	_, err := service.Firewalls.Insert(projectID, firewallRule).Do()
	if err != nil {
		log.Printf("Error setting firewall rule: %v", err)
		return err
	}
	return nil
}

//WindowsPasswordConfig stores metadata to be sent to GCE.
type WindowsPasswordConfig struct {
	key      *rsa.PrivateKey
	password string
	UserName string    `json:"userName"`
	Modulus  string    `json:"modulus"`
	Exponent string    `json:"exponent"`
	Email    string    `json:"email"`
	ExpireOn time.Time `json:"expireOn"`
}

//WindowsPasswordResponse stores data received from GCE.
type WindowsPasswordResponse struct {
	UserName          string `json:"userName"`
	PasswordFound     bool   `json:"passwordFound"`
	EncryptedPassword string `json:"encryptedPassword"`
	Modulus           string `json:"modulus"`
	Exponent          string `json:"exponent"`
	ErrorMessage      string `json:"errorMessage"`
}

//ResetWindowsPassword securely resets the admin Windows password.
//See https://cloud.google.com/compute/docs/instances/windows/automate-pw-generation
func ResetWindowsPassword(projectID string, service *compute.Service, inst *compute.Instance, username string) (string, error) {
	//Create random key and encode
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		log.Printf("Failed to generate random RSA key: %v", err)
		return "", err
	}
	buf := make([]byte, 4)
	binary.BigEndian.PutUint32(buf, uint32(key.E))
	wpc := WindowsPasswordConfig{
		key:      key,
		UserName: username,
		Modulus:  base64.StdEncoding.EncodeToString(key.N.Bytes()),
		Exponent: base64.StdEncoding.EncodeToString(buf[1:]),
		Email:    "nobody@nowhere.com",
		ExpireOn: time.Now().Add(time.Minute * 5),
	}
	data, err := json.Marshal(wpc)
	dstring := string(data)
	if err != nil {
		log.Printf("Failed to marshal JSON: %v", err)
		return "", err
	}

	//Write key to instance metadata and wait for op to complete
	log.Print("Writing Windows instance metadata for password reset.")
	inst.Metadata.Items = append(inst.Metadata.Items, &compute.MetadataItems{
		Key:   "windows-keys",
		Value: &dstring,
	})
	op, err := service.Instances.SetMetadata(projectID, zone, instanceName, &compute.Metadata{
		Fingerprint: inst.Metadata.Fingerprint,
		Items:       inst.Metadata.Items,
	}).Do()
	if err != nil {
		log.Printf("Failed to set instance metadata: %v", err)
		return "", err
	}
	err = WaitForComputeOperation(service, projectID, zone, op)
	if err != nil {
		log.Printf("Compute operation timed out")
		return "", err
	}

	//Read and decode password
	log.Print("Waiting for Windows password response.")
	timeout := time.Now().Add(time.Minute * 3)
	hash := sha1.New()
	random := rand.Reader
	for time.Now().Before(timeout) {
		output, err := service.Instances.GetSerialPortOutput(projectID, zone, instanceName).Port(4).Do()
		if err != nil {
			log.Printf("Unable to get serial port output: %v", err)
			return "", err
		}
		responses := strings.Split(output.Contents, "\n")
		for _, response := range responses {
			var wpr WindowsPasswordResponse
			if err := json.Unmarshal([]byte(response), &wpr); err != nil {
				continue
			}
			if wpr.Modulus == wpc.Modulus {
				decodedPassword, err := base64.StdEncoding.DecodeString(wpr.EncryptedPassword)
				if err != nil {
					log.Printf("Cannot Base64 decode password: %v", err)
					return "", err
				}
				password, err := rsa.DecryptOAEP(hash, random, wpc.key, decodedPassword, nil)
				if err != nil {
					log.Printf("Cannot decrypt password response: %v", err)
					return "", err
				}
				return string(password), nil
			}
		}
		time.Sleep(2 * time.Second)
	}
	err = errors.New("Could not retrieve password before timeout")
	return "", err
}

//WaitForComputeOperation waits for a compute operation
func WaitForComputeOperation(service *compute.Service, projectID string, zone string, op *compute.Operation) error {
	log.Printf("Waiting for operation %+v to complete", op.Name)
	timeout := time.Now().Add(computeTimeout)
	for time.Now().Before(timeout) {
		newop, err := service.ZoneOperations.Get(projectID, zone, op.Name).Do()
		if err != nil {
			log.Printf("Failed to update operation status: %v", err)
			return err
		}
		if newop.Status == "DONE" {
			return nil
		}
		time.Sleep(1 * time.Second)
	}
	err := fmt.Errorf("Compute operation %s timed out", op.Name)
	return err
}
