# PotBuddy Comprehensive Test Cases

This document outlines the test cases designed to cover all features and edge cases of the PotBuddy IoT Plant Care system.

## 📋 Test Summary Table

| Number | Test Case | Status | Type | Automated testcase name |
| :--- | :--- | :--- | :--- | :--- |
| **1. Auth & RBAC** | | | | |
| TC-AUTH-01 | Valid Google Login | Manual | UI | - |
| TC-AUTH-02 | Unregistered User | Manual | UI | - |
| TC-AUTH-03 | Invalid ID Token | Automated | API | `test_auth_invalid_id_token` |
| TC-RBAC-01 | Super Admin Permissions | Automated | API | `test_rbac_super_admin_access` |
| TC-RBAC-02 | Site Admin Permissions | Automated | API | `test_rbac_site_admin_permissions` |
| TC-RBAC-03 | Viewer Permissions | Automated | API | `test_rbac_viewer_permissions` |
| TC-RBAC-04 | Invalid/Expired JWT | Automated | API | `test_rbac_invalid_token` |
| **2. Node Enrollment** | | | | |
| TC-NODE-01 | Successful Enrollment | Automated | API | `test_infrastructure_node_lifecycle` |
| TC-NODE-02 | Missing Required Fields | Automated | API | `test_node_enrollment_negative` |
| TC-NODE-03 | Config Download (Valid) | Automated | API | `test_infrastructure_node_lifecycle` |
| TC-NODE-04 | Config Download (Invalid Node) | Automated | API | `test_node_enrollment_negative` |
| TC-NODE-05 | Node Deletion | Automated | API | `test_infrastructure_node_lifecycle` |
| TC-NODE-06 | Token Regeneration (Valid) | Automated | API | `test_infrastructure_node_lifecycle` |
| TC-NODE-07 | Token Regeneration (Forbidden) | Automated | API | `test_node_token_regeneration_forbidden` |
| TC-NODE-08 | CSR-based Enrollment (No Escrow) | Automated | API | `test_csr_node_enrollment` |
| **3. Device & Bootstrap** | | | | |
| TC-DEV-01 | Successful Registration (CA Enabled) | Automated | API | `test_device_bootstrap_lifecycle` |
| TC-DEV-02 | Successful Registration (CA Disabled) | Automated | API | `test_device_bootstrap_lifecycle` |
| TC-DEV-03 | Duplicate Device ID | Automated | API | `test_device_registration_negative` |
| TC-DEV-04 | Empty Device ID | Automated | API | `test_device_registration_negative` |
| TC-DEV-05 | AuthToken Regeneration (Valid) | Automated | API | `test_device_bootstrap_lifecycle` |
| TC-DEV-06 | AuthToken Regeneration (Forbidden) | Automated | API | `test_device_token_regeneration_forbidden` |
| TC-DEV-07 | CSR-based Registration (No Escrow) | Automated | API | `test_csr_device_enrollment` |
| TC-BOOT-01 | Valid Bootstrapping | Automated | API | `test_device_bootstrap_lifecycle` |
| TC-BOOT-02 | Invalid/Fake AuthToken | Automated | API | `test_bootstrap_invalid_token` |
| TC-BOOT-03 | Bootstrapping with CA Disabled | Automated | API | `test_device_bootstrap_lifecycle` |
| TC-BOOT-04 | Token-based CSR Bootstrapping | Automated | CLI | Manual `enroll` Verification |
| **4. MQTT & mTLS** | | | | |
| TC-MTLS-01 | Valid Client Connection | Manual | SIT | - |
| TC-MTLS-02 | No Certificate Provided | Manual | SIT | - |
| TC-MTLS-03 | Invalid/Self-Signed Certificate | Manual | SIT | - |
| TC-MTLS-04 | Identity Mapping | Manual | SIT | - |
| TC-MTLS-05 | Token-based MQTT Bootstrapping | Automated | CLI | Manual `enroll` Verification |
| **5. Processing & Validation**| | | | |
| TC-VAL-01 | Matching Identity | Automated | SIT | `test_sit_identity_spoof_detection` |
| TC-VAL-02 | Spoofed Identity (Valid. Enabled) | Automated | SIT | `test_sit_identity_spoof_detection` |
| TC-VAL-03 | Spoofed Identity (Valid. Disabled) | Manual | SIT | - |
| TC-PROC-01 | Valid Full Payload | Automated | SIT | `test_sit_health_grading_flow` |
| TC-PROC-02 | Missing Optional Fields | Manual | SIT | - |
| TC-PROC-03 | Invalid JSON | Manual | SIT | - |
| TC-EVAL-01 | All Healthy | Automated | SIT | `test_sit_health_grading_flow` |
| TC-EVAL-02 | Warning Range | Automated | SIT | `test_sit_health_grading_flow` |
| TC-EVAL-03 | Critical Range | Automated | SIT | `test_sit_health_grading_flow` |
| TC-EVAL-04 | Boundary Values | Manual | SIT | - |
| **6. Infra & Monitoring** | | | | |
| TC-INFRA-01 | Auto-Registration | Automated | SIT | `test_sit_auto_registration` |
| TC-INFRA-02 | Online Watchdog | Automated | SIT | `test_sit_online_watchdog` |
| TC-INFRA-03 | Service Health Check | Automated | API | `test_system_infrastructure_health` |
| TC-INFRA-04 | Database Polling | Automated | SIT | `test_sit_database_polling` |
| **7. Cross-Environment Execution** | | | | |
| TC-ENV-01 | Local Docker Compose | Automated | ALL | Supported via Default Config |
| TC-ENV-02 | Remote Infrastructure | Automated | ALL | Supported via Env Overrides |
| **8. End-to-End Lifecycle** | | | | |
| TC-E2E-01 | Node Lifecycle E2E | Manual | CLI/UI | - |

