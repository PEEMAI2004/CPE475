# PotBuddy Project Context & Handoff

Welcome, next Agent! This document contains all the crucial context for the PotBuddy IoT project to help you pick up exactly where we left off.

## 📐 General Architecture
PotBuddy is an end-to-end IoT Plant Care monitor relying heavily on MQTT streams.
- **Hardware:** ESP32 devices publishing raw sensor payload (Light, Temp, Humidity, Soil) over WiFi.
- **Data Pipeline:** 
  1. Devices -> Local Mosquitto Broker (`10.0.0.65`)
  2. Local Go Node (`/opt/potbuddy/...`) interprets readings, grades their health against thresholds, and republishes telemetry.
  3. Cloud MQTT Broker -> Prometheus Scraper -> Grafana Visualization.

## 🖥️ Server Topology
All servers run Debian/Linux and are accessible via SSH `root@<IP>` (no password / key configured for the user session automatically).
- **`10.0.0.63`**: Hosts **Prometheus** (config in `/etc/prometheus/prometheus.yml`, port `:9090`).
- **`10.0.0.64`**: Hosts **Grafana** (port `:3000`, `admin:admin`, dashboard DB in SQLite).
- **`10.0.0.65`**: Hosts the **Go Processor Application** running via the systemd unit `potbuddy.service`. 
  - Remote Directory: `/opt/potbuddy/local-node/`
- **`10.0.0.66`**: Hosts **PostgreSQL 17** (port `:5432`, `postgres:postgres`, DB name `potbuddy`).

## 🛠️ Codebase & Recent Milestones
The Go service source code resides strictly at: `/home/kamin/Documents/Uni/CPE475/Project/local-node`.

### 1. Offline Device Grafana Fix
We solved an issue where stale ESP32 devices lingered on the Grafana dashboard.
- **Implementation:** Added a Prometheus background TTL watchdog in `internal/metrics/metrics.go` exposing the metric `potbuddy_device_online`. This automatically flips to `0` if a payload hasn't arrived from a specific device in 30 seconds.
- **Grafana queries** were updated to cross-join using `and on(exported_device) (potbuddy_device_online == 1)` so offline rows organically disappear.

### 2. SQL Dynamic Threshold Profiles
We migrated the plant health boundary ranges from statically compiled hardcoded boundaries inside `internal/processor/thresholds.go` to PostgreSQL.
- The `internal/db/db.go` was created using `database/sql` + `lib/pq` to ping `10.0.0.66` every 60 seconds async.
- It parses tables `profiles` and `device_profiles` and injects them lock-safe into memory for high-throughput `Evaluate()` routing. Custom Profiles naturally fallback to the database profile named `default`.

## 🚀 Build and Deployment Workflow
If you modify the Go source code locally inside the `local-node` directory, execute via these exact steps to cross-compile and deploy gracefully considering file-locks.

```bash
# 1. Compile locally for Linux AMD64
cd /home/kamin/Documents/Uni/CPE475/Project/local-node
GOOS=linux GOARCH=amd64 go build -o bin/node-linux ./cmd/node

# 2. Stop service, upload replacing binary, Start service (Avoid "Text file busy" error)
ssh root@10.0.0.65 "systemctl stop potbuddy"
scp bin/node-linux root@10.0.0.65:/opt/potbuddy/local-node/bin/node
ssh root@10.0.0.65 "systemctl start potbuddy && sleep 3 && systemctl status potbuddy --no-pager"
```

*Good luck with the next iterations!*
