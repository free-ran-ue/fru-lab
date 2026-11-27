# fru-lab

fru-lab is a GUI-driven 5G lab: from the browser it deploys free5GC cores and free-ran-ue gNBs / UEs as containers on the host's Docker daemon, and shows them as a topology, a 2D map ("twin"), lists, events and logs. One binary: a Go backend (`web/backend`) that also serves the built React frontend (`web/frontend`).

## Commands

| What | Command |
| - | - |
| Build both | `make` (backend → `build/fru-lab`, frontend → `web/frontend/dist`) |
| Run | `make run` (uses `./config.yaml`, port 8888) |
| Backend checks | `cd web/backend && go build ./... && go test ./... && golangci-lint run` |
| Frontend checks | `cd web/frontend && npx tsc -b && yarn build` |
| Regenerate API clients | `make openapi` (fru-lab API), `make openapi-webconsole` (free5GC webconsole) |

`yarn lint` is broken (ESLint flat-config issue in `eslint.config.js`) and is not part of the checks. The generators run in Docker; files they create may be owned by root.

## Rules

- **API first.** Every REST endpoint lives in `web/openapi.yaml`. Change the YAML, run `make openapi`, then use the generated client (`api` from `src/apiClient.ts`; `webconsoleClient()` from `src/webconsoleApiClient.ts`). Never hand-write `fetch` / `axios` calls to REST paths. Only the WebSocket streams (`/api/stream/logs`, terminals) are built by hand, because the generator can't produce them.
- **Don't run a second fru-lab against the Docker daemon a real lab uses.** Every instance controls the same compose projects and networks (`frulab-*`). Stopping one with Ctrl+C / SIGTERM tears down every lab container and network on the host, whichever instance deployed them. Use SIGKILL to stop an instance without the teardown.
- **Verify with builds and unit tests.** UI changes are checked by a person running the app; don't deploy labs or take screenshots to verify.
- **Commits:** only when asked, and no AI co-author / attribution trailers.

## Backend (`web/backend`)

Layers: `internal/api_*.go` (gin handlers, routes) → `internal/processor` (validation, rules, conflicts) → `internal/context` (Docker Compose, bbolt, IPAM). Models are in `model/`. Errors go up as `*model.ErrorDetail` (HTTP status + message).

- **Deploy targets** (compose projects):
  - `free5gc`: the lab free5GC, project `frulab-free5gc`.
  - `core-<id>`: an extra free5GC on this host (a core network of kind `free5gc`), project `frulab-core-<id>`, containers `c<slot>-*`.
  - `gnb-<profileId>`: one gNB per gNB profile.
  - `ue` instances `imsi-<…>`: one container per UE.
- **Profiles are records:** a gNB profile is one gNB, a UE profile is one UE. Nothing deploys without a saved profile.
  - gNB profiles carry a preset core (`coreNetwork`) and an optional `radioRange` (metres; 0 = unlimited). It is only used by the UI map.
  - UE profiles carry a preset gNB (`gnbProfileId`).
  - A deploy request can override either preset (`core`, or `gnb` / `"auto"`).
- **Core networks** (`/api/core-networks`):
  - Kind `external`: enable/disable; the user gives the N2/N3 IP and interface. A gNB on it listens on the core's N2 interface, at a host IP typed into the gNB profile.
  - Kind `free5gc`: fru-lab deploys it and assigns slot 1–9, its addresses, and webconsole port = config port + slot. Its webconsole is proxied at `/api/core-networks/<id>/webconsole/…`.
  - The lab free5GC is not a record; its name `free5GC` is reserved.
- **IPAM** (`internal/context/ipamPlan.go`, `ipamContext.go`) is the only source of addresses. Never hard-code an IP elsewhere.
  - Pools:
    - `cn-ran`: 10.0.1.0/24, the lab core. AMF .3, UPF .5; gNB N2 range .20–.254.
    - `sbi`: 10.0.10.0/24.
    - `ran-ue`: 10.0.2.0/24. Docker gives UEs .2–.127; gNB RAN range .200–.250.
    - Per core: `cn-ran-<id>` 10.0.(100+slot).0/24 and `sbi-<id>` 10.0.(110+slot).0/24.
  - Leases are kept in bbolt, so a redeployed gNB gets its addresses back. They are released when its profile or core is deleted.
  - Reconcile against Docker only warns; it never drops leases. A network that would overlap a non-fru-lab network or route is refused.
  - `GET /api/ipam` exposes it all, shown on the Addresses page.