---

## 1. Authentication & RBAC (Manager API)

### 1.1 Google SSO Login
- **TC-AUTH-01: Valid Login.**
  1. Open the web dashboard login page.
  2. Click "Login with Google".
  3. Authenticate with an account present in the `users` table.
  - *Expected:* Successful login, redirects to dashboard.
- **TC-AUTH-02: Unregistered User.**
  1. Open the web dashboard login page.
  2. Click "Login with Google".
  3. Authenticate with a valid Google account NOT present in the `users` table.
  - *Expected:* `403 Forbidden` error shown.
- **TC-AUTH-03: Invalid ID Token.**
  1. Send POST request to `/api/auth/login` with a malformed `idToken`.
  - *Expected:* `401 Unauthorized`.

### 1.2 Role-Based Access Control (RBAC)
- **TC-RBAC-01: Super Admin Permissions.**
  1. Authenticate as Super Admin.
  2. Access `/users`, `/profiles`, `/devices`, `/infrastructure`, `/enrollment/nodes`, `/enrollment/devices`.
  - *Expected:* All endpoints return `200 OK`.
- **TC-RBAC-02: Site Admin Permissions.**
  1. Authenticate as Site Admin.
  2. Access `/devices`, `/profiles`.
  3. Access `/users`, `/enrollment/nodes`.
  - *Expected:* Step 2 returns `200 OK`, Step 3 returns `403 Forbidden`.
- **TC-RBAC-03: Viewer Permissions.**
  1. Authenticate as Viewer.
  2. Access `/devices`, `/profiles`.
  3. Access `/users`, `/enrollment/nodes`.
  4. Attempt POST to `/enrollment/devices`.
  - *Expected:* Step 2 returns `200 OK`, Step 3 & 4 return `403 Forbidden`.
- **TC-RBAC-04: Invalid/Expired JWT.**
  1. Send request to protected endpoint with an invalid or expired token.
  - *Expected:* `401 Unauthorized`.

---

## 2. Infrastructure Enrollment (Edge Nodes)

- **TC-NODE-01: Successful Enrollment.**
  1. POST to `/api/enrollment/nodes` with valid `name`, `site_id`, `address`.
  - *Expected:* `201 Created`, returns node token.
- **TC-NODE-02: Missing Required Fields.**
  1. POST to `/api/enrollment/nodes` with missing fields.
  - *Expected:* `400 Bad Request`.
- **TC-NODE-03: Config Download (Valid).**
  1. GET `/api/enrollment/nodes/{id}/config`.
  - *Expected:* Returns bundle containing `config.yaml` and mTLS certificates.
