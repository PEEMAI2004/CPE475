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
| **3. Device & Bootstrap** | | | | |
| TC-DEV-01 | Successful Registration (CA Enabled) | Automated | API | `test_device_bootstrap_lifecycle` |
| TC-DEV-02 | Successful Registration (CA Disabled) | Automated | API | `test_device_bootstrap_lifecycle` |
| TC-DEV-03 | Duplicate Device ID | Automated | API | `test_device_registration_negative` |
| TC-DEV-04 | Empty Device ID | Automated | API | `test_device_registration_negative` |
| TC-DEV-05 | AuthToken Regeneration (Valid) | Automated | API | `test_device_bootstrap_lifecycle` |
| TC-DEV-06 | AuthToken Regeneration (Forbidden) | Automated | API | `test_device_token_regeneration_forbidden` |
| TC-BOOT-01 | Valid Bootstrapping | Automated | API | `test_device_bootstrap_lifecycle` |
| TC-BOOT-02 | Invalid/Fake AuthToken | Automated | API | `test_bootstrap_invalid_token` |
| TC-BOOT-03 | Bootstrapping with CA Disabled | Automated | API | `test_device_bootstrap_lifecycle` |
| **4. MQTT & mTLS** | | | | |
| TC-MTLS-01 | Valid Client Connection | Manual | SIT | - |
| TC-MTLS-02 | No Certificate Provided | Manual | SIT | - |
| TC-MTLS-03 | Invalid/Self-Signed Certificate | Manual | SIT | - |
| TC-MTLS-04 | Identity Mapping | Manual | SIT | - |
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

---

## 1. Authentication & RBAC (Manager API)

### 1.1 Google SSO Login
- **TC-AUTH-01: Valid Login.** Attempt login with a Google account registered in the `users` table. 
  - *Expected:* Success, returns JWT token and user details.
- **TC-AUTH-02: Unregistered User.** Attempt login with a valid Google account NOT in the `users` table. 
  - *Expected:* `403 Forbidden` ("User not invited").
- **TC-AUTH-03: Invalid ID Token.** Send a malformed or expired Google ID token to the `/api/auth/login` endpoint. 
  - *Expected:* `401 Unauthorized`.

### 1.2 Role-Based Access Control (RBAC)
- **TC-RBAC-01: Super Admin Permissions.** Verify Super Admin can access all endpoints (Users, Nodes, Devices, Profiles, Infra).
- **TC-RBAC-02: Site Admin Permissions.** Verify Site Admin can manage Devices and Profiles but is blocked from managing Users or Infrastructure Nodes (`403 Forbidden`).
- **TC-RBAC-03: Viewer Permissions.** Verify Viewer can only GET data (Profiles, Devices, Infra) and is blocked from POST/PUT/DELETE operations (`403 Forbidden`).
- **TC-RBAC-04: Invalid/Expired JWT.** Access any protected endpoint with an expired or tampered JWT. 
  - *Expected:* `401 Unauthorized`.

---

## 2. Infrastructure Enrollment (Edge Nodes)

- **TC-NODE-01: Successful Enrollment.** Register a new node with valid data (Name, Site ID, Address). 
  - *Expected:* `201 Created`, node appears in list, `token` generated.
- **TC-NODE-02: Missing Required Fields.** Attempt to register a node with missing name or address. 
  - *Expected:* `400 Bad Request`.
- **TC-NODE-03: Config Download (Valid).** Download `config.yaml` for an existing node. 
  - *Expected:* Returns YAML file containing the correct `device_id`, `broker` address, and mTLS paths.
- **TC-NODE-04: Config Download (Invalid Node).** Request config for a non-existent node ID. 
  - *Expected:* `404 Not Found`.
- **TC-NODE-05: Node Deletion.** Delete an existing node. 
  - *Expected:* `200 OK`, node removed from the database.
- **TC-NODE-06: Token Regeneration (Valid).** Super Admin regenerates the node token.
  - *Expected:* `200 OK`, returns new `pb_node_` token, database updated.
- **TC-NODE-07: Token Regeneration (Forbidden).** Site Admin or Viewer attempts to regenerate node token.
  - *Expected:* `403 Forbidden`.

---

## 3. IoT Device Enrollment & Bootstrapping

### 3.1 Device Registration
- **TC-DEV-01: Successful Registration (CA Enabled).** Register a device. 
  - *Expected:* Returns `201 Created` with a JSON bundle containing `device_id`, `auth_token`, `ca.crt`, `client.crt`, and `client.key`.
- **TC-DEV-02: Successful Registration (CA Disabled).** Register a device when `ENABLE_CA=false`. 
  - *Expected:* Returns `201 Created` with only `device_id` and `auth_token` (no certs).
