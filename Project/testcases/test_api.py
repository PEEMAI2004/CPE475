import requests
import time
import pytest
import os
from cryptography import x509
from cryptography.x509.oid import NameOID
from cryptography.hazmat.primitives import hashes
from cryptography.hazmat.primitives import serialization
from cryptography.hazmat.primitives.asymmetric import rsa

# Configuration from environment or defaults
BASE_URL = os.getenv("BASE_URL", "http://localhost:8081/api")
TOKEN = os.getenv("TOKEN", "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJlbWFpbCI6ImFkbWluQHRlc3QuY29tIiwicm9sZSI6IlN1cGVyIEFkbWluIiwiZXhwIjoxNzc4MDYwMjQxfQ.dCE9-R8FYVGTmI9tHIxbbvnmOT0Wkxgorap9j9Jkl2E")

# Tokens for RBAC tests (can be overridden for different environments)
SITE_ADMIN_TOKEN = os.getenv("SITE_ADMIN_TOKEN", "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJlbWFpbCI6InNpdGVAdGVzdC5jb20iLCJyb2xlIjoiU2l0ZSBBZG1pbiIsImV4cCI6MTc3ODA2MDI0MX0.JxBPnKXnis8xPJuB_rFvNT87pZR5hApryhF-gyHpaow")
VIEWER_TOKEN = os.getenv("VIEWER_TOKEN", "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJlbWFpbCI6InZpZXdlckB0ZXN0LmNvbSIsInJvbGUiOiJWaWV3ZXIiLCJleHAiOjE3NzgwNjAyNDF9.IRxlgOkETPc3UePKjIpJxMhJpZLOKrRdnGKo_ufiBP4")

@pytest.fixture
def api_headers():
    return {
        "Authorization": f"Bearer {TOKEN}",
        "Content-Type": "application/json"
    }

@pytest.fixture
def site_admin_headers():
    return {
        "Authorization": f"Bearer {SITE_ADMIN_TOKEN}",
        "Content-Type": "application/json"
    }

@pytest.fixture
def viewer_headers():
    return {
        "Authorization": f"Bearer {VIEWER_TOKEN}",
        "Content-Type": "application/json"
    }

def generate_csr(common_name):
    """Generates a private key and a CSR for testing"""
    key = rsa.generate_private_key(
        public_exponent=65537,
        key_size=2048,
    )
    csr = x509.CertificateSigningRequestBuilder().subject_name(x509.Name([
        x509.NameAttribute(NameOID.COMMON_NAME, common_name),
    ])).sign(key, hashes.SHA256())
    
    csr_pem = csr.public_bytes(serialization.Encoding.PEM).decode()
    return csr_pem

def test_rbac_super_admin_access(api_headers):
    """TC-RBAC-01: Verify Super Admin has access to all management endpoints"""
    endpoints = ["/users", "/profiles", "/devices", "/infrastructure", "/enrollment/nodes", "/enrollment/devices"]
    for ep in endpoints:
        resp = requests.get(f"{BASE_URL}{ep}", headers=api_headers)
        assert resp.status_code == 200, f"Failed to access {ep} (status {resp.status_code})"

def test_rbac_site_admin_permissions(site_admin_headers):
    """TC-RBAC-02: Verify Site Admin permissions and restrictions"""
    # Allowed: Devices, Profiles
    assert requests.get(f"{BASE_URL}/devices", headers=site_admin_headers).status_code == 200
    assert requests.get(f"{BASE_URL}/profiles", headers=site_admin_headers).status_code == 200
    
    # Blocked: Users, Infrastructure Nodes
    assert requests.get(f"{BASE_URL}/users", headers=site_admin_headers).status_code == 403
    assert requests.get(f"{BASE_URL}/enrollment/nodes", headers=site_admin_headers).status_code == 403

def test_rbac_viewer_permissions(viewer_headers):
    """TC-RBAC-03: Verify Viewer permissions (Read-only)"""
    # Allowed: GET data
    assert requests.get(f"{BASE_URL}/devices", headers=viewer_headers).status_code == 200
    assert requests.get(f"{BASE_URL}/profiles", headers=viewer_headers).status_code == 200
    
    # Blocked: POST operations
    resp = requests.post(f"{BASE_URL}/enrollment/devices", headers=viewer_headers, json={"device_id": "v-dev"})
    assert resp.status_code == 403

def test_rbac_invalid_token():
    """TC-RBAC-04: Verify that a malformed token is rejected"""
    bad_headers = {"Authorization": "Bearer not-a-real-token"}
    resp = requests.get(f"{BASE_URL}/devices", headers=bad_headers)
    assert resp.status_code == 401