- **TC-NODE-04: Config Download (Invalid Node).**
  1. GET `/api/enrollment/nodes/99999/config`.
  - *Expected:* `404 Not Found`.
- **TC-NODE-05: Node Deletion.**
  1. DELETE `/api/enrollment/nodes/{id}`.
  - *Expected:* `200 OK`, node removed from DB.
- **TC-NODE-06: Token Regeneration (Valid).**
  1. POST to `/api/enrollment/nodes/{id}/regen-token`.
  - *Expected:* `200 OK`, returns a new token.
- **TC-NODE-07: Token Regeneration (Forbidden).**
  1. Authenticate as Site Admin.
  2. Attempt POST to `/api/enrollment/nodes/{id}/regen-token`.
  - *Expected:* `403 Forbidden`.
- **TC-NODE-08: CSR-based Enrollment (No Escrow).**
  1. POST to `/api/enrollment/nodes/{id}/client-cert` with a valid CSR.
  - *Expected:* `200 OK`, returns signed certificate without private key.

---

## 3. IoT Device Enrollment & Bootstrapping

### 3.1 Device Registration
- **TC-DEV-01: Successful Registration (CA Enabled).**
  1. POST to `/api/enrollment/devices` with `device_id`.
  - *Expected:* `201 Created`, returns full certificate bundle.
- **TC-DEV-02: Successful Registration (CA Disabled).**
  1. Set `ENABLE_CA=false`.
  2. POST to `/api/enrollment/devices`.
  - *Expected:* `201 Created`, returns `auth_token` only.
- **TC-DEV-03: Duplicate Device ID.**
  1. POST twice with same `device_id`.
  - *Expected:* `500 Internal Server Error` (unique constraint).
- **TC-DEV-04: Empty Device ID.**
  1. POST with empty `device_id`.
  - *Expected:* `400 Bad Request`.
- **TC-DEV-05: AuthToken Regeneration (Valid).**
  1. POST to `/api/enrollment/devices/{id}/regen-token`.
  - *Expected:* `200 OK`, returns new token.
- **TC-DEV-06: AuthToken Regeneration (Forbidden).**
  1. Authenticate as Viewer.
  2. POST to `/api/enrollment/devices/{id}/regen-token`.
  - *Expected:* `403 Forbidden`.
- **TC-DEV-07: CSR-based Registration (No Escrow).**
  1. POST to `/api/enrollment/devices` with `device_id` and a CSR.
  - *Expected:* `201 Created`, returns signed certificate without private key.

### 3.2 HTTPS Bootstrapping (`/api/enrollment/bootstrap`)
- **TC-BOOT-01: Valid Bootstrapping.**
  1. POST to `/api/enrollment/bootstrap` with valid `auth_token`.
  - *Expected:* `200 OK`, returns certificate bundle.
- **TC-BOOT-02: Invalid/Fake AuthToken.**
  1. POST with invalid `auth_token`.
  - *Expected:* `401 Unauthorized`.
- **TC-BOOT-03: Bootstrapping with CA Disabled.**
  1. Disable CA.
  2. POST to bootstrap endpoint.
  - *Expected:* `503 Service Unavailable`.
- **TC-BOOT-04: Token-based CSR Bootstrapping.**
  1. Create a node in the dashboard to obtain a site token.
  2. Run `./enroll -token pb_node_...` on an edge server.
  - *Expected:* Local private key generated, CSR submitted via `X-PotBuddy-Token` header, signed certificate and customized `config.yaml` returned and saved locally.

---

## 4. MQTT & mTLS Security (Zero-Trust)

- **TC-MTLS-01: Valid Client Connection.**
  1. Connect to port 8883 with valid CA-signed cert/key.
  - *Expected:* Successful mTLS handshake and connection.
- **TC-MTLS-02: No Certificate Provided.**
  1. Attempt connection to 8883 without client certificate.
  - *Expected:* Connection rejected by broker.
- **TC-MTLS-03: Invalid/Self-Signed Certificate.**
  1. Connect with certificate not signed by the Manager CA.
  - *Expected:* Connection rejected by broker.
- **TC-MTLS-04: Identity Mapping.**
  1. Connect with cert `CN=device-01`.
  2. Verify broker logs show mapping to identity `device-01`.
  - *Expected:* CN is correctly mapped to the session identity.
