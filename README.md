# fru Lab

![frulab](./docs/images/frulab-logo.png)

Deploy and monitor the free5GC core network and the free-ran-ue gNB / UE simulator.

## Develop Environment

| DevOpts | Version |
| - | - |
| OS | Ubuntu 25.04 |
| go | 1.26.2 |
| nodejs | v20.20.0 |
| yarn | 1.22.22 |

## Make

| Type | Command |
| - | - |
| Make all | `make` |
| Backend | `make backend` |
| Frontend | `make frontend` |
| Run | `make run` |
| Tidy | `make tidy` |
| Lint | `make lint` |
| Generate frontend openapi | `make openapi` |
| Generate frontend openapi for free5GC webconsole | `make openapi-webconsole` |
| Docker image | `make docker` |

## Install - Docker Compose

fru-lab itself deploys free5gc/gNB/UE by talking to the host's Docker daemon, so its own container needs the host's Docker socket mounted in, on top of the usual config/db volumes.

1. Download the compose files

    ```bash
    git clone https://github.com/free-ran-ue/fru-lab.git
    ```

2. Check then config

    Admin can modify system settings at `./docker/config.yaml`, e.g. the default login credential:

    ```yaml
    username: "admin"
    password: "frulab"
    ```

    For other settings, make sure the change is reflected in `docker-compose.yaml` too. In particular, `deploy.workDir` has to stay the exact same path on the host and inside the container (see the comment on that volume in `docker-compose.yaml`) - fru-lab hands that path to the host's Docker daemon when it deploys free5gc/gNB/UE, so it has to resolve to the same place on both sides.

3. Up the compose

    ```bash
    docker compose up -d
    ```

    The default db is stored at `/var/lib/frulab/db` and deploy state at `/var/lib/frulab/deploy`, both mounted in the compose file.

4. Down the compose

    ```bash
    docker compose down
    ```

## Guide

- [Basic Guide](docs/basic-guide.md) - how to log in and perform the core deploy / monitor workflow.
- [Images Guide](docs/images-guide.md) - how to check, pull, and clear the docker images this app deploys.

## TODO

- Setup Script
