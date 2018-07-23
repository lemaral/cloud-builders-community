# windows-builder

This is an experimental Windows builder.  If this is your first time using Container Builder, follow the [Quickstart for Docker](https://cloud.google.com/container-builder/docs/quickstart-docker) to
get started.

Build steps are run inside [Windows containers](https://docs.microsoft.com/en-us/virtualization/windowscontainers/about/).  The Container Builder workspace, normally available on `/workspace`, is copied to Cloud Storage and from there to the Windows host, before being mounted inside the container at `C:\workspace`.  The container must exit when the build is complete; the workspace is then synced back to Container Builder.

If a Windows host, username and password are provided to the build step, then the build will take place on that host.  Docker must be preinstalled, TCP port 5986 (WinRM over SSL) must be open to the Internet, and the host must support Basic Authentication.  Otherwise, an `n1-standard-1` Windows VM on Compute Engine is started within the project, used for the build, and then shut down afterwards.  It may take several minutes for Windows to boot and pull the container, so frequent builds will benefit from a persistent VM.

## Usage

First, clone this code and build the builder:

```bash
gcloud container builds submit --config=cloudbuild.yaml .
```

Then, if you wish to create Windows VMs on Compute Engine automatically, grant permissions to your Container Builder service account:

```bash
# Setup IAM
export PROJECT=$(gcloud info --format='value(config.project)')
export PROJECT_NUMBER=$(gcloud projects describe $PROJECT --format 'value(projectNumber)')
export CB_SA_EMAIL=$PROJECT_NUMBER@cloudbuild.gserviceaccount.com
gcloud projects add-iam-policy-binding $PROJECT --member=serviceAccount:$CB_SA_EMAIL --role='roles/iam.serviceAccountUser' --role='roles/iam.serviceAccountActor' --role='roles/compute.instanceAdmin.v1'
# Enable Compute API
gcloud services enable compute.googleapis.com
```

Then, use the build step in your `cloudbuild.yaml` as follows, giving the name of your Windows container image in `NAME`.  For example:

```yaml
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

```yaml
steps:
- name: gcr.io/$PROJECT_ID/windows-builder
  env:
    - 'NAME=gcr.io/$PROJECT_ID/build-tool'
    - 'ARGS=-v'
```

## Security

Traffic between Container Builder and the Windows VM is encrypted over SSL, and the initial password reset follows Google best practices.  However, enterprise servers using Kerberos or other more advanced authentication techniques will require code change in the upstream WinRM library.

## Creating a persistent Windows VM manually

To speed up frequent builds, you may wish to create a persistent Windows VM.  To create such a server named `winvm`, execute the following commands:

```bash
gcloud beta compute instances create winvm --image=windows-server-1709-dc-core-for-containers-v20180508 --image-project=windows-cloud
gcloud compute firewall-rules create allow-winrm --direction=INGRESS --priority=1000 --network=default --action=ALLOW --rules=tcp:5986 --source-ranges=0.0.0.0/0
gcloud --quiet beta compute reset-windows-password winvm
```

`gcloud` will then print the IP address, username, and password for your new VM, which can be used in the configuration above.