- **TC-DEV-03: Duplicate Device ID.** Attempt to register a device with an already existing ID. 
  - *Expected:* `500 Internal Server Error` (Database unique constraint violation).
- **TC-DEV-04: Empty Device ID.** Attempt to register without providing an ID. 
  - *Expected:* `400 Bad Request`.

### 3.2 HTTPS Bootstrapping (`/api/enrollment/bootstrap`)
- **TC-BOOT-01: Valid Bootstrapping.** Device calls endpoint with a valid `AuthToken`. 
  - *Expected:* Returns mTLS certificate bundle.
- **TC-BOOT-02: Invalid/Fake AuthToken.** Device calls endpoint with a random string. 
  - *Expected:* `401 Unauthorized`.
- **TC-BOOT-03: Bootstrapping with CA Disabled.** Device calls endpoint when server is not configured to issue certs. 
  - *Expected:* `503 Service Unavailable`.

---

## 4. MQTT & mTLS Security (Zero-Trust)

- **TC-MTLS-01: Valid Client Connection.** Connect to Mosquitto (port 8883) using the CA, client cert, and private key issued by the Manager API. 
  - *Expected:* Connection established successfully.
- **TC-MTLS-02: No Certificate Provided.** Connect to port 8883 without supplying a client certificate. 
  - *Expected:* Connection rejected by broker (`tlsv1 alert unknown ca` or similar).
- **TC-MTLS-03: Invalid/Self-Signed Certificate.** Connect using a certificate not signed by the Manager API's Root CA. 
  - *Expected:* Connection rejected by broker.
- **TC-MTLS-04: Identity Mapping.** Publish a message and check Mosquitto logs. 
  - *Expected:* Log shows `as <client-id> (..., u'<cert-common-name>')`, proving CN maps to username.

---

## 5. Local Node Processing & Validation

### 5.1 Payload Identity Validation
- **TC-VAL-01: Matching Identity.** Send payload where JSON `device_id` matches the MQTT Topic (which is restricted by the certificate CN). 
  - *Expected:* Payload processed normally.
- **TC-VAL-02: Spoofed Identity (Validation Enabled).** With `VALIDATE_DEVICE_ID=true`, send payload where JSON `device_id` does NOT match the topic. 
  - *Expected:* Payload dropped, `SECURITY ALERT` logged.
- **TC-VAL-03: Spoofed Identity (Validation Disabled).** With `VALIDATE_DEVICE_ID=false`, send spoofed payload. 
  - *Expected:* Payload processed (fallback mode).

### 5.2 Sensor Data Parsing & Enrichment
- **TC-PROC-01: Valid Full Payload.** Send JSON with `light`, `temp`, `hum`, `soil`. 
  - *Expected:* Parsed successfully, pushed to ring buffer, published to `processed` topic.
- **TC-PROC-02: Missing Optional Fields (DHT Error).** Send JSON missing `temp` and `hum`. 
  - *Expected:* Parsed successfully, evaluates available fields, marks missing fields gracefully.
- **TC-PROC-03: Invalid JSON.** Send malformed JSON payload. 
  - *Expected:* Dropped, `parse error` logged.

### 5.3 Health Logic Evaluation (Thresholds)
- **TC-EVAL-01: All Healthy.** All sensor values fall within the `inner_low` and `inner_high` bounds. 
  - *Expected:* Overall status: `healthy`.
- **TC-EVAL-02: Warning Range.** One sensor falls outside `inner` bounds but inside `outer` bounds. 
  - *Expected:* Overall status: `warning`.
- **TC-EVAL-03: Critical Range.** One sensor falls outside the `outer` bounds. 
  - *Expected:* Overall status: `critical`.
- **TC-EVAL-04: Boundary Values.** Test values exactly matching the boundary limits (e.g., exactly `inner_low`). 
  - *Expected:* Handled correctly based on inclusive/exclusive logic operators.

---

## 6. System Infrastructure & Monitoring

- **TC-INFRA-01: Auto-Registration.** Send an MQTT payload for a completely unknown device. 
  - *Expected:* `db.RegisterDevice()` automatically inserts it into the database with the `default` profile.
- **TC-INFRA-02: Online Watchdog.** Device stops sending payloads. 
  - *Expected:* After 30 seconds, Prometheus metric `potbuddy_device_online` drops to 0, device appears offline in Dashboard.
- **TC-INFRA-03: Service Health Check.** Hit the `/api/infrastructure` endpoint. 
  - *Expected:* Returns real-time TCP/HTTP health status for DB, Manager, Scrapers, and registered Edge Nodes.
- **TC-INFRA-04: Database Polling.** Update a profile threshold in the database. 
  - *Expected:* Local node's caching loop picks up the change within 1 minute and applies it to incoming payloads automatically.

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
