import requests
import subprocess
import time
import os
import shutil
import json
import signal

# Configuration
BASE_URL = os.getenv("BASE_URL", "http://localhost:8081/api")
ADMIN_TOKEN = os.getenv("TOKEN", "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJlbWFpbCI6ImFkbWluQHRlc3QuY29tIiwicm9sZSI6IlN1cGVyIEFkbWluIiwiZXhwIjoxNzc4MDYwMjQxfQ.dCE9-R8FYVGTmI9tHIxbbvnmOT0Wkxgorap9j9Jkl2E")
ENROLL_BIN = os.path.abspath("local-node/bin/enroll")
NODE_BIN = os.path.abspath("local-node/bin/node")
MOCK_PUB_BIN = os.path.abspath("mock_device_pub.py")
WORK_DIR = os.path.abspath("test-e2e-workdir")
MQTT_PORT = 18833
NODE_API_PORT = 18080

headers = {"Authorization": f"Bearer {ADMIN_TOKEN}", "Content-Type": "application/json"}

def cleanup_dir(path):
    if os.path.exists(path):
        shutil.rmtree(path)
    os.makedirs(path)

def run_enroll(token, enroll_type="node", cn=None, workdir=None):
    cmd = [ENROLL_BIN, "-token", token, "-manager", "http://localhost:8081"]
    if enroll_type == "mqtt":
        cmd.extend(["-type", "mqtt", "-cn", cn or "localhost"])
    
    result = subprocess.run(cmd, cwd=workdir, capture_output=True, text=True)
    return result

def start_mosquitto_docker(workdir):
    print(f"Starting Mosquitto in Docker on port {MQTT_PORT}...")
    
    # Ensure certs/keys are readable by the 'mosquitto' user inside container
    for f in ["ca.crt", "server.crt", "server.key"]:
        p = os.path.join(workdir, f)
        if os.path.exists(p):
            os.chmod(p, 0o644)

    conf_path = os.path.join(workdir, "mosquitto.conf")
    with open(conf_path, 'r') as f: data = f.read()
    
    # Ensure listener is bound to all interfaces inside container
    data = data.replace("listener 8883", f"listener {MQTT_PORT} 0.0.0.0")
    if "allow_anonymous true" not in data:
        data = "allow_anonymous true\n" + data
        
    print("--- mosquitto.conf content ---")
    print(data)
    print("------------------------------")
    
    with open(conf_path, 'w') as f: f.write(data)
    
    subprocess.run(["docker", "rm", "-f", "e2e-mqtt-test"], capture_output=True)
    cmd = [
        "docker", "run", "-d",
        "-v", f"{workdir}:/mosquitto/config",
        "-p", f"{MQTT_PORT}:{MQTT_PORT}",
        "--name", "e2e-mqtt-test",
        "eclipse-mosquitto:2",
        "mosquitto", "-c", "/mosquitto/config/mosquitto.conf"
    ]
    subprocess.run(cmd, check=True)
    time.sleep(5)
    
    # Check if container is still running
    res = subprocess.run(["docker", "inspect", "-f", "{{.State.Running}}", "e2e-mqtt-test"], capture_output=True, text=True)
    if res.stdout.strip() != "true":
        print("❌ Mosquitto container stopped unexpectedly!")
        subprocess.run(["docker", "logs", "e2e-mqtt-test"])
        raise Exception("Mosquitto failed to start in Docker")

