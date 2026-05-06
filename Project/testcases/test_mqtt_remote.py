import ssl
import paho.mqtt.client as mqtt
import json
import time
import os

MQTT_HOST = "mqtt-0.iot.kaminjitt.com"
MQTT_PORT = 8883
CA_CERT = "Project/certs/ca.crt"
CLIENT_CERT = "Project/certs/client.crt"
CLIENT_KEY = "Project/certs/client.key"

def test_mqtt():
    print(f"Connecting to {MQTT_HOST}:{MQTT_PORT}...")
    client = mqtt.Client(callback_api_version=mqtt.CallbackAPIVersion.VERSION2)
    
    context = ssl.create_default_context(ssl.Purpose.SERVER_AUTH, cafile=CA_CERT)
    context.load_cert_chain(certfile=CLIENT_CERT, keyfile=CLIENT_KEY)
    context.check_hostname = False
    context.verify_mode = ssl.CERT_NONE
    
    client.tls_set_context(context)
    
    connected = [False]
    def on_connect(client, userdata, flags, rc, properties):
        print(f"Connected with result code {rc}")
        if rc == 0:
            connected[0] = True

    client.on_connect = on_connect

    try:
        client.connect(MQTT_HOST, MQTT_PORT, 10)
        client.loop_start()
        time.sleep(5)
        client.loop_stop()
        client.disconnect()
    except Exception as e:
        print(f"Error: {e}")

    if connected[0]:
        print("Success: Connected to MQTT broker.")
    else:
        print("Failure: Could not connect to MQTT broker.")

if __name__ == "__main__":
    test_mqtt()
