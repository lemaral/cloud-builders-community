package builder

import "testing"

func TestCopyWorkspace(t *testing.T) {
	r := Remote{
		Hostname: "35.184.220.144",
		Username: "n_o_franklin",
		Password: ",SJLM|ZZ@p%zjzD",
	}
	err := r.CopyWorkspace("to")
	if err != nil {
		t.Errorf("Error copying workspace: %+v", err)
	}
}
