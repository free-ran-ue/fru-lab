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

## Guide

- [Basic Guide](docs/basic-guide.md) - how to log in and perform the core deploy / monitor workflow.
