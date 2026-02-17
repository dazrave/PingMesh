# PingMesh

Distributed multi-vantage monitoring system with quorum-based consensus. Multiple lightweight nodes run checks from different networks and confirm failures with each other before alerting — eliminating false positives caused by localised network issues.

## How It Works

```
                    ┌─────────────────────────┐
                    │      Coordinator         │
                    │  (also runs checks)      │
                    │                          │
                    │  Config store (canonical) │
                    │  Quorum evaluator        │
                    │  Alert manager           │
                    └─────┬──────┬──────┬──────┘
                          │      │      │
                     mTLS │ mTLS │ mTLS │
                          │      │      │
                 ┌────────┘      │      └────────┐
           ┌─────▼─────┐  ┌─────▼─────┐  ┌─────▼─────┐
           │  Node A    │  │  Node B    │  │  Node C    │
           │  EU-West   │  │  US-East   │  │  AU-Syd    │
           └────────────┘  └────────────┘  └────────────┘
```

Every node runs the **same binary**. One node is designated coordinator via `pingmesh init`; others join via `pingmesh join <token>`. All inter-node communication is secured with mutual TLS using an internal certificate authority.

When a node detects a failure, it asks peers to confirm before raising an alert. Only when a **quorum** of nodes agrees does an incident get confirmed — no more 3am pages because one datacenter had a blip.

## Features

- **6 check types**: ICMP ping, TCP port, HTTP/HTTPS status, DNS resolution, HTTP keyword match
- **Quorum consensus**: Majority or N-of-M confirmation before alerting
- **Automatic mTLS**: Internal CA generated on init, certificates issued on join
- **Single binary**: No runtime dependencies, cross-compiles to linux/amd64 and linux/arm64
- **Lightweight**: ~10-20MB RSS, runs on Raspberry Pi and LXC containers
- **Graceful degradation**: Nodes keep running with cached config if coordinator is unreachable
- **Hysteresis**: Configurable failure/recovery thresholds and cooldown periods

## Quick Start

### Build

Requires Go 1.23+.

```bash
make build
```

Cross-compile for ARM64:

```bash
make build-arm64
```

### Initialize the Coordinator

```bash
pingmesh init --name coordinator --listen 0.0.0.0:7433
```

This generates the internal CA, creates the local database, and starts the coordinator.

### Start the Agent

```bash
pingmesh agent
```

Or use the systemd service:

```bash
sudo cp configs/pingmesh.service /etc/systemd/system/
sudo systemctl enable --now pingmesh
```

### Add a Monitor

```bash
pingmesh monitor add \
  --name "Example Website" \
  --type https \
  --target example.com \
  --interval 60s \
  --timeout 5s
```

### Join a Node

On the coordinator:

```bash
pingmesh join-token
```

On the new node:

```bash
pingmesh join <token>
```

### View Status

```bash
pingmesh status         # Cluster overview
pingmesh monitor list   # All monitors
pingmesh incidents      # Active incidents
pingmesh history        # Check result history
```

## CLI Reference

```
pingmesh
├── init        [--listen addr] [--name name]         Initialize as coordinator
├── join        <token> [--name name]                  Join existing cluster
├── join-token  [--expires duration]                   Generate join token
├── agent                                              Start the daemon
├── node
│   ├── list                                           List cluster nodes
│   ├── show    <id>                                   Show node details
│   └── remove  <id>                                   Remove a node
├── monitor
│   ├── list    [--group name]                         List monitors
│   ├── add     --name N --type T --target HOST ...    Create monitor
│   ├── show    <id>                                   Show monitor details
│   ├── edit    <id> [flags]                           Update monitor
│   └── delete  <id>                                   Delete monitor
├── status                                             Cluster overview
├── incidents   [--active]                             List incidents
├── history     [--monitor id] [--node id] [--since]   Check result history
└── health                                             Local node health
```

## Check Types

| Type | Description | Key Options |
|------|-------------|-------------|
| `icmp` | ICMP ping | target |
| `tcp` | TCP port connectivity | target, port |
| `http` | HTTP status check | target, port, expected-status |
| `https` | HTTPS with TLS validation | target, port, expected-status |
| `dns` | DNS resolution | target, dns-type, dns-expect |
| `http_keyword` | HTTP response keyword match | target, keyword, expected-status |

## Consensus

PingMesh uses a two-phase approach to avoid false positives:

1. **Local detection**: A node sees consecutive failures exceeding `failure_threshold`
2. **Peer confirmation**: The node asks all peers to run the same check
3. **Quorum evaluation**: If enough peers confirm the failure, the incident is marked as confirmed

Quorum modes:
- **majority**: More than half of responding nodes must see the failure
- **n_of_m**: At least N nodes must confirm (configurable per monitor)

Recovery follows the same pattern — a monitor is only marked as recovered when a majority of nodes see it healthy, exceeding the `recovery_threshold` for consecutive successes.

## Configuration

PingMesh stores its configuration and data in `/var/lib/pingmesh` by default (override with `--data-dir`):

```
/var/lib/pingmesh/
├── config.json     # Node configuration
├── pingmesh.db     # SQLite database
└── certs/          # TLS certificates
    ├── ca.crt
    ├── ca.key      # Only on coordinator
    ├── node.crt
    └── node.key
```

### Network Ports

| Port | Interface | Purpose |
|------|-----------|---------|
| 7433 | 0.0.0.0 | Peer mTLS API (inter-node) |
| 7434 | 127.0.0.1 | CLI API (local only) |

## Deployment

### Installer Script

```bash
curl -fsSL https://raw.githubusercontent.com/dazrave/PingMesh/main/scripts/install.sh | bash
```

### Docker

```bash
docker build -t pingmesh .
docker run -v pingmesh-data:/var/lib/pingmesh pingmesh
```

### systemd

The included service file provides security hardening:
- Runs as dedicated `pingmesh` user
- `CAP_NET_RAW` for ICMP checks
- Read-only filesystem except data directory
- Private tmp, no new privileges

## Project Structure

```
PingMesh/
├── cmd/pingmesh/main.go          Entry point
├── internal/
│   ├── cli/                       CLI commands (cobra)
│   ├── config/                    Configuration management
│   ├── model/                     Domain types
│   ├── store/                     SQLite storage layer
│   ├── checker/                   Check implementations
│   ├── agent/                     Scheduler + agent loop
│   ├── api/                       REST API (CLI + peer)
│   ├── cluster/                   Membership + mTLS + join
│   ├── consensus/                 Quorum evaluation + incidents
│   └── alert/                     Alert dispatch
├── configs/pingmesh.service       systemd unit
├── scripts/install.sh             Installer
├── Makefile
├── Dockerfile
└── go.mod
```

## Development Status

- [x] **M1**: Foundation — CLI skeleton, config, SQLite store, domain types
- [ ] **M2**: Check Engine — All check types, scheduler, single-node operation
- [ ] **M3**: Cluster + Security — Init/join flow, mTLS, peer discovery, config sync
- [ ] **M4**: Consensus + Alerting — Peer confirmation, quorum, incidents, webhooks
- [ ] **M5**: Packaging — Cross-compilation, installer, systemd, upgrades

## License

[MIT](LICENSE)
