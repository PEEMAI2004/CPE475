import requests
import time
import pytest
import os
import json
import ssl
import paho.mqtt.client as mqtt

# Configuration from environment or defaults
BASE_URL = os.getenv("BASE_URL", "http://10.0.0.65:8081/api")
TOKEN = os.getenv("TOKEN", "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJlbWFpbCI6ImthbWluLmppdHRAbWFpbC5rbXV0dC5hYy50aCIsInJvbGUiOiJTdXBlciBBZG1pbiIsImV4cCI6MTc3NzQ1MTA3NX0.H5GVAAJESpQxfxsxXpRielD8nwB882EUZBRe3klopHo")
MQTT_HOST = os.getenv("MQTT_HOST", "10.0.0.67")
MQTT_PORT = int(os.getenv("MQTT_PORT", "8883"))
NODE_API = os.getenv("NODE_API", "https://10.0.0.68:8080/history")

headers = {"Authorization": f"Bearer {TOKEN}", "Content-Type": "application/json"}

CA_CERT = os.getenv("CA_CERT", "certs/ca-0.crt")
CLIENT_CERT = os.getenv("CLIENT_CERT", "certs/client-0.crt")
CLIENT_KEY = os.getenv("CLIENT_KEY", "certs/client-0.key")

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
    client.connect(MQTT_HOST, MQTT_PORT)
    client.loop_start()
    client.publish(topic, msg, qos=1)
    time.sleep(2) # Give it a bit more time for remote network
    client.loop_stop()
    client.disconnect()

def test_sit_auto_registration():
    """TC-INFRA-01: Verify that a new device is auto-registered upon first publish"""
    unique_id = f"new-esp32-{int(time.time())}"
    payload = {"light": 1000, "temp": 25, "hum": 50, "soil": 2000}
    
    # 1. Publish as a completely unknown device
    run_mqtt_pub(unique_id, payload)
    time.sleep(10) # Give more time for DB sync across nodes
    
    # 2. Check if it exists in Manager API
    resp = requests.get(f"{BASE_URL}/devices", headers=headers)
    devices = [d['device_id'] for d in resp.json()]
    
    assert unique_id in devices, f"Device {unique_id} was not auto-registered"

def test_sit_health_grading_flow():
    """TC-EVAL-01, 02: End-to-End logic check from MQTT to Local Node API"""
    dev_id = f"logic-test-{int(time.time())}"
    
    # 1. Send Healthy Data
    run_mqtt_pub(dev_id, {"light": 5000, "temp": 25, "hum": 50, "soil": 2000})
    time.sleep(5)
    
    # 2. Verify via Local Node API
    resp = requests.get(NODE_API, cert=(CLIENT_CERT, CLIENT_KEY), verify=False)
    assert resp.status_code == 200
    readings = resp.json()
    latest = next(r for r in reversed(readings) if r['device_id'] == dev_id)
    assert latest['status']['overall'] == "healthy"

    # 3. Send Warning Data
    run_mqtt_pub(dev_id, {"light": 600, "temp": 25, "hum": 50, "soil": 2000})
    time.sleep(5)
    
    # 4. Verify status updated
    resp = requests.get(NODE_API, cert=(CLIENT_CERT, CLIENT_KEY), verify=False)
    readings = resp.json()
    latest = next(r for r in reversed(readings) if r['device_id'] == dev_id)
    assert latest['status']['overall'] == "warning"

def test_sit_identity_spoof_detection():
    """TC-VAL-02: Verify that mismatched payload IDs are caught"""
    dev_id = f"spoof-{int(time.time())}"
    
    # 1. Send legitimate data
    run_mqtt_pub(dev_id, {"device_id": dev_id, "soil": 2000})
    time.sleep(5)
    
    # 2. Try to spoof
    run_mqtt_pub(dev_id, {"device_id": "attacker", "soil": 4000})
    time.sleep(5)
    
    # 3. Verify rejection
    resp = requests.get(NODE_API, cert=(CLIENT_CERT, CLIENT_KEY), verify=False)
    readings = resp.json()
    latest = next(r for r in reversed(readings) if r['device_id'] == dev_id)
    assert latest['raw']['soil'] == 2000