def test_auth_invalid_id_token():
    """TC-AUTH-03: Verify malformed Google ID token is rejected"""
    resp = requests.post(f"{BASE_URL}/auth/login", json={"idToken": "invalid-google-token"})
    assert resp.status_code == 401

def test_infrastructure_node_lifecycle(api_headers):
    """TC-NODE-01, 03, 05, 06: Verify Node Enrollment, Config, Regeneration, and Deletion"""
    node_data = {
        "name": "IDE-Test-Node",
        "site_id": 77,
        "address": "ide.test.com"
    }
    
    # Enroll
    resp = requests.post(f"{BASE_URL}/enrollment/nodes", headers=api_headers, json=node_data)
    assert resp.status_code == 201
    node_id = resp.json()['id']
    old_token = resp.json()['token']

    # Token Regeneration
    resp = requests.post(f"{BASE_URL}/enrollment/nodes/{node_id}/regen-token", headers=api_headers)
    assert resp.status_code == 200
    new_token = resp.json()['token']
    assert new_token != old_token

    # Config Download
    resp = requests.get(f"{BASE_URL}/enrollment/nodes/{node_id}/config", headers=api_headers)
    assert resp.status_code == 200
    assert "local_mqtt" in resp.text

    # Delete
    resp = requests.delete(f"{BASE_URL}/enrollment/nodes/{node_id}", headers=api_headers)
    assert resp.status_code == 200

def test_csr_node_enrollment(api_headers):
    """Verify that a node can enroll using a CSR (No Key Escrow)"""
    # 1. Create a node
    node_data = {"name": "CSR-Node", "site_id": 1, "address": "csr.node.com"}
    resp = requests.post(f"{BASE_URL}/enrollment/nodes", headers=api_headers, json=node_data)
    node_id = resp.json()['id']

    # 2. Generate CSR and request Client Certificate
    csr = generate_csr("CSR-Node-Client")
    resp = requests.post(f"{BASE_URL}/enrollment/nodes/{node_id}/client-cert", headers=api_headers, json={"csr": csr})
    
    assert resp.status_code == 200
    data = resp.json()
    assert "client.crt" in data
    assert "ca.crt" in data
    assert "client.key" not in data # CRITICAL: No private key returned

    # 3. Generate CSR and request Server Certificate
    csr_server = generate_csr("csr.node.com")
    resp = requests.post(f"{BASE_URL}/enrollment/nodes/{node_id}/server-cert", headers=api_headers, json={"csr": csr_server})
    
    assert resp.status_code == 200
    data = resp.json()
    assert "server.crt" in data
    assert "server.key" not in data # CRITICAL: No private key returned

    # Cleanup
    requests.delete(f"{BASE_URL}/enrollment/nodes/{node_id}", headers=api_headers)

def test_node_token_regeneration_forbidden(site_admin_headers, api_headers):
    """TC-NODE-07: Verify Site Admin cannot regenerate node tokens"""
    # 1. Create a node as Super Admin
    resp = requests.post(f"{BASE_URL}/enrollment/nodes", headers=api_headers, json={
        "name": "Forbidden-Node", "site_id": 1, "address": "test.com"
    })
    node_id = resp.json()['id']
    
    # 2. Attempt regeneration as Site Admin
    resp = requests.post(f"{BASE_URL}/enrollment/nodes/{node_id}/regen-token", headers=site_admin_headers)
    assert resp.status_code == 403
    
    # Cleanup
    requests.delete(f"{BASE_URL}/enrollment/nodes/{node_id}", headers=api_headers)

def test_node_enrollment_negative(api_headers):
    """TC-NODE-02, 04: Verify error handling for nodes"""
    # Missing fields
    resp = requests.post(f"{BASE_URL}/enrollment/nodes", headers=api_headers, json={"name": "Incomplete"})
    assert resp.status_code == 400

    # Non-existent ID for config
    resp = requests.get(f"{BASE_URL}/enrollment/nodes/99999/config", headers=api_headers)
    assert resp.status_code == 404

def test_device_bootstrap_lifecycle(api_headers):
    """TC-DEV-01, TC-DEV-05, TC-BOOT-01: Verify Device Registration, Regeneration, and Bootstrap"""
    dev_id = f"ide-dev-{int(time.time())}"
    
    # Register
    resp = requests.post(f"{BASE_URL}/enrollment/devices", headers=api_headers, json={"device_id": dev_id})
    assert resp.status_code == 201
    auth_token = resp.json()['auth_token']

    # Regeneration
    resp = requests.post(f"{BASE_URL}/enrollment/devices/{dev_id}/regen-token", headers=api_headers)
    assert resp.status_code == 200
    new_token = resp.json()['auth_token']
    assert new_token != auth_token

    # Bootstrap (Public)
    resp = requests.post(f"{BASE_URL}/enrollment/bootstrap", json={"auth_token": new_token})
    # CA might be disabled in some environments, but endpoint must respond
    assert resp.status_code in [200, 503]

    # Cleanup
    resp = requests.delete(f"{BASE_URL}/enrollment/devices/{dev_id}", headers=api_headers)
    assert resp.status_code == 200

