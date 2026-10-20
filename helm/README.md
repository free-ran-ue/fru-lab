# fru-lab Helm chart

Deploys the fru-lab web console and the fru-tester throughput tester into
Kubernetes.

```bash
cd helm
helm install frulab .                     # defaults: see values.yaml
helm install frulab . -f my-values.yaml   # your masters, node, CPUs
```

Browse to `http://<node IP>:30888` (login `admin` / `frulab`).

## Before installing

- **Multus** with the `macvlan` CNI plugin (the default network mode creates
  `NetworkAttachmentDefinition`s; without the CRD the install fails).
  On MicroK8s: `microk8s enable multus`.
- **SCTP** on the tester's node: `modprobe sctp`.
- The macvlan masters (`dummy-n2`, `dummy-n3`, `dummy-n6` by default) exist
  on that node, and the tester pod is pinned there with `tester.nodeSelector`.

## Networking

`tester.network.mode: macvlan` (default) gives the tester pod interfaces
`n2`, `n3` and `n6`, macvlan links on the masters in
`tester.network.macvlan.interfaces`. A 5GC whose pods are macvlan links on the
same masters in bridge mode reaches them directly; a pod on the node's own
network could not, since a macvlan master cannot talk to its children.

- Profiles use `n2`, `n3` and `n6` as their interface names.
- The NADs have no IPAM: fru-tester adds each run's gNB IPs and sink IP itself
  and removes them afterwards. Keep the profiles' gNB ranges out of the 5GC's
  IPAM ranges.
- On macvlan the data plane uses UDP sockets. AF_XDP needs a NIC (for
  example an SR-IOV VF) in the pod.
- To use NADs that already exist, set `tester.network.macvlan.create: false`
  and each interface's `name` (`name` or `namespace/name`).

`tester.network.mode: host` runs the tester on the node's network like
docker-compose; profiles then name the node's interfaces.

## CPUs

The System page offers the CPUs the pod may use. For exclusive data plane
CPUs, run the kubelet with the static CPU manager policy and give the tester
Guaranteed QoS (equal integer cpu requests and limits in `tester.resources`).

## Data

`persistence.type` is `hostPath` (default, under `/var/lib/frulab`), `pvc` or
`emptyDir` for both fru-lab's database and the tester's System settings.
PVCs are kept on `helm uninstall`.

## free5GC deploy pages

fru-lab's own free5GC/gNB/UE deploy pages need a Docker daemon. They are off
by default; `frulab.docker.enabled: true` mounts the node's Docker socket and
needs `frulab.docker.webconsoleHost` (the node's IP).
