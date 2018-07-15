# windows-builder

This is an experimental Windows builder.  If this is your first time using Container Builder, follow the [Quickstart for Docker](https://cloud.google.com/container-builder/docs/quickstart-docker) to
get started.

Build steps are run inside [Windows containers](https://docs.microsoft.com/en-us/virtualization/windowscontainers/about/).  The Container Builder workspace, normally mounted on `/workspace`, is synced to the Windows host and mounted as `C:\workspace`.  The container must exit when the build is complete; the workspace is then synced back to Container Builder.

If a Windows host, username and password are provided to the build step, then the build will take place on that host.  Docker must be preinstalled, TCP port 5986 must be open to the Internet, and the host must support Basic Authentication.  Otherwise, an `n1-standard-1` Windows VM on Compute Engine is started within the project, used for the build, and then shut down afterwards.  It can take many seconds for Windows to boot and pull the container, so frequent builds will benefit from a persistent VM.

## Usage

First, clone this code, create a GCS bucket, and build the builder:

```
gsutil mb gs://cloudbuild-windows-$PROJECT
gcloud container builds submit --config=cloudbuild.yaml .
```

Then, if you wish to create Windows VMs on Compute Engine automatically, grant permissions to your Container Builder service account:

```
# Setup IAM
export PROJECT=$(gcloud info --format='value(config.project)')
export PROJECT_NUMBER=$(gcloud projects describe $PROJECT --format 'value(projectNumber)')
export CB_SA_EMAIL=$PROJECT_NUMBER@cloudbuild.gserviceaccount.com
gcloud projects add-iam-policy-binding $PROJECT --member=serviceAccount:$CB_SA_EMAIL --role='roles/iam.serviceAccountUser' --role='roles/iam.serviceAccountActor' --role='roles/compute.instanceAdmin.v1'
# Enable Compute API
gcloud services enable compute.googleapis.com
```

Then, use the build step as follows:

```
steps:
- name: gcr.io/$PROJECT_ID/windows-builder
  env:
    - 'HOST=my-server.domain.com'
    - 'USERNAME=johnsmith'
    - 'PASSWORD=53cret!'
    - 'NAME=gcr.io/$PROJECT_ID/build-tool'
    - 'ARGS=-v'
```

Or, to start a Windows VM automatically:

```
steps:
- name: gcr.io/$PROJECT_ID/windows-builder
  env:
    - 'NAME=gcr.io/$PROJECT_ID/build-tool'
    - 'ARGS=-v'
```
