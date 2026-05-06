import pytest
import requests
import time
import json
import paho.mqtt.client as mqtt
import os
import subprocess

# --- Configuration ---
MQTT_HOST = os.getenv("MQTT_HOST", "localhost")
MQTT_PORT = int(os.getenv("MQTT_PORT", "1884"))
NODE_API = os.getenv("NODE_API", "https://localhost:8080/history")
CA_CERT = "certs/ca.crt"
CLIENT_CERT = "certs/client.crt"
CLIENT_KEY = "certs/client.key"

@pytest.fixture
def mqtt_client():
    client = mqtt.Client(callback_api_version=mqtt.CallbackAPIVersion.VERSION2)
    client.connect(MQTT_HOST, MQTT_PORT)
    yield client
    client.disconnect()

def get_latest_status(device_id):
    # Fetch from local-node history
    try:
        resp = requests.get(f"{NODE_API}?n=10", cert=(CLIENT_CERT, CLIENT_KEY), verify=False, timeout=5)
        data = resp.json()
        # Find the latest record for this device (iterate backwards since API returns oldest first)
        for record in reversed(data):
            if record['device_id'] == device_id:
                return record
    except Exception as e:
        print(f"Error fetching status: {e}")
    return None

def publish_reading(client, device_id, light):
    topic = f"potbuddy/{device_id}/raw"
    payload = {"light": light, "temp": 25, "hum": 50, "soil": 2000}
    client.publish(topic, json.dumps(payload))

def update_thresholds(max_sun):
    # Helper to set thresholds via SQL for testing
    cmd = f"docker exec potbuddy-local-postgres-1 psql -U postgres -d potbuddy -c \"UPDATE profiles SET max_direct_sun_minutes = {max_sun} WHERE name = 'default';\""
    subprocess.run(cmd, shell=True, check=True)
    # Give the node time to poll the DB
    time.sleep(2)

@pytest.mark.timeout(300)
def test_sun_accumulation(mqtt_client):
    """TC-SUN-01 & TC-SUN-02: Direct/Indirect accumulation"""
    device_id = "test-sun-acc-sit"
    
    # 1. Start Direct Sun
    print("\nSending Direct Sun (40k lux)...")
    publish_reading(mqtt_client, device_id, 40000)
    time.sleep(2)
    
    status = get_latest_status(device_id)
    assert status is not None, "Device should be registered and visible in history"
    initial_direct = status['sunlight']['today_direct_min']
    initial_indirect = status['sunlight']['today_indirect_min']

    # 2. Wait for 1-minute throttle
    print("Waiting 65s for accumulation...")
    time.sleep(65)
    
    publish_reading(mqtt_client, device_id, 45000)
    time.sleep(2)
    
    status = get_latest_status(device_id)
    assert status['sunlight']['today_direct_min'] == initial_direct + 1
    assert status['sunlight']['today_indirect_min'] == initial_indirect

    # 3. Test Indirect
    print("Waiting 65s for indirect cycle...")
    time.sleep(65)
    publish_reading(mqtt_client, device_id, 10000) # Indirect
    time.sleep(2)
    
    status = get_latest_status(device_id)
    assert status['sunlight']['today_indirect_min'] == initial_indirect + 1

@pytest.mark.timeout(200)
def test_sun_alerts(mqtt_client):
    """TC-SUN-03: Too Much Sun Alert"""
    device_id = "test-sun-alert-sit"
    
    # Set limit to 0 to trigger immediately on first accumulation
    update_thresholds(max_sun=0)
    
    try:
        # First reading (initializes)
        publish_reading(mqtt_client, device_id, 40000)
        time.sleep(2)
        
        # Wait for throttle
        print("Waiting 65s for alert accumulation...")
        time.sleep(65)
        
        # Second reading (increments to 1, which is > 0)
        publish_reading(mqtt_client, device_id, 40000)
        time.sleep(2)
        
        status = get_latest_status(device_id)
        assert status['sunlight']['status'] == "too_much_sun"
        assert "Too much sun" in status['message']
    finally:
        # Clean up
        update_thresholds(max_sun=180)

def test_sun_schema_presence():
    """Verify sunlight object exists in API response"""
    resp = requests.get(f"{NODE_API}?n=1", cert=(CLIENT_CERT, CLIENT_KEY), verify=False)
    data = resp.json()
    if data:
        assert "sunlight" in data[0]
        assert "today_direct_min" in data[0]["sunlight"]
        assert "status" in data[0]["sunlight"]
