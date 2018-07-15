package builder

import (
	"context"
	"log"
	"testing"
	"time"
)

const (
	projectID = "cloud-builders-community-test"
)

func TestStartRefreshStopWindowsVM(t *testing.T) {
	ctx := context.Background()
	svc, err := GCEService(ctx)
	if err != nil {
		t.Errorf("Error starting GCE service %v", err)
	}
	inst, err := StartWindowsVM(ctx, svc, projectID)
	if err != nil {
		t.Errorf("Failed to start Windows VM: %v", err)
	}
	log.Printf("Got instance %+v", *inst)
	for {
		time.Sleep(5 * time.Second)
		log.Printf("Refreshing instance %v", inst.Name)
		inst, err = RefreshWindowsVM(ctx, svc, projectID)
		if err != nil {
			t.Errorf("Failed to refresh Windows VM: %v", err)
		}
		log.Printf("Got instance status: %v", inst.Status)
		if inst.Status == "RUNNING" {
			break
		}
	}
	err = StopWindowsVM(ctx, svc, projectID)
	if err != nil {
		t.Errorf("Failed to stop Windows VM: %v", err)
	}
}
