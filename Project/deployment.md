# PotBuddy Deployment Guide

PotBuddy supports two main avenues of deployment: **Local Docker Compose** (great for testing or single-machine environments) and **Multi-Site Distributed Linux Services** (designed for scale and independent site processors).

Choose the pathway that fits your environment below:

---

## Method A: Distributed Linux Node Deployment (Non-Docker)

This is the recommended deployment for a truly decoupled edge-cloud architecture, running directly as `systemd` background services.

### 1. Prerequisites
- **A Central Database Server**: PostgreSQL 17 exposed.
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
1. Create a `potbuddy-manager.service` file ensuring `Environment=DB_DSN=postgres://...` points to your central database.
2. Upload the `manager-linux` binary to `/opt/potbuddy/manager-api/manager` and the `frontend/dist/` folder adjacent to it.
3. Install and run the service:
```bash
systemctl daemon-reload
systemctl enable --now potbuddy-manager
systemctl status potbuddy-manager
```

### 4. Edge Processor Deployment
These servers intercept local MQTT devices and relay them.
1. Create customized config files (`config-0.yaml`, `config-1.yaml`) where `local_mqtt.broker` points to the site's local broker (e.g., `mqtt-0.iot.kaminjitt.com`).
2. Upload `node-linux` and the mapped config file to `/opt/potbuddy/local-node/`.
3. Configure `potbuddy.service` to utilize this path.
4. Run the service:
```bash
systemctl daemon-reload
systemctl enable --now potbuddy
```

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

This single command spins up:
1. `eclipse-mosquitto` broker on Port `1884`.
2. `postgres:17-alpine` on Port `5433` (pre-seeded with `init.sql`).
3. The `local-node` Go processor container.
4. The `manager-api` container listening on Port `8081`.

### 3. Teardown
```bash
docker compose down -v
```
