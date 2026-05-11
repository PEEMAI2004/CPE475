# PotBuddy Deployment Guide

PotBuddy supports two main avenues of deployment: **Local Development (Docker Compose)** and **Multi-Site Distributed Linux Services** (Production).

---

## 🌐 Networking & Port Map

For a distributed deployment, ensure the following ports are open:

| Component | Port | Protocol | Purpose |
| :--- | :---: | :--- | :--- |
| **Edge Site** | 8883 | TCP/mTLS | MQTT Device/Node Communication |
| **Edge Site** | 8080 | TCP/mTLS | Metrics Scraping (Prometheus) |
| **Manager API** | 8081 | TCP/HTTPS | Web Dashboard & Enrollment |
| **Database** | 5432 | TCP | Central Persistence (PostgreSQL) |
| **Monitoring** | 9090 | TCP | Prometheus Web Interface |
| **Monitoring** | 3000 | TCP | Grafana Visualization |

---

## Method A: Multi-Site Linux Deployment (Production)

This is the recommended deployment for a truly decoupled edge-cloud architecture, running as `systemd` services. All servers run Debian/Linux and are accessible via SSH `root@<IP>`.

### 1. Central Infrastructure (Cloud)

The central infrastructure hosts the database, manager API, and observability stack. 
Example IP topology:
- `postgresql.example.com` 
- `manager.example.com` 
- `prometheus.example.com` 
- `grafana.example.com` 

**Step 1.1: Database Setup**
1. Install PostgreSQL 17 on the database server .
2. Initialize the schema using the provided SQL file:
   ```bash
   psql -U postgres -f Project/init.sql potbuddy
   ```

**Step 1.2: Manager API Build & Deploy**
1. Compile the manager binary locally for Linux:
   ```bash
   cd Project/manager-api
   GOOS=linux GOARCH=amd64 go build -o bin/manager-linux ./cmd/manager/main.go
   ```
2. Upload and run on the manager server . Ensure `DB_DSN` and `ENABLE_CA=true` are set.
3. Access the web dashboard (e.g., `http://manager.example.com:8081`), log in via Google SSO, and invite site administrators via the **Users** tab.

### 2. Edge Site Setup (Repeat for Site 0...N)

Each physical site requires a local MQTT broker (Mosquitto) and a Go processor node.
Example Site 0 topology:
- `mqtt-0.example.com` 
- `debian-0.example.com` 

**Step 2.1: Site Enrollment (Zero-Trust)**
1. In the Central Dashboard, navigate to Sites and create a new node to get a **Site Token**.
2. On the site server, use the `enroll` tool to generate keys locally (private keys never leave the server).
   
   *Enroll Local Node:*
   ```bash
   ./enroll -token <SITE_TOKEN> -manager https://manager.example.com
   ```
   *Generates:* `client.key`, `client.crt`, `ca.crt`, `config.yaml`.

   *Enroll MQTT Broker:*
   ```bash
   ./enroll -type mqtt -token <SITE_TOKEN> -cn <BROKER_DOMAIN_OR_IP>
   ```
   *Generates:* `server.key`, `server.crt`, `mosquitto.conf`.

**Step 2.2: Edge Component Deployment**
1. **Mosquitto Broker**: 
   Move the generated certs to `/etc/mosquitto/certs/`. Apply the generated `mosquitto.conf`. Restart the Mosquitto service on the broker server.
2. **Local Node Processor**:
   Compile the node binary locally:
   ```bash
   cd Project/local-node
   GOOS=linux GOARCH=amd64 go build -o bin/node-linux ./cmd/node
   ```
   Deploy to the edge server (e.g., `debian-0.example.com`):
   ```bash
   ssh root@debian-0.example.com "systemctl stop potbuddy"
   scp Project/local-node/bin/node-linux root@debian-0.example.com:/opt/potbuddy/local-node/bin/node
   # Ensure config.yaml and certs are in the working directory
   ssh root@debian-0.example.com "systemctl start potbuddy"
   ```

---

## Method B: Docker Deployment (Development)

Best for testing everything on a single machine.

### 1. Configuration
Check `Project/.env` to ensure `GOOGLE_CLIENT_ID` and `JWT_SECRET` are set correctly for local testing.

### 2. Startup & Enrollment
Docker deployment requires an initial "Enrollment" phase to generate the mTLS certificates used for secure communication.

**Step 2.1: Start Core Infrastructure**
```bash
cd Project/
mkdir -p certs
docker compose up -d postgres manager-api prometheus
```

