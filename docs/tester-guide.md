# Throughput Tester Guide

The Throughput Tester is a lab tool that sits next to fru-lab's deploy features. It simulates many gNBs against **any** 5G core. It does not depend on the free5GC that fru-lab deploys.

**Phase 1** brings up N2 for every simulated gNB (an SCTP association plus NG Setup) and holds the associations until you press **Stop**, showing live statistics the whole time. UE registration, PDU sessions and the data plane come in later phases.

The engine is a separate program, `fru-tester`. fru-lab stores your profile and forwards the tester pages' API calls to fru-tester, using a shared API token.

## Prerequisites

- The `sctp` kernel module is loaded: `sudo modprobe sctp`.
- fru-tester runs as root, or with `CAP_NET_ADMIN`. It adds one IP per gNB to the N2 interface when a run starts and removes them when the run stops.
- Pick a CIDR and a first IP that nothing else uses. fru-tester skips the network and broadcast addresses, the AMF and UPF IPs, and every IP already configured **on this host**. It cannot see IPs used by other machines, or by containers on a Docker bridge (for example the gNB container fru-lab deploys at `10.0.1.2`). Start well above those, for example `10.0.1.100`.
- The Setup page's defaults match fru-lab's basic free5GC template: interface `docker-cn-ran`, AMF `10.0.1.3:38412`, UPF `10.0.1.5:2152`, PLMN 208/93, TAC 000001, S-NSSAI 1/010203. For another core, change them.

## Run in development

```bash
make backend frontend tester
make run          # fru-lab on :8888, reads config.yaml
make run-tester   # fru-tester on 127.0.0.1:9100 (sudo), reads tester.yaml
```

`config.yaml` → `backend.tester.url` / `apiToken` must match `tester.yaml` → `listen` / `apiToken`. If `backend.tester.url` is empty, the tester pages answer "Throughput Tester is not configured".

## Run with Docker Compose

`docker/docker-compose.yaml` runs `fru-tester` on the host network with `NET_ADMIN`. fru-lab reaches it at `http://host.docker.internal:9100`. Because the tester API then listens on every host interface, change `apiToken` from the default in **both** `docker/tester.yaml` and `docker/config.yaml`. Build both images with `make docker`.

## Using it

1. **Throughput Tester → Setup**
   - Enter the gNB count and UE count. UEs fill gNBs in order, and the last gNB may be partly filled.
   - Fill in the gNB template, the N2 and N3 interface and CIDR, and the N2 timeout and retries.
   - The plan preview shows the ID, name, N2 IP, N3 IP and UE range each gNB will get.
   - Any problem is shown under the field it belongs to.
2. **Start run** saves the profile and opens **Run**. Only one run can be active at a time.
3. **Stop** closes every N2 association and removes the gNB IPs.

## What the N2 card means

| Number | Meaning |
|---|---|
| accepted / expected | gNBs whose NG Setup succeeded / gNBs in the run |
| Rejected | The AMF answered NGSetupFailure; the cause, e.g. `misc(4)`, is listed under *Failure causes* |
| Timed out | No answer within the timeout (connect or NG Setup). `operation now in progress` means the SCTP connect itself got no answer |
| Connection errors | Local or transport error, e.g. `connection refused` |
| Retries | Extra attempts. A failed gNB with retries left goes to the back of the queue |
| Total time | Wall clock from the first attempt starting to the last gNB finishing |
| Average, p50/p95/p99, Max | Setup time of **accepted** gNBs, measured from each gNB's first attempt, so retries count |

gNB states: `pending` → `connecting` → `up` or `failed`.
- `lost` means the association was up and the AMF side dropped it, for example an AMF restart.
- `closed` means it was closed by Stop.

## Known limitations (Phase 1)

- If fru-tester is killed with SIGKILL, the gNB IPs it added stay on the interface. Later runs skip them as host IPs; remove them with `ip addr del`.
- Messages the AMF sends after NG Setup are read and ignored.
- There is no run history yet.
