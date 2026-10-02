# Throughput Tester Guide

The Throughput Tester is a lab tool that sits next to fru-lab's deploy features. It simulates many gNBs against **any** 5G core. It does not depend on the free5GC that fru-lab deploys.

A run does four things, with live statistics for each:

1. Brings up N2 for every simulated gNB (an SCTP association plus NG Setup).
2. Registers each gNB's UEs as soon as that gNB is up.
3. Establishes one PDU session per registered UE.
4. Starts fixed-rate uplink and downlink traffic for each UE 500 ms after its PDU session is up, and checks that it comes back through the UPF. The pause gives the UPF time to learn the gNB's downlink tunnel.

Traffic runs until you press **Stop**, which halts it at once and then cleans up.

The UEs never open a radio link: each gNB carries its UEs' NAS messages inside NGAP itself, over its single SCTP association.

The engine is a separate program, `fru-tester`. fru-lab stores your profile and forwards the tester pages' API calls to fru-tester, using a shared API token.

## Prerequisites

- The `sctp` kernel module is loaded: `sudo modprobe sctp`.
- fru-tester runs as root, or with `CAP_NET_ADMIN`. It adds one IP per gNB to the N2 interface when a run starts and removes them when the run stops.
- Pick a CIDR and a first IP that nothing else uses. fru-tester skips the network and broadcast addresses, the AMF and UPF IPs, and every IP already configured **on this host**. It cannot see IPs used by other machines, or by containers on a Docker bridge (for example the gNB container fru-lab deploys at `10.0.1.2`). Start well above those, for example `10.0.1.100`.
- The Setup page's defaults match fru-lab's basic free5GC template: interface `docker-cn-ran`, AMF `10.0.1.3:38412`, UPF `10.0.1.5:2152`, PLMN 208/93, TAC 000001, S-NSSAI 1/010203. For another core, change them.

## Subscribers

fru-tester never creates subscribers. Every UE in the run needs a subscriber in the core with all of the following:

- the SUPI the plan preview shows: each gNB row lists its first and last SUPI, and the UE template card lists the whole range;
- the UE template's K, OPc, AMF and SQN;
- a slice (S-NSSAI) that the gNB template advertises.

The defaults match free-ran-ue's sample subscriber, `imsi-208930000000001` with K `8baf…6862` and OPc `8e27…605d`, on slice 1/010203.

If a subscriber is missing or its keys differ, that UE fails registration. If its slice is not one the gNB advertises, the AMF rejects it with `5gmm(62)` ("no network slices available").

## Data plane

No TUN devices are created; the tester writes the packets itself.

| Direction | Path | Received at |
|---|---|---|
| Uplink | GTP-U from the gNB's N3 IP to the UPF's N3, inner IPv4/UDP from the UE's IP to the **sink IP** | the sink IP on the N6 interface, after the UPF decapsulated it |
| Downlink | plain UDP from the sink IP to the UE's IP, routed via the **UPF N6 IP** | the gNB's N3 IP, as GTP-U |

