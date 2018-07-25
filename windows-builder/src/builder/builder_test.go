package builder

import (
	"bytes"
	"context"
	"io"
	"os"
	"reflect"
	"testing"
)

var projectID string

func init() {
	projectID = os.Getenv("PROJECT_ID")
}

func TestZipUploadLinux(t *testing.T) {
	//TODO: make this hermetic, so it doesn't rely on /workspace.
	ctx := context.Background()
	client, err := NewGCSClient(ctx)
	if err != nil {
		t.Errorf("Failed to create GCS client: %v", err)
	}
	_, _, err = ZipUploadLinux(ctx, client, projectID)
	if err != nil {
		t.Errorf("Failed to zip and upload dir: %v", err)
	}
}

type fakeClient struct {
	stdout   io.Writer
	stderr   io.Writer
	commands []string
}

func newFakeClient() *fakeClient {
	return &fakeClient{
		stdout:   bytes.NewBuffer([]byte{}),
		stderr:   bytes.NewBuffer([]byte{}),
		commands: []string{},
	}
}

func (f *fakeClient) Run(command string, stdout io.Writer, stderr io.Writer) (int, error) {
	f.commands = append(f.commands, command)
	return 0, nil
}

func TestRunRemoteCommand(t *testing.T) {
	client := newFakeClient()
	_, err := client.Run("Hello world", client.stdout, client.stderr)
	if err != nil {
		t.Errorf("Received error: %+v", err)
	}
}

func TestPullContainer(t *testing.T) {
	client := newFakeClient()
	s := Server{
		Client: client,
	}
	tests := []struct {
		container string
		want      []string
	}{
		{
			container: "gcr.io/test/test",
			want:      []string{"gcloud --quiet auth configure-docker", "docker pull gcr.io/test/test"},
		},
	}

	for _, test := range tests {
		client.commands = []string{}
		err := s.PullContainer(test.container)
		if err != nil {
			t.Errorf("Received error: %v", err)
		}
		if !reflect.DeepEqual(client.commands, test.want) {
			t.Errorf("Received %v, expected %v", client.commands, test.want)
		}

	}
}

func TestDownloadUnzipWindows(t *testing.T) {
	client := newFakeClient()
	s := Server{
		Client: client,
	}
	tests := []struct {
		bucketname string
		filename   string
		want       []string
	}{
		{
			bucketname: "test-bucket",
			filename:   "test-filename.zip",
			want: []string{
				`gsutil cp gs://test-bucket/test-filename.zip C:\test-filename.zip`,
				`cd C:\ & unzip C:\test-filename.zip C:\workspace`,
			},
		},
		{
			bucketname: "test-bucket",
			filename:   "folder/filename.zip",
			want: []string{
				`gsutil cp gs://test-bucket/folder/filename.zip C:\filename.zip`,
				`cd C:\ & unzip C:\filename.zip C:\workspace`,
			},
		},
		{
			bucketname: "test-bucket",
			filename:   "folder1/folder2/filename.zip",
			want: []string{
				`gsutil cp gs://test-bucket/folder1/folder2/filename.zip C:\filename.zip`,
				`cd C:\ & unzip C:\filename.zip C:\workspace`,
			},
		},
	}

	for _, test := range tests {
		client.commands = []string{}
		err := s.DownloadUnzipWindows(test.bucketname, test.filename)
		if err != nil {
			t.Errorf("Received error: %v", err)
		}
		if !reflect.DeepEqual(client.commands, test.want) {
			t.Errorf("\nReceived %v\nExpected %v", client.commands, test.want)
		}

	}
}

func TestRunContainer(t *testing.T) {
	client := newFakeClient()
	s := Server{
		Client: client,
	}
	tests := []struct {
		container string
		want      []string
	}{
		{
			container: "gcr.io/test/test",
			want:      []string{`docker run -it --mount C:\workspace gcr.io/test/test`},
		},
	}

	for _, test := range tests {
		client.commands = []string{}
		err := s.RunContainer(test.container)
		if err != nil {
			t.Errorf("Received error: %v", err)
		}
		if !reflect.DeepEqual(client.commands, test.want) {
			t.Errorf("Received %v, expected %v", client.commands, test.want)
		}

	}
}

func TestZipUploadWindows(t *testing.T) {
	client := newFakeClient()
	s := Server{
		Client: client,
	}
	tests := []struct {
		bucketname string
		filename   string
		want       []string
	}{
		{
			bucketname: "test-bucket",
			filename:   "test-filename.zip",
			want: []string{
				`cd C:\ & zip C:\test-filename.zip C:\workspace`,
				`gsutil cp C:\test-filename.zip gs://test-bucket/test-filename.zip`,
			},
		},
	}

	for _, test := range tests {
		client.commands = []string{}
		err := s.ZipUploadWindows(test.bucketname, test.filename)
		if err != nil {
			t.Errorf("Received error: %v", err)
		}
		if !reflect.DeepEqual(client.commands, test.want) {
			t.Errorf("\nReceived %v\nExpected %v", client.commands, test.want)
		}

	}
}
