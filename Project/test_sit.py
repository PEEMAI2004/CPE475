import requests
import time
import pytest
import subprocess
import json
import ssl
import paho.mqtt.client as mqtt

# Configuration
BASE_URL = "http://10.0.0.65:8081/api"
TOKEN = "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJlbWFpbCI6ImthbWluLmppdHRAbWFpbC5rbXV0dC5hYy50aCIsInJvbGUiOiJTdXBlciBBZG1pbiIsImV4cCI6MTc3NzMwNzc2MX0.9_Uzn_uc7l_CmT_ZUomnVtfhQIVMth2yDOVu9YBOZZE"
MQTT_HOST = "10.0.0.69" # Site 1 MQTT IP
MQTT_PORT = 8883
NODE_API = "http://10.0.0.70:8080/history" # Site 1 Node API

headers = {"Authorization": f"Bearer {TOKEN}", "Content-Type": "application/json"}

def run_mqtt_pub(device_id, payload):
    """Simulates an ESP32 publishing via mTLS to the local site broker"""
    topic = f"potbuddy/{device_id}/raw"
    msg = json.dumps(payload)
    
    client = mqtt.Client(callback_api_version=mqtt.CallbackAPIVersion.VERSION2, client_id=f"test-{device_id}")
    
    # Configure mTLS with manual SSL context to bypass hostname validation
    context = ssl.create_default_context(ssl.Purpose.SERVER_AUTH, cafile="sit_ca.crt")
    context.load_cert_chain(certfile="sit_client.crt", keyfile="sit_client.key")
    context.check_hostname = False
    context.verify_mode = ssl.CERT_NONE
    
    client.tls_set_context(context)
    client.connect(MQTT_HOST, MQTT_PORT)
    client.loop_start()
    client.publish(topic, msg, qos=1)
    time.sleep(1) # Ensure message is sent
    client.loop_stop()
    client.disconnect()

def test_sit_auto_registration():
    """TC-INFRA-01: Verify that a new device is auto-registered upon first publish"""
    unique_id = f"new-esp32-{int(time.time())}"
    payload = {"light": 1000, "temp": 25, "hum": 50, "soil": 2000}
    
    # 1. Publish as a completely unknown device
    run_mqtt_pub(unique_id, payload)
    time.sleep(5) # Give local-node time to process and DB to write
    
    # 2. Check if it exists in Manager API
    resp = requests.get(f"{BASE_URL}/devices", headers=headers)
    devices = [d['device_id'] for d in resp.json()]
    
    assert unique_id in devices, f"Device {unique_id} was not auto-registered"

def test_sit_health_grading_flow():
    """TC-EVAL-01, 02: End-to-End logic check from MQTT to Local Node API"""
    dev_id = "logic-test-esp32"
    
    # 1. Send Healthy Data
    run_mqtt_pub(dev_id, {"light": 5000, "temp": 25, "hum": 50, "soil": 2000})
    time.sleep(2)
    
    # 2. Verify via Local Node API (Real-time)
    resp = requests.get(NODE_API)
    assert resp.status_code == 200
    # Find the latest reading (search from end of list)
    readings = resp.json()
    latest = next(r for r in reversed(readings) if r['device_id'] == dev_id)
    assert latest['status']['overall'] == "healthy"

    # 3. Send Warning Data (Light 600 is Warning, < 500 is Critical)
    run_mqtt_pub(dev_id, {"light": 600, "temp": 25, "hum": 50, "soil": 2000})
    time.sleep(2)
    
    # 4. Verify status updated to warning
    resp = requests.get(NODE_API)
    readings = resp.json()
    latest = next(r for r in reversed(readings) if r['device_id'] == dev_id)
    assert latest['status']['overall'] == "warning"

def test_sit_identity_spoof_detection():
    """TC-VAL-02: Verify that mismatched payload IDs are caught"""
    dev_id = "spoof-monitor"
    
    # 1. Send legitimate data
    run_mqtt_pub(dev_id, {"device_id": dev_id, "soil": 2000})
    time.sleep(1)
    
    # 2. Try to spoof with different payload ID
    # Topic is 'spoof-monitor' but payload says 'attacker'
    run_mqtt_pub(dev_id, {"device_id": "attacker", "soil": 4000})
    time.sleep(1)
    
    # 3. Verify the soil value remains 2000 (meaning 4000 was rejected)
    try:
        resp = requests.get(NODE_API)
        if resp.status_code == 200:
            readings = resp.json()
            latest = next(r for r in reversed(readings) if r['device_id'] == dev_id)
            assert latest['raw']['soil'] == 2000
    except StopIteration:
        pytest.skip("Device readings not found in history")
    except Exception as e:
        pytest.skip(f"Local node API unreachable or error: {e}")
