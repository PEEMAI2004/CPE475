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

This is the recommended deployment for a truly decoupled edge-cloud architecture, running as `systemd` services.

### 1. Central Infrastructure (Cloud)
- **Database**: Install PostgreSQL 17. Run `psql -f init.sql potbuddy` to initialize.
- **Manager API**: 
  - Build: `cd manager-api && go build -o manager ./cmd/manager`
  - Run: Ensure `DB_DSN` and `ENABLE_CA=true` are set.
  - Setup: Log in via Google SSO and invite site administrators.

### 2. Edge Site Setup (Repeat for Site 0...N)
Each physical site requires a local MQTT broker and a Go processor node.

**Step 1: Enrollment (Zero-Trust)**
On the site server, use the `enroll` tool to generate keys locally (private keys never leave the server).
1. **Enroll Local Node**:
   ```bash
   ./enroll -token <SITE_TOKEN> -manager https://manager.iot.kaminjitt.com
   ```
   *Generates:* `client.key`, `client.crt`, `ca.crt`, `config.yaml`.

2. **Enroll MQTT Broker**:
   ```bash
   ./enroll -type mqtt -token <SITE_TOKEN> -cn <BROKER_DOMAIN_OR_IP>
   ```
   *Generates:* `server.key`, `server.crt`, `mosquitto.conf`.

**Step 2: Deployment**
- **Mosquitto**: Move certs to `/etc/mosquitto/certs/`. Apply `mosquitto.conf`. Restart service.
- **Local Node**: Ensure `config.yaml` and certs are in the working directory. Start the `node` binary.

---

## Method B: Docker Deployment (Development)

Best for testing everything on a single machine.

### 1. Configuration
Check `Project/.env` to ensure `GOOGLE_CLIENT_ID` and `JWT_SECRET` are set correctly for local testing.

### 2. Startup
```bash
cd Project/
docker compose up --build -d
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
```

---

## 🛠️ Troubleshooting

| Issue | Likely Cause | Solution |
| :--- | :--- | :--- |
| **mTLS Handshake Failed** | Clock Skew | Run `ntpdate` or ensure `systemd-timesyncd` is active. TLS requires synced time. |
| **Device Rejected** | CN Mismatch | Ensure the `device_id` in the MQTT JSON matches the **Common Name** in the client certificate. |
| **Identity Spoofing Alert** | Security Logic | The Local Node detected a device trying to report data for an ID that doesn't match its certificate. |