- **TC-MTLS-05: Token-based MQTT Bootstrapping.**
  1. Obtain a site token from the dashboard.
  2. Run `./enroll -type mqtt -token pb_node_... -cn BROKER_IP`.
  - *Expected:* Local `server.key` generated; signed `server.crt` and `mosquitto.conf` returned and saved locally.

---

## 5. Local Node Processing & Validation

### 5.1 Payload Identity Validation
- **TC-VAL-01: Matching Identity.**
  1. Publish to `potbuddy/dev-01/raw` with JSON `{"device_id": "dev-01"}`.
  - *Expected:* Payload accepted and processed.
- **TC-VAL-02: Spoofed Identity (Validation Enabled).**
  1. Set `VALIDATE_DEVICE_ID=true`.
  2. Publish to `potbuddy/dev-01/raw` with JSON `{"device_id": "attacker"}`.
  - *Expected:* Payload dropped; SECURITY ALERT logged.
- **TC-VAL-03: Spoofed Identity (Validation Disabled).**
  1. Set `VALIDATE_DEVICE_ID=false`.
  2. Publish with mismatched ID.
  - *Expected:* Payload processed (identity check bypassed).

### 5.2 Sensor Data Parsing & Enrichment
- **TC-PROC-01: Valid Full Payload.**
  1. Publish JSON with `light`, `temp`, `hum`, `soil`.
  - *Expected:* Parsed successfully; status updated.
- **TC-PROC-02: Missing Optional Fields.**
  1. Publish JSON missing `temp` and `hum`.
  - *Expected:* Parsed successfully; missing fields marked as `null`.
- **TC-PROC-03: Invalid JSON.**
  1. Publish malformed JSON string.
  - *Expected:* Payload dropped; parse error logged.

### 5.3 Health Logic Evaluation (Thresholds)
- **TC-EVAL-01: All Healthy.**
  1. Send sensor data within all healthy ranges.
  - *Expected:* Overall status: `healthy`.
- **TC-EVAL-02: Warning Range.**
  1. Send sensor data in warning range.
  - *Expected:* Overall status: `warning`.
- **TC-EVAL-03: Critical Range.**
  1. Send sensor data in critical range.
  - *Expected:* Overall status: `critical`.
- **TC-EVAL-04: Boundary Values.**
  1. Send sensor data exactly at threshold boundary.
  - *Expected:* Evaluated correctly based on logic operators.

---

## 6. System Infrastructure & Monitoring

- **TC-INFRA-01: Auto-Registration.**
  1. Publish data for an unknown device ID.
  - *Expected:* Device automatically appears in Manager API device list.
- **TC-INFRA-02: Online Watchdog.**
  1. Send data to mark device online.
  2. Stop sending data for 30+ seconds.
  - *Expected:* Metrics/API show device as offline.
- **TC-INFRA-03: Service Health Check.**
  1. GET `/api/infrastructure`.
  - *Expected:* Returns list of services with "online" status.
- **TC-INFRA-04: Database Polling.**
  1. Update profile threshold in DB.
  2. Wait 60s for node poller.
  3. Verify new threshold applies to incoming data.
  - *Expected:* Node uses updated thresholds without restart.

---

## 7. Cross-Environment Execution

The PotBuddy test suite is designed to run interchangeably against local development environments and remote production-like infrastructure.

### 7.1 Local Docker Compose
By default, the tests target `localhost` and the internal Docker port mapping.
```bash
cd Project/
./test_venv/bin/pytest -v test_api.py test_sit.py
```

### 7.2 Remote Infrastructure Execution
To target a remote deployment, override the environment variables. The SIT tests automatically switch from `docker exec` to `ssh` for database manipulations based on the `BASE_URL` or `REMOTE_TEST` flag.

#### **Targeting Site 0 (Default)**
```bash
REMOTE_TEST=true \
BASE_URL=http://manager.iot.kaminjitt.com:8081/api \
MQTT_HOST=mqtt-0.iot.kaminjitt.com \
NODE_API=https://debian-0.iot.kaminjitt.com:8080/history \
METRICS_API=https://debian-0.iot.kaminjitt.com:8080/metrics \
./test_venv/bin/pytest -v test_api.py test_sit.py
```

