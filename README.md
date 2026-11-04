# fru Lab

![frulab](./docs/images/frulab-logo.png)

Deploy and monitor the free5GC core network and the free-ran-ue gNB / UE simulator, and load-test a 5G core with the Throughput Tester.

fru-lab has two parts:

- **fru-lab**, the web console (Go backend + React frontend). It deploys free5GC, the gNB and one UE container per subscriber on the host's Docker daemon, manages subscribers through the free5GC webconsole, and shows logs and images.
- **fru-tester**, the Throughput Tester. It simulates gNBs and tens of thousands of UEs against any 5G core (N2 setup, registration, PDU sessions) and drives GTP-U traffic through it. fru-lab's "Load testing" pages talk to one or more fru-testers over their API.

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
| Make all (backend, frontend, tester) | `make` |
| Backend | `make backend` |
| Frontend | `make frontend` |
| Tester | `make tester` |
| Run | `make run` |
| Run tester | `make run-tester` |
| Test | `make test` |
| Tidy | `make tidy` |
| Lint | `make lint` |
| Clean | `make clean` |
| Generate frontend openapi | `make openapi` |
| Generate frontend openapi for free5GC webconsole | `make openapi-webconsole` |
| Docker images (fru-lab and fru-tester) | `make docker` |

`make run` starts fru-lab with `./config.yaml` on port `8888`. `make run-tester` loads the `sctp` kernel module and starts fru-tester with `./tester.yaml` under `sudo` (it needs root to add gNB IPs and open SCTP).

## Throughput Tester

fru-tester is built with `make tester` and started with `make run-tester`. fru-lab finds it through `backend.tester` in `config.yaml`:

```yaml
tester:
  # fru-tester API (see tester.yaml). Leave url empty to disable the pages.
  url: "http://127.0.0.1:9100"
  apiToken: "frulab-tester"
```

`apiToken` must match `apiToken` in `tester.yaml`. More testers can be added from the Testers page.

The Throughput Tester guide is written in Chinese: [docs/superpowers/throughput-tester-guide.html](docs/superpowers/throughput-tester-guide.html) (open the file in a browser).

## Install - Docker Compose

The compose file runs two containers: `fru-lab` and `fru-tester`. fru-lab deploys free5GC/gNB/UE by talking to the host's Docker daemon, so its container needs the host's Docker socket mounted in, on top of the usual config/db volumes. fru-tester runs on the host network.

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

    fru-tester's settings are in `tester.yaml`. Its `apiToken` must match `backend.tester.apiToken` in `config.yaml`.

3. Load SCTP on the host (fru-tester needs it)

    ```bash
    sudo modprobe sctp
    ```

4. Up the compose

    ```bash
    docker compose up -d
    ```

    Open `http://<host IP>:8888`. The default db is stored at `/var/lib/frulab/db` and deploy state at `/var/lib/frulab/deploy`, both mounted in the compose file. fru-tester keeps its CPU allocation in `./frutester-data`.

5. Down the compose

    ```bash
    docker compose down
    ```

## Install - Helm

The chart in `helm/` deploys fru-lab and fru-tester to Kubernetes. See [helm/README.md](helm/README.md).

## Guide

- [Basic Guide](docs/basic-guide.md) - log in, deploy the core, add subscribers, deploy UEs, open a UE terminal and read logs.
- [Images Guide](docs/images-guide.md) - check, pull, update and clear the Docker images this app deploys.
- [Throughput Tester guide (Chinese)](docs/superpowers/throughput-tester-guide.html) - set up testers and profiles, start runs and read the results.

## TODO

- Setup Script
