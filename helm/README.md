# fru-lab Helm chart

Deploys the fru-lab web console into Kubernetes.

```bash
cd helm
helm install frulab . --set frulab.docker.enabled=true --set frulab.docker.webconsoleHost=<node IP>
helm install frulab . -f my-values.yaml
```

Browse to `http://<node IP>:30888` (login `admin` / `frulab`).

## Docker

fru-lab deploys free5GC, the gNB and the UEs as containers on a Docker
daemon. `frulab.docker.enabled: true` mounts the node's Docker socket and
needs `frulab.docker.webconsoleHost` (the node's IP). Pin the pod to that node
with `frulab.nodeSelector`. With it off (the default) the console starts but
the deploy pages fail.

## Data

`frulab.persistence.type` (fru-lab's database) is `hostPath` (default,
`/var/lib/frulab/db`), `pvc` or `emptyDir`. PVCs are kept on `helm uninstall`.