#### **Targeting Site 1 (Other Node)**
To test against a different site, simply update the `MQTT_HOST`, `NODE_API`, and `METRICS_API` domains:
```bash
REMOTE_TEST=true \
BASE_URL=http://manager.iot.kaminjitt.com:8081/api \
MQTT_HOST=mqtt-1.iot.kaminjitt.com \
NODE_API=https://debian-1.iot.kaminjitt.com:8080/history \
METRICS_API=https://debian-1.iot.kaminjitt.com:8080/metrics \
./test_venv/bin/pytest -v test_api.py test_sit.py
```

**Required Overrides:**
- `BASE_URL`: URL of the central Manager API.
- `MQTT_HOST`: Domain of the site-specific Mosquitto broker.
- `NODE_API`: URL of the site-specific Edge Processor (`/history`).
- `METRICS_API`: URL of the site-specific Edge Processor (`/metrics`).
- `REMOTE_TEST`: Set to `true` to enable SSH-based DB updates (targets the central DB server at `10.0.0.66`).
- `CA_CERT`, `CLIENT_CERT`, `CLIENT_KEY`: Paths to the Root CA and client identity bundle valid for the target site.

---

## 8. End-to-End Lifecycle

- **TC-E2E-01: Node Lifecycle E2E.**
  1. **Infrastructure Creation:** Create a new "Local Node" in the Manager UI to obtain a site token.
  2. **Initial Enrollment:**
     - Run `./enroll -token <TOKEN>` to enroll the **local-node**.
     - Run `./enroll -type mqtt -token <TOKEN> -cn localhost` to enroll the **MQTT broker**.
     - *Expected:* Success for both; certificates (`client.*` for node, `server.*` for broker) and configs (`config.yaml`, `mosquitto.conf`) generated.
  3. **Deployment & Connectivity Verification:**
     - Start the **MQTT broker** using the generated `mosquitto.conf`.
     - Start the **local-node** using the generated `config.yaml`.
     - Verify **local-node** successfully connects/subscribes to the local **MQTT broker** via mTLS.
     - Bootstrap and verify a new **device** using the mock script:
       ```bash
       # 1. Register device to get token
       TOKEN=$(curl -s -X POST http://localhost:8081/api/enrollment/devices \
         -H "Authorization: Bearer <ADMIN_TOKEN>" \
         -H "Content-Type: application/json" \
         -d '{"device_id": "e2e-mock-01"}' | grep -oP '"auth_token":"\K[^"]+')
       
       # 2. Run mock script to bootstrap, validate certs, and test mTLS pub
       python3 mock_device_pub.py --token $TOKEN --id e2e-mock-01
       ```
     - Verify the message is processed by the **local-node**.
     - *Expected:* Full end-to-end mTLS connectivity and data flow.
  4. **Token Refresh:** In the Manager UI, click "Regenerate Token" for the node.
  5. **Old Token Rejection:**
     - Attempt to enroll a **local-node** using the previous token.
     - Attempt to enroll an **MQTT broker** using the previous token.
     - *Expected:* Failure (`401 Unauthorized`) for both.
  6. **New Token Acceptance & Deployment:**
     - Attempt to enroll a **local-node** using the newly generated token.
     - Attempt to enroll an **MQTT broker** using the newly generated token.
     - *Expected:* Success for both; new certificates generated.
     - **Deploy New Certificates:** 
       - Update the **local-node** with the new `client.crt` and `client.key`.
       - Update the **MQTT broker** with the new `server.crt` and `server.key`.
       - Restart both services.
     - **Verify Connectivity:** 
       - Ensure the **local-node** successfully reconnects to the **MQTT broker** using its new identity.
       - Use the **Existing device certificate** (from step 3) to publish a message to the broker using `mock_device_pub.py`.
       - *Expected:* Success; existing devices remain operational after infrastructure certificate rotation.
  7. **Infrastructure Deletion:** Delete the node in the Manager UI.
  8. **Post-Deletion Rejection:**
     - Attempt to enroll a **local-node** using the (now deleted) token.
     - Attempt to enroll an **MQTT broker** using the (now deleted) token.
     - *Expected:* Failure (`401 Unauthorized`) for both.
