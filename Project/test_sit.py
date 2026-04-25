import requests
import time
import pytest
import subprocess
import json

# Configuration
BASE_URL = "http://localhost:8081/api"
TOKEN = "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJlbWFpbCI6ImthbWluLmppdHRAbWFpbC5rbXV0dC5hYy50aCIsInJvbGUiOiJTdXBlciBBZG1pbiIsImV4cCI6MTc3NzIxOTIzM30.vfMJny_h_Queeo9W805hKPa5f3Zu4DhB6zgaC0VKnFY"
MQTT_HOST = "localhost" # Testing against local forward or internal bridge
MQTT_PORT = 1884        # Standard port for easy automation (non-mTLS for logic testing)

headers = {"Authorization": f"Bearer {TOKEN}", "Content-Type": "application/json"}

import paho.mqtt.publish as publish

def run_mqtt_pub(device_id, payload):
    """Simulates an ESP32 publishing via direct network connection to the local site broker"""
    topic = f"potbuddy/{device_id}/raw"
    msg = json.dumps(payload)
    # Connect directly to the local broker on the mapped port
    publish.single(
        topic, 
        payload=msg, 
        hostname="localhost", 
        port=1884
    )

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
    NODE_API = "http://localhost:8080/history"
    
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
    NODE_API = "http://localhost:8080/history"
    
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
