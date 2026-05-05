import requests
import paho.mqtt.client as mqtt
import ssl
import json
import os
import argparse
import sys
import time
from cryptography import x509

def validate_cert(cert_text, label):
    """Validates if the provided text is a valid X.509 certificate in PEM format"""
    try:
        x509.load_pem_x509_certificate(cert_text.encode())
        print(f"✅ {label} is valid.")
        return True
    except Exception as e:
        print(f"❌ {label} validation failed: {e}")
        return False

def bootstrap_device(manager_url, token):
    """Bootstraps the device to get mTLS certificates"""
    print(f"📡 Bootstrapping with token: {token}...")
    url = f"{manager_url}/api/enrollment/bootstrap"
    payload = {"auth_token": token}
    
    try:
        resp = requests.post(url, json=payload, timeout=10)
        if resp.status_code != 200:
            print(f"❌ Bootstrap failed: {resp.status_code} - {resp.text}")
            sys.exit(1)
        
        data = resp.json()
        certs = {
            "ca.crt": data.get("ca.crt"),
            "client.crt": data.get("client.crt"),
            "client.key": data.get("client.key")
        }
        
        # Basic field presence check
        for k, v in certs.items():
            if not v:
                print(f"❌ Missing {k} in bootstrap response.")
                sys.exit(1)
        
        # Crypto validation
        valid = True
        valid &= validate_cert(certs["ca.crt"], "CA Certificate")
        valid &= validate_cert(certs["client.crt"], "Client Certificate")
        
        # Client Key check (just check headers)
        if "-----BEGIN PRIVATE KEY-----" in certs["client.key"] or "-----BEGIN RSA PRIVATE KEY-----" in certs["client.key"]:
            print("✅ Client Private Key format looks correct.")
        else:
            print("❌ Client Private Key format is invalid.")
            valid = False
            
        if not valid:
            print("❌ Certificate validation failed. Aborting.")
            sys.exit(1)
            
        return certs
    except Exception as e:
        print(f"❌ Bootstrap error: {e}")
        sys.exit(1)

def test_mqtt_pub(broker_host, broker_port, certs, device_id):
    """Tests connectivity to the MQTT broker using mTLS"""
    print(f"🔌 Connecting to {broker_host}:{broker_port} via mTLS...")
    
    # Save certs to temp files for Paho
    with open("temp_ca.crt", "w") as f: f.write(certs["ca.crt"])
    with open("temp_client.crt", "w") as f: f.write(certs["client.crt"])
    with open("temp_client.key", "w") as f: f.write(certs["client.key"])
    
    client = mqtt.Client(callback_api_version=mqtt.CallbackAPIVersion.VERSION2, client_id=f"mock-{device_id}")
    
    context = ssl.create_default_context(ssl.Purpose.SERVER_AUTH, cafile="temp_ca.crt")
    context.load_cert_chain(certfile="temp_client.crt", keyfile="temp_client.key")
    context.check_hostname = False
    context.verify_mode = ssl.CERT_NONE # Rely on CA trust, ignore hostname for local testing
    
    client.tls_set_context(context)
    
    connected = False
    def on_connect(client, userdata, flags, rc, properties):
        nonlocal connected
        if rc == 0:
            print("✅ MQTT Connected successfully via mTLS.")
            connected = True
        else:
            print(f"❌ MQTT Connection failed with code: {rc}")

    client.on_connect = on_connect
    
    try:
        client.connect(broker_host, broker_port, 60)
        client.loop_start()
        
        # Wait for connection
        for _ in range(50):
            if connected: break
            time.sleep(0.1)
            
        if connected:
            topic = f"potbuddy/{device_id}/raw"
            payload = {"light": 1500, "temp": 24.5, "hum": 45, "soil": 1800}
            print(f"📤 Publishing to {topic}...")
            info = client.publish(topic, json.dumps(payload), qos=1)
            info.wait_for_publish()
            print("✅ Message published and acknowledged.")
        else:
            print("❌ Connection timeout.")
            sys.exit(1)
            
    finally:
        client.loop_stop()
        client.disconnect()
        # Cleanup
        for f in ["temp_ca.crt", "temp_client.crt", "temp_client.key"]:
            if os.path.exists(f): os.remove(f)

if __name__ == "__main__":
    parser = argparse.ArgumentParser(description="Mock PotBuddy Device Publisher")
    parser.add_argument("--manager", default="http://localhost:8081", help="Manager API URL")
    parser.add_argument("--token", help="Device Auth Token (if bootstrapping is needed)")
    parser.add_argument("--host", default="localhost", help="MQTT Broker Host")
    parser.add_argument("--port", type=int, default=8883, help="MQTT Broker Port")
    parser.add_argument("--id", default="mock-device-01", help="Device ID for topic")
    parser.add_argument("--ca", help="Path to existing CA certificate")
    parser.add_argument("--cert", help="Path to existing client certificate")
    parser.add_argument("--key", help="Path to existing client key")
    
    args = parser.parse_args()
    
    if args.ca and args.cert and args.key:
        print("📁 Using existing certificates from local files...")
        with open(args.ca, 'r') as f: ca_data = f.read()
        with open(args.cert, 'r') as f: cert_data = f.read()
        with open(args.key, 'r') as f: key_data = f.read()
        certs = {"ca.crt": ca_data, "client.crt": cert_data, "client.key": key_data}
        validate_cert(certs["ca.crt"], "CA Certificate")
        validate_cert(certs["client.crt"], "Client Certificate")
    elif args.token:
        certs = bootstrap_device(args.manager, args.token)
    else:
        print("❌ Error: Must provide either --token or all of (--ca, --cert, --key)")
        sys.exit(1)
        
    test_mqtt_pub(args.host, args.port, certs, args.id)
    print("\n✨ Mock Device Test Complete! ✨")