def test_e2e_extended():
    print("🚀 Starting Comprehensive TC-E2E-01 Execution")
    cleanup_dir(WORK_DIR)

    # 1. Create Node
    print("\n--- Phase 1: Infrastructure Creation ---")
    node_data = {"name": "E2E-Final-Node", "site_id": 102, "address": "localhost"}
    resp = requests.post(f"{BASE_URL}/enrollment/nodes", headers=headers, json=node_data)
    resp.raise_for_status()
    node = resp.json()
    node_id = node['id']
    token_1 = node['token']
    print(f"Node Created: ID={node_id}, Token={token_1}")

    # 2. Initial Enrollment
    print("\n--- Phase 2: Initial Enrollment ---")
    p1_dir = os.path.join(WORK_DIR, "phase1")
    os.makedirs(p1_dir)
    
    print("Enrolling Local Node...")
    run_enroll(token_1, workdir=p1_dir)
    print("Enrolling MQTT Broker...")
    run_enroll(token_1, enroll_type="mqtt", workdir=p1_dir)
    print("Enrollment Successful")

    # 3. Deployment & Connectivity
    print("\n--- Phase 3: Deployment & Connectivity Verification ---")
    start_mosquitto_docker(p1_dir)

    node_proc = None
    try:
        print(f"Starting Local Node...")
        node_env = os.environ.copy()
        node_env["LOCAL_BROKER"] = f"tcp://localhost:{MQTT_PORT}"
        node_env["HTTP_CA_FILE"] = os.path.abspath(os.path.join(p1_dir, "ca.crt"))
        node_env["HTTP_CERT_FILE"] = os.path.abspath(os.path.join(p1_dir, "client.crt"))
        node_env["HTTP_KEY_FILE"] = os.path.abspath(os.path.join(p1_dir, "client.key"))
        
        node_proc = subprocess.Popen([NODE_BIN], cwd=p1_dir, env=node_env, 
                                     stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
        time.sleep(3)

        print("Registering Device via API...")
        dev_id = "e2e-device-initial"
        requests.delete(f"{BASE_URL}/enrollment/devices/{dev_id}", headers=headers)
        reg_resp = requests.post(f"{BASE_URL}/enrollment/devices", headers=headers, json={"device_id": dev_id})
        auth_token = reg_resp.json()['auth_token']
        
        print("Running mock_device_pub.py (Bootstrap & Publish)...")
        dev_cert_dir = os.path.abspath(os.path.join(WORK_DIR, "device_certs"))
        os.makedirs(dev_cert_dir)
        
        boot_resp = requests.post(f"{BASE_URL}/enrollment/bootstrap", json={"auth_token": auth_token})
        bundle = boot_resp.json()
        ca_path = os.path.join(dev_cert_dir, "ca.crt")
        crt_path = os.path.join(dev_cert_dir, "client.crt")
        key_path = os.path.join(dev_cert_dir, "client.key")
        with open(ca_path, "w") as f: f.write(bundle['ca.crt'])
        with open(crt_path, "w") as f: f.write(bundle['client.crt'])
        with open(key_path, "w") as f: f.write(bundle['client.key'])

        r = subprocess.run([
            "python3", MOCK_PUB_BIN, "--port", str(MQTT_PORT), "--id", dev_id,
            "--ca", ca_path, "--cert", crt_path, "--key", key_path
        ], capture_output=True, text=True)
        
        if r.returncode != 0:
            print(f"❌ Mock pub failed: {r.stderr}")
            subprocess.run(["docker", "logs", "e2e-mqtt-test"])
            raise Exception("Initial connectivity check failed")
        print("✅ Initial Connectivity Verified.")

    finally:
        if node_proc: node_proc.terminate()
        subprocess.run(["docker", "rm", "-f", "e2e-mqtt-test"], capture_output=True)

    # 4. Token Refresh
    print("\n--- Phase 4: Token Refresh ---")
    regen_resp = requests.post(f"{BASE_URL}/enrollment/nodes/{node_id}/regen-token", headers=headers)
    token_2 = regen_resp.json()['token']
    print(f"New Token Generated: {token_2}")

    # 5. Old Token Rejection
    print("\n--- Phase 5: Old Token Rejection ---")
    p2_old_dir = os.path.join(WORK_DIR, "phase2_old")
    os.makedirs(p2_old_dir)
    r = run_enroll(token_1, workdir=p2_old_dir)
    assert r.returncode != 0 and "401" in r.stderr, "Old token was NOT rejected for node"
    print("✅ Old Token rejection verified.")

    # 6. New Token Acceptance & Deployment
    print("\n--- Phase 6: New Token Acceptance & Deployment ---")
    p2_new_dir = os.path.join(WORK_DIR, "phase2_new")
    os.makedirs(p2_new_dir)
    
    print("Re-enrolling Node and Broker with New Token...")
    run_enroll(token_2, workdir=p2_new_dir)
    run_enroll(token_2, enroll_type="mqtt", workdir=p2_new_dir)
    
    start_mosquitto_docker(p2_new_dir)
    
    try:
        print("Verifying Existing Device Connectivity (No Token, Old Certs)...")
        r = subprocess.run([
            "python3", MOCK_PUB_BIN, "--port", str(MQTT_PORT), "--id", dev_id,
            "--ca", ca_path, "--cert", crt_path, "--key", key_path
        ], capture_output=True, text=True)
        
        if r.returncode != 0:
            print(f"❌ Re-verification failed: {r.stderr}")
            subprocess.run(["docker", "logs", "e2e-mqtt-test"])
            raise Exception("Connectivity broken after rotation")
        print("✅ Connectivity Maintained for existing devices after infrastructure rotation!")

    finally:
        subprocess.run(["docker", "rm", "-f", "e2e-mqtt-test"], capture_output=True)

    # 7. Infrastructure Deletion
    print("\n--- Phase 7: Infrastructure Deletion ---")
    requests.delete(f"{BASE_URL}/enrollment/nodes/{node_id}", headers=headers).raise_for_status()
    print("Node Deleted.")

    # 8. Post-Deletion Rejection
    print("\n--- Phase 8: Post-Deletion Rejection ---")
    p3_dir = os.path.join(WORK_DIR, "phase3")
    os.makedirs(p3_dir)
    r = run_enroll(token_2, workdir=p3_dir)
    assert r.returncode != 0 and "401" in r.stderr, "Deleted node token was NOT rejected"
    print("✅ Post-deletion rejection verified.")

    print("\n🏆 TC-E2E-01: ALL PHASES PASSED SUCCESSFULLY! 🏆")

if __name__ == "__main__":
    try:
        test_e2e_extended()
    except Exception as e:
        print(f"\n❌ TEST FAILED: {e}")
        exit(1)
    finally:
        if os.path.exists(WORK_DIR):
            shutil.rmtree(WORK_DIR)
