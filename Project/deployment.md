# PotBuddy Deployment Guide

PotBuddy supports two main avenues of deployment: **Local Docker Compose** (great for testing or single-machine environments) and **Multi-Site Distributed Linux Services** (designed for scale and independent site processors).

Choose the pathway that fits your environment below:

---

## 🌐 Networking & Port Map

For a distributed deployment, ensure the following ports are open in your firewalls:

| Component | Port | Protocol | Purpose |
| :--- | :---: | :--- | :--- |
| **Edge Site** | 8883 | TCP/mTLS | MQTT Device/Node Communication |
| **Edge Site** | 8080 | TCP/mTLS | Metrics Scraping (Prometheus) |
| **Manager API** | 8081 | TCP/HTTPS | Web Dashboard & Enrollment |
| **Database** | 5432 | TCP | Central Persistence (PostgreSQL) |
| **Monitoring** | 9090 | TCP | Prometheus Web Interface |
| **Monitoring** | 3000 | TCP | Grafana Visualization |

---

## Method A: Distributed Linux Node Deployment (Non-Docker)

This is the recommended deployment for a truly decoupled edge-cloud architecture, running directly as `systemd` background services.

### 1. Prerequisites
- **A Central Database Server**: PostgreSQL 17 exposed.
- **Database Initialization**: Run `psql -h <host> -U <user> -f init.sql potbuddy` to set up the schema before starting the Manager API.
- **A Central Manager Server**: Runs the API & Web Dashboard.
- **Edge Site Servers (`debian-*`)**: Local network gateways.
- **Local MQTT Brokers (`mqtt-*`)**: Dedicated Mosquitto brokers per site.

### 2. Compilation
Compile all Go binaries using your local Linux machine to match your target architectures:

```bash
# Compile Manager Backend
cd manager-api 
GOOS=linux GOARCH=amd64 go build -o bin/manager-linux main.go 

# Compile Local Node (Processor)
cd ../local-node 
GOOS=linux GOARCH=amd64 go build -o bin/node-linux ./cmd/node 

# Compile React Frontend
cd ../frontend 
npm install && npm run build
```

### 3. Manager API Deployment
This server serves the configuration and dashboard.

**Environment Variables:**
| Variable | Description |
| :--- | :--- |
| `DB_DSN` | Connection string for PostgreSQL (e.g., `postgres://user:pass@host:5432/potbuddy?sslmode=disable`). |
| `PORT` | Listening port for the API and static frontend (Default: `8081`). |

1. Create a `potbuddy-manager.service` file ensuring `DB_DSN` points to your central database.
2. Upload the `manager-linux` binary to `/opt/potbuddy/manager-api/manager` and the `frontend/dist/` folder adjacent to it.
3. Install and run the service:
```bash
systemctl daemon-reload
systemctl enable --now potbuddy-manager
systemctl status potbuddy-manager
```

### 4. Edge Processor Deployment
These servers intercept local MQTT devices and relay them.

**Enrollment (Zero-Trust CSR):**
To ensure zero-trust security, private keys are generated locally at the edge and never leave the server.
1. **Compile Enrollment Tool**:
   ```bash
   cd local-node
   go build -o bin/enroll ./cmd/enroll
   ```
2. **Perform Enrollment**:
   Obtain the **Site Token** from the Manager UI. Run the enrollment tool on the target edge server:
   ```bash
   ./bin/enroll -token <SITE_TOKEN> -url http://<MANAGER_IP>:8081
   ```
   This command generates `client.key` (locally), `client.csr`, and downloads the signed `client.crt`, Root `ca.crt`, and a customized `config.yaml`.

3. **Deploy**: Move the generated files to `/opt/potbuddy/local-node/`.
4. **Run**: Start the `node-linux` binary as a `systemd` service. It will now serve HTTPS on port 8080 with mandatory mTLS.

### 5. Local MQTT Broker Deployment (Secure mTLS)
To secure the local site broker, you must enroll it as a Server and use the `enroll` tool to obtain its signed identity.

1. **Enroll Broker**: In the Manager UI, ensure the site's **MQTT Address** is set correctly.
2. **Run Enrollment Tool**: On the broker server, run the enrollment tool with `-type mqtt`:
   ```bash
   ./bin/enroll -type mqtt -token <SITE_TOKEN> -cn <BROKER_DOMAIN_OR_IP> -url http://<MANAGER_IP>:8081
   ```
   This generates `server.key` (locally), `server.csr`, and downloads:
   - `server.crt` (Signed Server Certificate with SANs)
   - `ca.crt` (Root CA)
   - `mosquitto.conf` (Pre-configured mTLS template)

