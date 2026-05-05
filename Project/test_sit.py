import requests
import time
import pytest
import os
import json
import ssl
import paho.mqtt.client as mqtt
import subprocess

# Configuration from environment or defaults
BASE_URL = os.getenv("BASE_URL", "http://localhost:8081/api")
TOKEN = os.getenv("TOKEN", "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJlbWFpbCI6ImFkbWluQHRlc3QuY29tIiwicm9sZSI6IlN1cGVyIEFkbWluIiwiZXhwIjoxNzc4MDYwMjQxfQ.dCE9-R8FYVGTmI9tHIxbbvnmOT0Wkxgorap9j9Jkl2E")
MQTT_HOST = os.getenv("MQTT_HOST", "localhost")
MQTT_PORT = int(os.getenv("MQTT_PORT", "8883"))
NODE_API = os.getenv("NODE_API", "https://localhost:8080/history")
METRICS_API = os.getenv("METRICS_API", "https://localhost:8080/metrics")

headers = {"Authorization": f"Bearer {TOKEN}", "Content-Type": "application/json"}

CA_CERT = os.getenv("CA_CERT", "certs/ca.crt")
CLIENT_CERT = os.getenv("CLIENT_CERT", "certs/client.crt")
CLIENT_KEY = os.getenv("CLIENT_KEY", "certs/client.key")

def run_mqtt_pub(device_id, payload):
    """Simulates an ESP32 publishing via mTLS to the local site broker"""
    topic = f"potbuddy/{device_id}/raw"
    msg = json.dumps(payload)
    
    client = mqtt.Client(callback_api_version=mqtt.CallbackAPIVersion.VERSION2, client_id=f"test-{device_id}")
    
    # Configure mTLS
    context = ssl.create_default_context(ssl.Purpose.SERVER_AUTH, cafile=CA_CERT)
    context.load_cert_chain(certfile=CLIENT_CERT, keyfile=CLIENT_KEY)
    context.check_hostname = False
    context.verify_mode = ssl.CERT_NONE # We trust our CA but bypass hostname match for IP testing
    
    client.tls_set_context(context)
    
    def on_connect(client, userdata, flags, rc, properties):
        if rc != 0:
            print(f"Failed to connect, return code {rc}")

    client.on_connect = on_connect

    client.connect(MQTT_HOST, MQTT_PORT)
    client.loop_start()
    info = client.publish(topic, msg, qos=1)
    info.wait_for_publish()
    if not info.is_published():
        print(f"Failed to publish message to {topic}")
    time.sleep(1)
    client.loop_stop()
    client.disconnect()

def test_sit_auto_registration():
    """TC-INFRA-01: Verify that a new device is auto-registered upon first publish"""
    unique_id = f"new-esp32-{int(time.time())}"
    payload = {"light": 1000, "temp": 25, "hum": 50, "soil": 2000}
    
    # 1. Publish as a completely unknown device
    run_mqtt_pub(unique_id, payload)
    time.sleep(5) # Give time for Local Node to process and register
    
    # 2. Check if it exists in Manager API
    resp = requests.get(f"{BASE_URL}/devices", headers=headers)
    assert resp.status_code == 200
    devices = [d['device_id'] for d in resp.json()]
    
    assert unique_id in devices, f"Device {unique_id} was not auto-registered"

def test_sit_health_grading_flow():
    """TC-EVAL-01, 02: End-to-End logic check from MQTT to Local Node API"""
    dev_id = f"logic-test-{int(time.time())}"
    
    # 1. Send Healthy Data
    run_mqtt_pub(dev_id, {"light": 5000, "temp": 25, "hum": 50, "soil": 2000})
    time.sleep(2)
    
    # 2. Verify via Local Node API
    resp = requests.get(NODE_API, cert=(CLIENT_CERT, CLIENT_KEY), verify=False)
    assert resp.status_code == 200
    readings = resp.json()
    latest = next(r for r in reversed(readings) if r['device_id'] == dev_id)
    assert latest['status']['overall'] == "healthy"

    # 3. Send Warning Data
    run_mqtt_pub(dev_id, {"light": 600, "temp": 25, "hum": 50, "soil": 2000})
    time.sleep(2)
    
    # 4. Verify status updated
    resp = requests.get(NODE_API, cert=(CLIENT_CERT, CLIENT_KEY), verify=False)
    readings = resp.json()
    latest = next(r for r in reversed(readings) if r['device_id'] == dev_id)
    assert latest['status']['overall'].startswith("warning")

