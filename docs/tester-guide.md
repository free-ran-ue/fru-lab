# Throughput Tester Guide

The Throughput Tester is a lab tool that sits next to fru-lab's deploy features. It simulates many gNBs against **any** 5G core. It does not depend on the free5GC that fru-lab deploys.

A run does four things, with live statistics for each:

1. Brings up N2 for every simulated gNB (an SCTP association plus NG Setup).
2. Registers each gNB's UEs as soon as that gNB is up.
3. Establishes one PDU session per registered UE.
4. Starts fixed-rate uplink and downlink traffic for each UE 500 ms after its PDU session is up, and checks that it comes back through the UPF. The pause gives the UPF time to learn the gNB's downlink tunnel.

Traffic runs until you press **Stop** (or the max run time is reached). Traffic halts at once, then the run cleans up:

1. Every UE that registered deregisters. The core releases its PDU session as part of that; there is no separate PDU Session Release.
2. Every gNB's SCTP association is closed.
3. The IPs and route the run added are removed.

Both cleanup steps appear as stage cards on the Run page. Pressing Stop again during cleanup (**Skip cleanup**) stops waiting for the remaining deregistrations; SCTP is still closed and the IPs removed. A new run can start once cleanup is over.

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

Rates are per UE (uplink and downlink Mbps, 0 = off, default 1 each) with one packet size. The packet size (inner IP packet, default 1400) can go up to the N3 interface's MTU minus 44 bytes of GTP-U and its headers, and up to the N6 interface's MTU: 1456 on a 1500 MTU network. **Max run time** (minutes, 0 = no limit) stops the run by itself, exactly like pressing Stop. **Downlink batch** (ms, default 1, 0 = off) sends each UE that much time's worth of downlink packets in a row, so the kernel can take them in one send (UDP GSO); a larger value saves CPU but makes each UE's downlink burstier. Uplink needs no setting: all of a gNB's uplink goes to the UPF and is always sent this way. The Setup page shows the total load for the run.

## Pacing

Registration, PDU sessions and deregistration (on Stop) each have four settings:

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
3. **Stop** stops the traffic, deregisters the UEs, closes every N2 association and removes the gNB IPs.
4. **History** lists the last 50 finished runs (kept by fru-lab). Tick two runs to compare them side by side; **JSON** downloads the full report (profile and final numbers), **CSV** the throughput time series.

## What the stage cards mean

There are three cards: N2 setup (per gNB), Registration (per UE) and PDU session (per UE). After Stop, two cleanup cards appear above them: UE deregistration (per UE) and gNB SCTP close (per gNB). They all share the same numbers:

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
| UE deregistration | Deregistration Request sent | Deregistration Accept received |
| gNB SCTP close | close started | close returned (up to about 1 s: the socket lingers to deliver what is queued) |

In the cleanup cards, *Not started* counts UEs that never registered, UEs and gNBs whose association was lost, and deregistrations still queued when a second Stop skipped the cleanup. Deregistrations already sent at that moment count as *Connection errors* with the cause `cleanup skipped`. Either way those UEs end up *cancelled*; a UE whose PDU session had failed stays *failed*.

If **every** PDU session of a run times out while registration succeeds, look at the core first. Check the SMF log for charging (CHF) timeouts; the usual cause is CHF billing (CGF) being on (see Known limitations).

The **Data plane** card shows, per direction:

- **Sent / received**: the rate over the last second. *Received* is what actually made it through the UPF.
- **Packets per second** and **Total sent / received**: totals count the inner IP packets, shown in bits (Mb, Gb) like the rates.
- **Loss**: `1 - received/sent` for the run so far.
- **Latency p50 / p99**: one way; send and receive use the same host clock.
- **Out of order** and **Send errors** (the local socket refused a packet).
- **Misrouted** (shown only when not 0): downlink that came back in another UE's tunnel or at another gNB. It is not counted as received.

