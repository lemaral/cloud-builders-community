package builder

/*
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
		time.Sleep(3 * time.Second)
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
	err = StopWindowsVM(ctx, projectID)
	if err != nil {
		t.Errorf("Failed to stop Windows VM: %v", err)
	}
}

func TestResetWindowsPassword(t *testing.T) {
	ctx := context.Background()
	svc, err := GCEService(ctx)
	if err != nil {
		t.Errorf("Error starting GCE service %v", err)
	}
	user, err := user.Current()
	if err != nil {
		t.Errorf("Error getting current user name %v", err)
	}
	inst, err := StartWindowsVM(ctx, svc, projectID)
	if err != nil {
		t.Errorf("Failed to start Windows VM: %v", err)
	}
	password, err := ResetWindowsPassword(projectID, svc, inst, user.Name)
	log.Printf("Got password %s", password)
	err = StopWindowsVM(ctx, projectID)
	if err != nil {
		t.Errorf("Failed to stop Windows VM: %v", err)
	}
}
*/
