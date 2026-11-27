# fru Lab

![frulab](./web/frontend/src/assets/frulab-logo.png)

fru-lab is a GUI-driven 5G deployment simulator. From the browser it deploys the free5GC core network and the free-ran-ue gNB / UE simulator as containers on the host's Docker daemon (one UE container per subscriber), manages subscribers through the free5GC webconsole, and shows status, logs, images and a terminal into every UE.

It is a Go backend and a React frontend in one binary.

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
| Make all (backend, frontend) | `make` |
| Backend | `make backend` |
| Frontend | `make frontend` |
| Run | `make run` |
| Test | `make test` |
| Tidy | `make tidy` |
| Lint | `make lint` |
| Clean | `make clean` |
| Generate frontend openapi | `make openapi` |
| Generate frontend openapi for free5GC webconsole | `make openapi-webconsole` |
| Docker image | `make docker` |

`make run` starts fru-lab with `./config.yaml` on port `8888`. Deploying free5GC/gNB/UE needs Docker on the host, reachable by the user running fru-lab.

## Install - Docker Compose

The compose file runs the `fru-lab` container. fru-lab deploys free5GC/gNB/UE by talking to the host's Docker daemon, so its container needs the host's Docker socket mounted in, on top of the usual config/db volumes.

1. Download the compose files

    ```bash
    git clone https://github.com/free-ran-ue/fru-lab.git
    cd fru-lab/docker
    ```

2. Check the config

    Admin can modify system settings in `config.yaml` (in `docker/`), e.g. the default login credential:

    ```yaml
    username: "admin"
    password: "frulab"
    ```

    For other settings, make sure the change is reflected in `docker-compose.yaml` too. In particular, `deploy.workDir` has to stay the exact same path on the host and inside the container (see the comment on that volume in `docker-compose.yaml`) - fru-lab hands that path to the host's Docker daemon when it deploys free5GC/gNB/UE, so it has to resolve to the same place on both sides.

3. Up the compose

    ```bash
    docker compose up -d
    ```

    Open `http://<host IP>:8888`. The default db is stored at `/var/lib/frulab/db` and deploy state at `/var/lib/frulab/deploy`, both mounted in the compose file.

4. Down the compose

    ```bash
    docker compose down
    ```

## Install - Helm

The chart in `helm/` deploys fru-lab to Kubernetes. See [helm/README.md](helm/README.md).

## TODO

- Setup Script