3. **Configure Mosquitto**:
   - Move the certificates to `/etc/mosquitto/certs/`.
   - Apply the `mosquitto.conf` template to your configuration directory.
   - Restart the service: `sudo systemctl restart mosquitto`.

4. **Verify**: Ensure the Local Node can connect to this broker using its own mTLS client certificate (obtained in Step 4).

---

## Method B: Docker Deployment (Single Machine/Development)

Docker is the best choice if you want to test everything on a single laptop without configuring Linux dependencies or managing `systemd`. 

### Prerequisites
- Docker & Docker Compose installed.

### 1. Configure the Environment
Ensure your `.env` or configurations in `docker-compose.yml` point realistically to internal Docker endpoints:
- **Broker**: `tcp://mqtt:1883`
- **Database**: `postgres://postgres:postgres@postgres:5432`

### 2. Startup
You don't need to manually compile anything. Docker handles building the isolated containers.
```bash
cd Project/
docker compose up --build -d
```

### 3. Teardown
```bash
docker compose down -v
```

## 📊 Monitoring Infrastructure

PotBuddy uses a centralized monitoring stack to aggregate metrics from all distributed edge sites.

### 1. Prometheus Deployment
Prometheus acts as the central scraper. It is typically deployed in the cloud or a central management site.

1. **Installation**: Install Prometheus using your package manager or Docker.
2. **Identity**: Enroll Prometheus as a "Node" in the Manager Dashboard and use the `enroll` tool to generate its unique mTLS credentials (`ca.crt`, `prometheus.crt`, `prometheus.key`).
3. **Configuration**: Mount these certificates into `/etc/prometheus/certs/` and update `prometheus.yml`:
   ```yaml
   scrape_configs:
     - job_name: 'potbuddy-local-node'
       scheme: https
       tls_config:
         ca_file: /etc/prometheus/certs/ca.crt
         cert_file: /etc/prometheus/certs/prometheus.crt
         key_file: /etc/prometheus/certs/prometheus.key
       static_configs:
         - targets: ['site-0.example.com:8080', 'site-1.example.com:8080']
   ```

### 2. Grafana Deployment
Grafana provides the visual dashboard for plant health and system infrastructure.

1. **Installation**: Deploy Grafana (port 3000) and connect it to your Prometheus instance as a Data Source.
2. **Dashboard Import**: Import the PotBuddy dashboard JSON (if provided) or create a new one using the `potbuddy_*` metrics.
3. **Health Integration**: Ensure Grafana is accessible at the URL configured in the Manager API (`grafana.iot.kaminjitt.com`) to enable integrated health checks in the Manager UI.

---

## 🧪 Deployment Verification

After deployment, you can verify the system integrity using the provided test suite:

1. **API Integration**: Run `pytest test_api.py` from the project root. Ensure `BASE_URL` in the script points to your Manager API.
2. **MQTT Flow**: Use `python3 mqtt_test_helper.py` to simulate an mTLS-authenticated device. This verifies that the Broker, Local Node, and Database are communicating correctly.

---

## 🛠️ Troubleshooting

| Issue | Likely Cause | Solution |
| :--- | :--- | :--- |
| **mTLS Handshake Failed** | Clock Skew | Ensure all Edge devices and Nodes are synced via **NTP** (e.g., `pool.ntp.org`). Certificates will fail if the system time is behind the "Not Before" date. |
| **Hostname Mismatch** | Invalid SANs | Ensure Broker certificates include the correct IP or Domain Name in the Subject Alternative Names (SANs) field during enrollment. |
| **Connection Refused** | Firewall | Verify that port `8883` (Edge) or `5432` (Cloud) is open and listeners are bound to `0.0.0.0`. |
| **"Identity Spoofing" Alert** | CN Mismatch | The Local Node logged a security alert because a device connected with a valid certificate but tried to report data for a different `device_id`. |

---

## 📈 Scaling: Adding More Site Nodes

PotBuddy is designed to support an unlimited number of physical sites. To add a new Edge Processor (Site Node):

1.  **New Configuration**: Create a new config file (e.g., `local-node/config-2.yaml`) specifying the site's unique local MQTT broker and mTLS credentials.
2.  **Edge Deployment**: Deploy the `node` binary and its unique certificate bundle to the new site's gateway server.
3.  **Prometheus Integration**: Update the central `prometheus.yml` to include the new node's IP/Hostname. **Note:** Prometheus must use a valid client certificate to scrape these nodes.
4.  **Hardware Alignment**: Ensure the ESP32 devices at the new site are configured to publish to the local broker defined in step 1.

The new node will automatically fetch all existing Plant Health Profiles from the central database upon startup.