def test_bootstrap_invalid_token():
    """TC-BOOT-02: Verify that a fake or invalid AuthToken is rejected"""
    resp = requests.post(f"{BASE_URL}/enrollment/bootstrap", json={"auth_token": "pb_dev_fake123"})
    assert resp.status_code == 401

def test_csr_device_enrollment(api_headers):
    """Verify that a device can enroll using a CSR"""
    dev_id = f"csr-dev-{int(time.time())}"
    csr = generate_csr(dev_id)

    # 1. Register with CSR
    resp = requests.post(f"{BASE_URL}/enrollment/devices", headers=api_headers, json={
        "device_id": dev_id,
        "csr": csr
    })
    
    assert resp.status_code == 201
    data = resp.json()
    assert "client.crt" in data
    assert "client.key" not in data # CRITICAL

    # 2. Bootstrap with CSR
    auth_token = data['auth_token']
    resp = requests.post(f"{BASE_URL}/enrollment/bootstrap", json={
        "auth_token": auth_token,
        "csr": csr
    })
    
    assert resp.status_code in [200, 503]
    if resp.status_code == 200:
        assert "client.crt" in resp.json()
        assert "client.key" not in resp.json()

    # Cleanup
    requests.delete(f"{BASE_URL}/enrollment/devices/{dev_id}", headers=api_headers)

def test_device_token_regeneration_forbidden(viewer_headers, api_headers):
    """TC-DEV-06: Verify Viewer cannot regenerate device tokens"""
    dev_id = "test-forbidden-dev"
    requests.post(f"{BASE_URL}/enrollment/devices", headers=api_headers, json={"device_id": dev_id})
    
    resp = requests.post(f"{BASE_URL}/enrollment/devices/{dev_id}/regen-token", headers=viewer_headers)
    assert resp.status_code == 403
    
    requests.delete(f"{BASE_URL}/enrollment/devices/{dev_id}", headers=api_headers)

def test_device_registration_negative(api_headers):
    """TC-DEV-03, 04: Verify constraints for device registration"""
    # TC-DEV-03: Duplicate
    dev_id = f"dup-test-{int(time.time())}"
    requests.post(f"{BASE_URL}/enrollment/devices", headers=api_headers, json={"device_id": dev_id})
    resp = requests.post(f"{BASE_URL}/enrollment/devices", headers=api_headers, json={"device_id": dev_id})
    assert resp.status_code == 500 

    # TC-DEV-04: Empty ID
    resp = requests.post(f"{BASE_URL}/enrollment/devices", headers=api_headers, json={"device_id": ""})
    assert resp.status_code == 400

    # Cleanup
    requests.delete(f"{BASE_URL}/enrollment/devices/{dev_id}", headers=api_headers)

def test_system_infrastructure_health(api_headers):
    """TC-INFRA-03: Verify real-time health monitoring data"""
    resp = requests.get(f"{BASE_URL}/infrastructure", headers=api_headers)
    assert resp.status_code == 200
    data = resp.json()
    assert isinstance(data, list)
    assert len(data) > 0

def test_machine_heartbeat(api_headers):
    """Verify heartbeat endpoint works with both Node and Device tokens"""
    # 1. Create a node to get a token
    resp = requests.post(f"{BASE_URL}/enrollment/nodes", headers=api_headers, json={
        "name": "Heartbeat-Node", "address": "localhost"
    })
    node_id = resp.json()['id']
    node_token = resp.json()['token']

    # 2. Test Node Heartbeat
    resp = requests.post(f"{BASE_URL}/infrastructure/heartbeat", headers={"X-PotBuddy-Token": node_token})
    assert resp.status_code == 200
    assert resp.json()['identity'] == "Heartbeat-Node"

    # 3. Create a device to get a token
    dev_id = "heartbeat-device-123"
    resp = requests.post(f"{BASE_URL}/enrollment/devices", headers=api_headers, json={"device_id": dev_id})
    dev_token = resp.json()['auth_token']

    # 4. Test Device Heartbeat
    resp = requests.post(f"{BASE_URL}/infrastructure/heartbeat", headers={"Authorization": f"Bearer {dev_token}"})
    assert resp.status_code == 200
    assert resp.json()['identity'] == dev_id

    # Cleanup
    requests.delete(f"{BASE_URL}/enrollment/nodes/{node_id}", headers=api_headers)
    requests.delete(f"{BASE_URL}/enrollment/devices/{dev_id}", headers=api_headers)
