## pv_backend

`pv_backend` is the main API backend of Postverta. It exposes a couple of HTTP
endpoints for both external and internal uses.

Postverta is designed as a monolithic service as there has not been enough
motivation to split the service into multiple microservices.

## Usage

`pv_backend` requires the following services and resources to function:

- A `mongodb` database to store all meta data.
- A number of compute hosts to run the development containers. Each host must
  have the `pv_agent` daemon running. Please refer to the `pv_agent`
  repository.
- A docker image on [Docker Hub](https://hub.docker.com/) for the container
  base image. Refer to the `base_image` repository.
- A file directory for application logs. It is recommended to use mounted
  remote file systems (such as Azure File) as there can be quite a lot of log
  files.
- An Azure storage account to store the workspace image blobs. Please refer to
  `pv_exec` and `lazytree` repositories for details.
- An [Auth0](https://auth0.com) for user authentication. From the backend
  perspective, it only needs the JWT secret to verify the access token of the
  incoming requests.
- A [Cloudinary](https://cloudinary.com/) account for image processing.
- (Optional) a [Segment](https://segment.com) account to track app accesses.
- (Optional) a TLS certificate for HTTPS.

## Configuration

Most configurations can be found either in `main.go` or `config/config.go`.
The complied binary doesn't take any command line parameter.