Uplink and downlink each have their own panel and chart. The chart always shows the whole run from 0 (time as h:mm:ss) and squeezes as the run goes on; after the first 10 minutes each point is the average of 2, then 4, 8 ... seconds, so the chart stays light however long the run is. The dashed orange line is sent (Tx), the solid green line is received (Rx). When they overlap, nothing is being lost. Hover over a chart to read both rates at that second. The gNB table adds the data received per gNB in each direction (in bits), with their loss.

The **UEs** card counts every UE by state: established, establishing, registered, registering, pending, failed, gNB down, and cancelled (stopped before it finished). After Stop it also counts deregistering and deregistered UEs. It also lists the first 200 failed UEs with their SUPI, gNB, stage (registration, PDU or deregistration), cause and attempt count. The gNB table shows how many of each gNB's UEs registered and how many got a PDU session.

gNB states: `pending` → `connecting` → `up` or `failed`.
- `lost` means the association was up and the AMF side dropped it, for example an AMF restart.
- `closed` means it was closed by Stop.

## Known limitations

- If fru-tester is killed with SIGKILL, the gNB IPs it added stay on the interface. Later runs skip them as host IPs; remove them with `ip addr del`.
- Messages the AMF sends after NG Setup are read and ignored.
- fru-tester keeps its last 10 finished runs for fru-lab to collect; if fru-lab is down while more than 10 runs finish, the older ones never reach the History page.
- free5GC's CHF stops answering charging requests after the first few UEs when its CDR delivery to the webconsole billing FTP (`chfcfg.yaml` → `cgf.enable`) is on. The SMF then waits 10 s per PDU session and the sessions time out (the SMF logs `Send Charging Data Request ... Failed`). fru-lab's free5GC templates therefore ship with `cgf.enable: false`. A core deployed by an older fru-lab, or any other core with CGF on, needs the same change and a CHF restart.
- The SQN in the network's AUTN is accepted without a freshness check (as in free-ran-ue), so re-running the same UEs never needs a resynchronisation.
- A UE that reached `established` on a gNB later marked `lost` still counts as established.
- Traffic starts a fixed 500 ms after each PDU session. A core that takes longer to install the downlink tunnel in the UPF loses the first downlink packets of each UE. Against fru-lab's free5GC, 4 UEs at 1 Mbps ran with 0 loss in both directions.
- The data plane uses plain UDP sockets with UDP GSO and GRO, which any Linux kernel since 5.0 has (an older kernel falls back to one packet per message, 32 messages per system call). Downlink has one sender per CPU and uplink spreads as many over the gNBs, each with its own socket; uplink is received by one sink socket per CPU. The host's CPUs are the limit. Before GSO, on a 6-CPU test host, 10 gNBs × 40 UEs at 250 Mbps each way received 4.3 Gbps uplink and 2.2 Gbps downlink with under 0.3 % loss, with the CPUs fully busy (half of it the kernel's own packet handling, the UPF's included). With GSO one sender sends about 1.7 M packets/s (19 Gbps at 1400 bytes) on the same host, measured by the Bench page. The core network has not been measured again since GSO was added. The kernel still handles every packet on receive and in the UPF.
- UDP GRO only helps when packets arrive already merged: from a NIC that does GRO, or sent locally with GSO. Docker's veth links do not do GRO by default, so on fru-lab's bridge the tester mostly still reads one packet per message (`ethtool -K <veth> gro on` turns it on).
- **Bench** (Throughput Tester → Bench) measures how fast this host's fru-tester can send, with no core network: uplink over loopback with 1, 2, 4 … senders up to one per CPU. Its UDP GSO switch compares sending with and without GSO. It cannot run while a test run is active.
- High rates need the SMF's `urrThreshold` raised. At a few hundred bytes, the UPF sends a usage report every packet or two, PFCP starves, and downlink stops reaching the gNB. fru-lab's templates set it to 10 GB. A core deployed by an older fru-lab, or any other core, needs the same change.
- A run whose cleanup was skipped (second Stop, or fru-tester killed) leaves its UEs registered in the core. Running the same UEs again can then hit `Duplicated PDU session ID` in free5GC and time out; restart the core in that case.
- When you restart free5GC's containers, restart the NRF first and the SMF after the UPF; otherwise the SMF loses its PFCP association and every PDU session times out.
