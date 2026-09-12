# Azure Container Apps deployment

The production app is hosted at:

https://camplist.yellowcliff-1d686ee3.westeurope.azurecontainerapps.io

Google's OAuth client must allow this exact redirect URI:

```text
https://camplist.yellowcliff-1d686ee3.westeurope.azurecontainerapps.io/auth/callback
```

Add it alongside the existing localhost callback in Google Cloud Console → Google
Auth Platform → Clients. The existing OAuth client credentials are reused.
The app's `REDIRECT_URL` is configured automatically by `infra/app.bicep`.

## Resources

| Resource | Configuration |
| --- | --- |
| Subscription | `komit portal` (`986c176d-46ed-4e07-9016-cd3c56ab3f03`) |
| Resource group | `rg-camplist-prod`, West Europe |
| Container Apps environment | `env-camplist-prod`, Consumption only |
| App | `camplist`, 0.25 vCPU / 0.5 GiB, minimum 0 / maximum 1 replica |
| Registry | `camplistjefeoxrtdy32k.azurecr.io`, Basic, admin credentials disabled |
| Runtime identity | `id-camplist-runtime`, AcrPull on this registry |
| Deployment identity | `id-camplist-github`, federated GitHub OIDC |
| Existing database | `gowithme`, database `dev`, container `packing_list`; no new Cosmos resource |

The app scales to zero when idle, with cold starts on subsequent traffic. The
Basic registry has a standing charge even when the app is idle. Cosmos remains
billed under its existing configuration. Consumption free grants are shared
across the subscription, so this setup is not a guarantee of a zero bill.
[Container Apps pricing](https://azure.microsoft.com/en-us/pricing/details/container-apps/),
[Container Registry pricing](https://azure.microsoft.com/en-us/pricing/details/container-registry/).

No Log Analytics workspace, dedicated compute profile, NAT gateway, or private
network was added. Live log streaming is available; retained application logs are
not configured. `/healthz` checks the running process without making Cosmos calls.

## Continuous delivery

`.github/workflows/deploy.yml` runs on pull requests, pushes to `main`, and manual
runs. It checks generated templates, runs Go tests with the race detector, vet,
build, offline JavaScript checks/tests, Bicep compilation, and a container build.
Only successful `main` runs can deploy to the GitHub `production` environment.

The deployment job signs into Azure using OIDC, pushes a commit-SHA-tagged image,
updates the existing app, and checks `/healthz` and `/login` over HTTPS. It keeps
application secrets and resource sizing unchanged. Deployments are serialized.
Actions and container base images are pinned; update those pins intentionally.

GitHub environment variables:

- `AZURE_CLIENT_ID`, `AZURE_TENANT_ID`, `AZURE_SUBSCRIPTION_ID`
- `AZURE_RESOURCE_GROUP`, `AZURE_CONTAINER_APP`
- `AZURE_REGISTRY_NAME`, `AZURE_REGISTRY_SERVER`

The deployment identity has AcrPush on the dedicated registry, Container Apps
Contributor on the dedicated resource group, and Managed Identity Operator on the
runtime identity. Federation trusts only
`repo:Baqiwaqi/camplist:environment:production`; the GitHub environment permits
only `main`. No Azure client secret is stored in GitHub.

## Secrets and packaging

The Cosmos key, Google client secret, and independently generated production
session/CSRF keys are ACA secrets referenced by environment variables. Database
and Google settings were loaded from the local `.env` during initial bootstrap.
The production cookie keys are distinct from local development keys and survive
routine deployments. Rotating either cookie key invalidates existing sessions or
CSRF tokens.

The Docker build context uses an allowlist that excludes `.env`, Git history,
local validation fixtures, and deployment parameters. The final distroless image
contains the Go executable, static assets, and CA certificates. It runs as
non-root and listens on port 3000; ACA terminates HTTPS.

## Recreate infrastructure

Select the intended subscription before running these commands. The Bicep files
are the infrastructure source of truth. Foundation deployment is incremental:

```sh
az account set --subscription 986c176d-46ed-4e07-9016-cd3c56ab3f03
az group create --name rg-camplist-prod --location westeurope
az deployment group create --name camplist-foundation \
  --resource-group rg-camplist-prod --template-file infra/foundation.bicep \
  --query properties.outputs
```

Build and push an initial image before creating the app. For the first app
creation only, generate a protected parameter file without printing secret values:

```sh
go run infra/prepare-parameters.go /tmp/camplist-app-parameters.json \
  camplistjefeoxrtdy32k camplistjefeoxrtdy32k.azurecr.io/camplist:YOUR_IMAGE_TAG
az deployment group create --name camplist-app --resource-group rg-camplist-prod \
  --template-file infra/app.bicep --parameters @/tmp/camplist-app-parameters.json \
  --query properties.outputs
rm /tmp/camplist-app-parameters.json
```

The helper refuses to overwrite an existing file and generates new cookie keys.
Do not use it for routine releases. When changing infrastructure for an existing
app, preserve the existing secret values in your secure parameter input. Do not
commit secret parameter files. The workflow only updates the image.

## Operations

View revisions and live logs:

```sh
az containerapp revision list -g rg-camplist-prod -n camplist -o table
az containerapp logs show -g rg-camplist-prod -n camplist --type console --follow
```

Roll back by deploying a previously successful image tag:

```sh
az containerapp update -g rg-camplist-prod -n camplist \
  --image camplistjefeoxrtdy32k.azurecr.io/camplist:PREVIOUS_COMMIT_SHA
```

Keep useful rollback images, then periodically remove older unused registry
versions to control storage growth. Changing the maximum replica count raises
the potential compute spend. See [deployment validation](deployment-validation.md)
for browser and database checks already performed and their limits.