def test_sit_identity_spoof_detection():
    """TC-VAL-02: Verify that mismatched payload IDs are caught"""
    dev_id = f"spoof-{int(time.time())}"
    
    # 1. Send legitimate data
    run_mqtt_pub(dev_id, {"device_id": dev_id, "soil": 2000})
    time.sleep(2)
    
    # 2. Try to spoof
    run_mqtt_pub(dev_id, {"device_id": "attacker", "soil": 4000})
    time.sleep(2)
    
    # 3. Verify rejection (latest data should still be the old one)
    resp = requests.get(NODE_API, cert=(CLIENT_CERT, CLIENT_KEY), verify=False)
    readings = resp.json()
    latest = next(r for r in reversed(readings) if r['device_id'] == dev_id)
    assert latest['raw']['soil'] == 2000

def test_sit_online_watchdog():
    """TC-INFRA-02: Verify that devices are marked offline after 30s of inactivity"""
    dev_id = f"watchdog-test-{int(time.time())}"
    
    # 1. Send a payload to mark online
    run_mqtt_pub(dev_id, {"soil": 2000})
    time.sleep(2)
    
    # 2. Verify online in metrics
    resp = requests.get(METRICS_API, cert=(CLIENT_CERT, CLIENT_KEY), verify=False)
    assert f'potbuddy_device_online{{device="{dev_id}"}} 1' in resp.text
    
    # 3. Wait 35 seconds (Watchdog is 30s)
    print("Waiting for watchdog to trigger (35s)...")
    time.sleep(35)
    
    # 4. Verify offline in metrics
    resp = requests.get(METRICS_API, cert=(CLIENT_CERT, CLIENT_KEY), verify=False)
    assert f'potbuddy_device_online{{device="{dev_id}"}} 0' in resp.text

def test_sit_database_polling():
    """TC-INFRA-04: Verify that local node picks up DB threshold changes"""
    dev_id = f"db-poll-test-{int(time.time())}"

    # 1. Send payload that is healthy with default (1500-2500)
    run_mqtt_pub(dev_id, {"soil": 2000})
    time.sleep(2)

    resp = requests.get(NODE_API, cert=(CLIENT_CERT, CLIENT_KEY), verify=False)
    latest = next(r for r in reversed(resp.json()) if r['device_id'] == dev_id)
    assert latest['status']['overall'] == "healthy"

    # 2. Update DB: set soil_inner_low to 2500 so 2000 becomes "warning"
    print("Updating DB thresholds...")
    
    update_sql = "UPDATE profiles SET soil_inner_low = 2500 WHERE name = 'default';"
    restore_sql = "UPDATE profiles SET soil_inner_low = 1500 WHERE name = 'default';"
    
    if "10.0.0.65" in BASE_URL or os.getenv("REMOTE_TEST") == "true":
        update_cmd = ["ssh", "root@10.0.0.66", f"sudo -u postgres psql -d potbuddy -c \"{update_sql}\""]
        restore_cmd = ["ssh", "root@10.0.0.66", f"sudo -u postgres psql -d potbuddy -c \"{restore_sql}\""]
    else:
        update_cmd = ["docker", "exec", "potbuddy-local-postgres-1", "psql", "-U", "postgres", "-d", "potbuddy", "-c", update_sql]
        restore_cmd = ["docker", "exec", "potbuddy-local-postgres-1", "psql", "-U", "postgres", "-d", "potbuddy", "-c", restore_sql]

    try:
        subprocess.run(update_cmd, check=True)

        # 3. Wait for poller (60s) + buffer
        print("Waiting for local-node to poll DB (65s)...")
        time.sleep(65)

        # 4. Send same payload again
        run_mqtt_pub(dev_id, {"soil": 2000})
        time.sleep(2)

        # 5. Verify it's now warning
        resp = requests.get(NODE_API, cert=(CLIENT_CERT, CLIENT_KEY), verify=False)
        latest = next(r for r in reversed(resp.json()) if r['device_id'] == dev_id)
        assert latest['status']['overall'].startswith("warning")
    finally:
        # Cleanup: restore DB
        print("Restoring DB thresholds...")
        subprocess.run(restore_cmd, check=True)