Every packet carries a small header with the UE, a sequence number and the send time. Packets are matched to UEs even when the UPF rewrites the source address (fru-lab's UPF does), and leftovers from an earlier run are ignored.

For the run, the tester adds each gNB's N3 IP, the sink IP if the host doesn't have it, and the route `UE pool via UPF N6 IP`. Afterwards it removes exactly what it added. It never overwrites an existing route to the UE pool through a different gateway; the run fails with a message instead.

fru-lab's free5GC has no separate N6 network: the UPF sends decapsulated uplink out on `docker-cn-ran`. So the defaults use the host's own address there (`10.0.1.1`) as the sink, `10.0.1.5` as the UPF N6 IP, and `10.60.0.0/16` as the UE pool.

Rates are per UE (uplink and downlink Mbps, 0 = off, default 1 each) with one packet size. The Setup page shows the total load for the run.

## Pacing

Registration and PDU sessions each have four settings:

- **Starts per second**: how fast new attempts begin.
- **Max in flight**: how many attempts may be waiting for the core at once.
- **Timeout per attempt**.
- **Retries**.

With a fast core, the rate decides. When the core slows down, the in-flight limit stops requests from piling up; you see the queue grow instead. A UE whose attempt failed with retries left goes to the back of the queue.

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
   - Fill in the gNB template, the UE template, the N2 and N3 interface and CIDR, and the pacing for N2, registration and PDU sessions.
   - The plan preview shows the ID, name, N2 IP, N3 IP, UE range and SUPI range each gNB will get.
   - Any problem is shown under the field it belongs to.
2. **Start run** saves the profile and opens **Run**. Only one run can be active at a time.
3. **Stop** closes every N2 association and removes the gNB IPs.

## What the stage cards mean

There are three cards: N2 setup (per gNB), Registration (per UE) and PDU session (per UE). They share the same numbers:

| Number | Meaning |
|---|---|
| accepted / expected | Items (gNBs or UEs) that succeeded / items in the run |
| Rejected | The core said no: NGSetupFailure (`misc(4)`), Registration Reject (`5gmm(N)`) or PDU Session Establishment Reject (`5gsm(N)`). Causes are listed under *Failure causes* |
| Timed out | No answer within the timeout (connect or NG Setup). `operation now in progress` means the SCTP connect itself got no answer |
| Connection errors | Local or transport error, e.g. `connection refused` or `association lost` |
| Retries | Extra attempts. A failed item with retries left goes to the back of the queue |
| Not started | Items never attempted: their gNB never came up, an earlier stage failed them, or the run was stopped first |
| Total time | Wall clock from the first attempt starting to the last item finishing |
| Average, p50/p95/p99, Max | Time of **accepted** items, measured from each item's first attempt, so retries count |

What each stage times:

| Stage | Timed from | Timed to |
|---|---|---|
| N2 setup | SCTP connect | NG Setup Response |
| Registration | Initial UE Message (Registration Request) sent | Registration Complete sent |
| PDU session | PDU Session Establishment Request sent | Accept received (the gNB has already answered PDU Session Resource Setup) |

If **every** PDU session of a run times out while registration succeeds, look at the core first. Check the SMF log for charging (CHF) timeouts; the usual cause is CHF billing (CGF) being on (see Known limitations).

The **Data plane** card shows, per direction:

- **Sent / received**: the rate over the last second. *Received* is what actually made it through the UPF.
- **Packets per second** and **Total sent / received**: totals count inner IP packet bytes.
- **Loss**: `1 - received/sent` for the run so far.
- **Latency p50 / p99**: one way; send and receive use the same host clock.
- **Out of order** and **Send errors** (the local socket refused a packet).
- **Misrouted** (shown only when not 0): downlink that came back in another UE's tunnel or at another gNB. It is not counted as received.

Uplink and downlink each have their own panel and chart. The chart always shows the whole run from 0 (time as h:mm:ss) and squeezes as the run goes on; after the first 10 minutes each point is the average of 2, then 4, 8 ... seconds, so the chart stays light however long the run is. the dashed orange line is sent (Tx), the solid green line is received (Rx). When they overlap, nothing is being lost. Hover over a chart to read both rates at that second. The gNB table adds the bytes received per gNB in each direction, with their loss.

The **UEs** card counts every UE by state: established, establishing, registered, registering, pending, failed, gNB down, and cancelled (stopped before it finished). It also lists the first 200 failed UEs with their SUPI, gNB, stage, cause and attempt count. The gNB table shows how many of each gNB's UEs registered and how many got a PDU session.

gNB states: `pending` → `connecting` → `up` or `failed`.
- `lost` means the association was up and the AMF side dropped it, for example an AMF restart.
- `closed` means it was closed by Stop.

## Known limitations

- If fru-tester is killed with SIGKILL, the gNB IPs it added stay on the interface. Later runs skip them as host IPs; remove them with `ip addr del`.
- Messages the AMF sends after NG Setup are read and ignored.
- There is no run history yet.
- Stop does not release PDU sessions or deregister UEs (that comes in a later phase), so the core keeps their sessions until the same SUPIs register again. free5GC then tears them down, which works.
- free5GC's CHF stops answering charging requests after the first few UEs when its CDR delivery to the webconsole billing FTP (`chfcfg.yaml` → `cgf.enable`) is on. The SMF then waits 10 s per PDU session and the sessions time out (the SMF logs `Send Charging Data Request ... Failed`). fru-lab's free5GC templates therefore ship with `cgf.enable: false`. A core deployed by an older fru-lab, or any other core with CGF on, needs the same change and a CHF restart.
- The SQN in the network's AUTN is accepted without a freshness check (as in free-ran-ue), so re-running the same UEs never needs a resynchronisation.
- A UE that reached `established` on a gNB later marked `lost` still counts as established.
- Traffic starts a fixed 500 ms after each PDU session. A core that takes longer to install the downlink tunnel in the UPF loses the first downlink packets of each UE. Against fru-lab's free5GC, 4 UEs at 1 Mbps ran with 0 loss in both directions.
- The sender uses plain UDP sockets, and one socket sends about 80–90 k packets/s (~1 Gbps at 1400-byte packets). Uplink has one socket per gNB and downlink four, so on the test host one gNB's uplink and one UE's downlink each top out near 1 Gbps. 4 UEs reached 3 Gbps downlink with no loss.
- High rates need the SMF's `urrThreshold` raised. At a few hundred bytes, the UPF sends a usage report every packet or two, PFCP starves, and downlink stops reaching the gNB. fru-lab's templates set it to 10 GB. A core deployed by an older fru-lab, or any other core, needs the same change.
- Back-to-back runs with the same UEs can leave free5GC with duplicate PDU sessions (`Duplicated PDU session ID` in the AMF log), so their PDU sessions time out. Restart the core, or wait for the next phase, which releases sessions and deregisters UEs on Stop.
