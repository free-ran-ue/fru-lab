# Basic Guide

After starting the app with `make run`, open it in your browser with port `http://<ip>:8888` to get started.

## 1. Login

Sign in with username and password.

- username: `admin`
- password: `frulab`

![Login screen](./images/login.png)

## 2. Dashboard overview

After logging in you land on the Dashboard, which shows a simple topology of the Core / gNB / UE nodes along with stats cards at the top. Click a node to see its details and Deploy / Stop actions on the right.

![Dashboard overview](./images/dashboard-overview.png)

## 3. Deploy / stop order

The system enforces a `Core -> gNB -> UE` order for both deploying and stopping, to avoid leaving the network in an inconsistent state:

- Deploy: Core must be deployed before gNB, and gNB must be deployed before any UE.
- Stop: all UEs must be stopped before gNB, and gNB must be stopped before Core.

If the current state doesn't satisfy the order, the corresponding button is disabled and shows the reason.

## 4. Deploy Core / gNB

Click the Core or gNB node in the topology to see its current status and container details, with Deploy / Stop buttons. Once Core is deployed, its network functions (AMF, SMF, UPF, etc.) show their health below.

While Core is stopped, a "Template" dropdown appears next to "Deploy Service" so you can pick which compose template to deploy:

- **Basic** - a single UPF (default).
- **ULCL** - the data plane is split into an Intermediate UPF (I-UPF) and a PDU Session Anchor UPF (PSA-UPF), chained over N9.

The dropdown is only shown while stopped - once deployed, which template is running is shown in the "Core Network" stats card and reflected in the detailed topology below.

![Core detail panel](./images/core-detail-panel.png)

## 5. Add a 5G Subscriber

Open "5G Subscriber" in the left sidebar to create / edit / delete UE subscriber data (IMSI, key, slice, etc.), which is used to deploy the matching UE.

![5G Subscriber page](./images/subscribers-page.png)

## 6. Deploy a UE

Back on the Dashboard, click the UE node to see all subscribers listed on the right. Click "Deploy" on any row to start that UE's container, or "Stop All" to stop every running UE at once.

![UE panel](./images/ue-panel.png)

## 7. Detailed network topology

Below the main topology, a more detailed diagram shows the live SBI service mesh, N2/N3 links, and the Uu link between gNB and each currently running UE. It automatically adapts to whichever Core template is deployed - with the ULCL template, it shows the I-UPF / PSA-UPF chain and the N9 link between them instead of a single UPF.

![Detailed network topology](./images/detailed-topology.png)

## 8. View logs

The "Logs" page in the sidebar shows live logs for Core / gNB / each UE. When Core is selected, logs can also be filtered by individual network function.

![Logs page](./images/logs-page.png)

## 9. Open a UE terminal

In the UE panel, click "Terminal" on a running UE instance to open a web-based terminal into that container and run commands directly.

![UE terminal](./images/ue-terminal.png)