- **free5GC templates** (`internal/context/templates/free5gc*`) are Go `text/template` files (`*.tmpl`). They take their addresses, network names and container prefix from the IPAM plan. The lab core renders exactly as the original files did.
- **Dependency rules** (enforced in the processor, mirrored in the UI's `blocked`):
  - A core can't stop while a gNB on it runs.
  - A gNB can't stop while a UE recorded on it runs.
  - A UE needs a running gNB.
- **Shutdown:** `FlContext.Release` releases compose before the DB, because the teardown reads the plan.
  - Teardown order: UEs → gNBs → extra cores → lab free5GC → the `frulab-*` networks.

## Frontend (`web/frontend/src`)

React 19 + Vite + CSS Modules. One shared state: `lab/useLab.ts` via `LabContext`. It polls every deploy target, the profiles, the core networks and IPAM. `lab/console/EventsContext.tsx` follows logs, replaying everything since each container started, and parses them into events (`lab/console/events.ts`).

- **Pages:**
  - Overview: Topology and Twin, plus Events and Logs sub-pages.
  - Core Network: list, then free5GC (with a Subscribers tab), then one page per core.
  - free-ran: gNBs.
  - free-ue: UEs.
  - Tools: Images, Addresses.
- **Layout pattern for record pages** (gNB, UE, core networks):
  - Left `ItemList`, right detail, in a `Split` (gNB/UE default 6:4, draggable).
  - List actions sit in the shared `workbench/ListToolbar.tsx`:
    - Row 1: + New, Bulk create, find, All/Running.
    - Row 2: Deploy/Delete selected, Start all stopped, Stop all.
  - Each row ends with its Deploy/Stop.
  - Single-item actions are in the sticky `workbench/DetailHeader.tsx`.
  - The detail has Status / Config tabs; Config forms have Save/Discard at the top.
  - New and Duplicate forms have Cancel.
- **Deploy menus** (`lab/GnbPicker.tsx`):
  - `PickMenu` is the generic picker. `CoreMenu` picks a gNB's core; `DeployMenu` picks a UE's gNB.
  - Bulk actions deploy onto the presets; a row's Deploy opens the menu.
- **Tables:** every table sorts (`lab/sort.tsx`: `useSort`, `SortTh`; `ItemList` sorts built in). Lists default to profile name A→Z.
- **Feedback:** use `components/feedback/Feedback.tsx`, never `window.confirm` / `alert`.
  - `useNotifications()` shows toasts top-right for 5 s.
  - `useConfirm()` opens the confirm dialog.
- **Inputs:** number fields use `lab/NumberInput.tsx`. The user can clear and retype; leaving the field invalid restores the last value.
- **Topology** (`lab/canvas/graph.ts`, `LabCanvas.tsx`):
  - One invisible row per core: UEs → gNBs → core → its DNs.
  - DNs with the same name are one node.
  - Every link is labelled (Uu, N2 · N3, N6).
  - Click selects, double-click opens a terminal (running gNB/UE).
- **Twin** (`lab/twin/TwinView.tsx`):
  - A grid map with four layers (UE, gNB, Core, DN) that can be shown, hidden or locked.
  - Every point is draggable and snaps to the grid. Positions are kept in localStorage, and the scale (metres per cell) is configurable.
  - Radio model (`lab/twin/signal.ts`): `S(d) = 1 / (1 + 99 (d/R)²)`. That is 100% at the gNB and 1% at the radio range R, then inverse-square and never 0. It is display only; UEs never detach because of it.
- **Terminals:** `workbench/TerminalDock.tsx` on the Overview, with split panes by dragging tabs. A shell that exits closes its pane.
- **Style:** an "engineering workbench" look.
  - No gradients, bouncy motion, big radii, glow or emoji.
  - Colours only from the tokens in `src/index.css`. This must print nothing: `grep -rnE '#[0-9a-fA-F]{3,8}\b' src --include=*.css --include=*.tsx | grep -v 'src/index.css\|src/api/\|webconsoleApi'`
  - Sizes are in rem. The root font size sets the scale of the whole UI (`html` in `index.css`).
  - Honour `prefers-reduced-motion`.