**Step 2.2: Generate Enrollment Tokens**
In a local environment, you can manually insert tokens into the database to bypass the UI invitation flow:
```bash
# Register Local Broker
docker exec potbuddy-local-postgres-1 psql -U postgres -d potbuddy -c \
"INSERT INTO infrastructure_nodes (name, type, site_id, address, token) \
VALUES ('Local Broker', 'MQTT Broker', 0, 'mqtt', 'pb_node_broker_token') ON CONFLICT (token) DO NOTHING;"

# Register Local Node
docker exec potbuddy-local-postgres-1 psql -U postgres -d potbuddy -c \
"INSERT INTO infrastructure_nodes (name, type, site_id, address, token) \
VALUES ('Local Node', 'Local Node', 0, 'localhost', 'pb_node_local_token') ON CONFLICT (token) DO NOTHING;"
```

**Step 2.3: Run Enrollment Tool**
Generate the certificates locally. Ensure you have Go installed, or use a pre-compiled binary.
```bash
# Enroll Broker
cd certs/
go run ../local-node/cmd/enroll/main.go -token pb_node_broker_token -type mqtt -cn mqtt

# Enroll Node
go run ../local-node/cmd/enroll/main.go -token pb_node_local_token -type node

# Ensure permissions (Required for Mosquitto)
chmod 644 *
```

**Step 2.4: Launch Remaining Services**
```bash
cd ..
docker compose up -d mqtt local-node
```

### 3. Verification
- **Dashboard**: `http://localhost:8081`
- **Prometheus**: `http://localhost:9091`
- **MQTT**: `localhost:1884` (TCP) or `8883` (mTLS)

---

## 📊 Monitoring Integration

### Prometheus Scraper
Prometheus must be enrolled as a "Node" in the Manager UI to receive its own mTLS client certificate for scraping.

```yaml
scrape_configs:
  - job_name: 'potbuddy-sites'
    scheme: https
    tls_config:
      ca_file: /etc/prometheus/certs/ca.crt
      cert_file: /etc/prometheus/certs/prometheus.crt
      key_file: /etc/prometheus/certs/prometheus.key
    static_configs:
      - targets: ['site-0.example.com:8080', 'site-1.example.com:8080']

### Grafana Alerting (Discord Webhook)
Provisioning files for Discord notifications are located in `Project/monitoring/grafana/provisioning/alerting/`.

**To deploy to the central Grafana server:**
1. Copy the YAML files to the server:
   ```bash
   scp -r Project/monitoring/grafana/provisioning/alerting root@grafana.example.com:/etc/grafana/provisioning/
   ```
2. Restart Grafana to apply changes:
   ```bash
   ssh root@grafana.example.com "systemctl restart grafana-server"
   ```

The system will now have a "Discord Notifications" contact point available for all alert rules.

---

## 📄 Example Configurations & Auto-Generation

### Auto-Generated via `./enroll`
When deploying Edge components (Method A), the `./enroll` tool automatically generates the required mTLS certificates **and** the correctly formatted configuration files based on the Manager API's response:
- **For Nodes:** Generates `config.yaml` containing the CA endpoint, Site ID, and thresholds.
- **For Brokers:** Generates `mosquitto.conf` pre-configured for port 8883 and strict mTLS verification.

### Repository Examples (Manual Reference)
If you need to manually configure components without the tool, or want to review the expected formats, example configurations are provided in the repository:
- **Environment Variables:** `Project/.env.example`
- **Device Firmware:** `Project/device-firmware/config.example.h`
- **Prometheus Scraper:** `Project/prometheus_local.yml` and `Project/prometheus.yml`

---

## 🛠️ Troubleshooting

| Issue | Likely Cause | Solution |
| :--- | :--- | :--- |
| **mTLS Handshake Failed** | Clock Skew | Run `ntpdate` or ensure `systemd-timesyncd` is active. TLS requires synced time. |
| **Device Rejected** | CN Mismatch | Ensure the `device_id` in the MQTT JSON matches the **Common Name** in the client certificate. |
| **Identity Spoofing Alert** | Security Logic | The Local Node detected a device trying to report data for an ID that doesn't match its certificate. |
| **Status String Mismatch** | Logic Update | Ensure tests/UI handle directional status (e.g., `warning_low` vs generic `warning`). |
| **Stale Thresholds** | DB Pollution | Clear the `threshold_overrides` table or ensure test cleanups run successfully. |
