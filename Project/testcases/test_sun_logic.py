import requests
import time
import json
import paho.mqtt.client as mqtt
import ssl
import os

# Config
MQTT_HOST = "localhost"
MQTT_PORT = 1884 # TCP port from docker-compose
NODE_API = "https://localhost:8080/history?n=1"
DEVICE_ID = "sun-test-device"
CA_CERT = "certs/ca.crt"
CLIENT_CERT = "certs/client.crt"
CLIENT_KEY = "certs/client.key"

def pub_light(lux):
    print(f"☀️ Publishing Light: {lux} lux...")
    topic = f"potbuddy/{DEVICE_ID}/raw"
    payload = {"light": lux, "temp": 25, "hum": 50, "soil": 2000}
    
    client = mqtt.Client(callback_api_version=mqtt.CallbackAPIVersion.VERSION2)
    # Using TCP port 1884 (no mTLS required for simulation convenience on this port)
    client.connect(MQTT_HOST, MQTT_PORT)
    client.publish(topic, json.dumps(payload))
    client.disconnect()

def get_status():
    resp = requests.get(NODE_API, cert=(CLIENT_CERT, CLIENT_KEY), verify=False)
    data = resp.json()
    if not data: return None
    return data[0]

def test_sun_logic():
    print("🚀 Starting Local Sun Tracking Test")
    
    # 1. Initial Reading (Direct Sun > 30,000)
    pub_light(40000)
    time.sleep(2)
    
    status = get_status()
    print(f"Initial State: {json.dumps(status['sunlight'], indent=2)}")
    assert status['sunlight']['today_direct_min'] == 0, "Should be 0 on very first reading (starts counting from 1st min)"
    
    print("\n⏳ Waiting 65 seconds for next accumulation cycle...")
    time.sleep(65)
    
    # 2. Second Reading (Direct Sun) - Should increment to 1
    pub_light(45000)
    time.sleep(2)
    status = get_status()
    print(f"After 1 min Direct: {status['sunlight']['today_direct_min']} mins")
    assert status['sunlight']['today_direct_min'] == 1
    
    # 3. Third Reading (Indirect Sun < 30,000)
    print("\n⏳ Waiting 65 seconds...")
    time.sleep(65)
    pub_light(10000)
    time.sleep(2)
    status = get_status()
    print(f"State: Direct={status['sunlight']['today_direct_min']}, Indirect={status['sunlight']['today_indirect_min']}")
    assert status['sunlight']['today_direct_min'] == 1
    assert status['sunlight']['today_indirect_min'] == 1

    print("\n✅ Local Sun Tracking Logic Verified!")

if __name__ == "__main__":
    test_sun_logic()
