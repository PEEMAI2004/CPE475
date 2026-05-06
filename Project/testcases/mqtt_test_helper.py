
import ssl
import paho.mqtt.client as mqtt
import json
import time

def run_test_pub(device_id, payload):
    topic = f"potbuddy/{device_id}/raw"
    msg = json.dumps(payload)
    
    client = mqtt.Client(callback_api_version=mqtt.CallbackAPIVersion.VERSION2)
    
    # Direct SSL context manipulation to bypass all verification
    context = ssl.create_default_context(ssl.Purpose.SERVER_AUTH)
    context.check_hostname = False
    context.verify_mode = ssl.CERT_NONE
    context.load_cert_chain(certfile="sit_client.crt", keyfile="sit_client.key")
    
    client.tls_set_context(context)
    client.connect("localhost", 8883)
    client.loop_start()
    client.publish(topic, msg, qos=1)
    time.sleep(1)
    client.loop_stop()
    client.disconnect()

if __name__ == "__main__":
    run_test_pub("manual-test", {"light": 1000})
