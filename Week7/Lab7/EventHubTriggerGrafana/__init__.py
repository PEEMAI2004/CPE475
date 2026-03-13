import azure.functions as func
import logging
import json
import os
import datetime
import struct
import snappy
import requests

# --- Minimal Prometheus Protobuf Encoder ---
def encode_varint(value):
    encoded = b''
    while value >= 0x80:
        encoded += bytes([(value & 0x7f) | 0x80])
        value >>= 7
    encoded += bytes([value])
    return encoded

def encode_string(value):
    value_bytes = value.encode('utf-8')
    return encode_varint(len(value_bytes)) + value_bytes

def build_label(name, value):
    return b'\x0a' + encode_string(name) + b'\x12' + encode_string(value)

def build_sample(value, timestamp_ms):
    return b'\x09' + struct.pack('<d', float(value)) + b'\x10' + encode_varint(timestamp_ms)

def build_timeseries(metric_name, labels_dict, value, timestamp_ms):
    ts_bytes = b''
    all_labels = {"__name__": metric_name}
    all_labels.update(labels_dict)
    
    for k, v in all_labels.items():
        lbl = build_label(k, str(v))
        ts_bytes += b'\x0a' + encode_varint(len(lbl)) + lbl
        
    smp = build_sample(value, timestamp_ms)
    ts_bytes += b'\x12' + encode_varint(len(smp)) + smp
    return ts_bytes

def build_write_request(timeseries_list):
    req_bytes = b''
    for ts in timeseries_list:
        req_bytes += b'\x0a' + encode_varint(len(ts)) + ts
    return req_bytes
# -------------------------------------------

def main(event: func.EventHubEvent):
    logging.info("==== STARTING NEW EVENT PROCESSING ====")
    
    try:
        # Decode incoming message
        message_body = event.get_body().decode('utf-8')
        data = json.loads(message_body)
        
        temperature = data.get('temperature')
        humidity = data.get('humidity')
        device_id = data.get('deviceId', 'wio-terminal-01')

        # Pull Configuration
        grafana_url = os.environ.get('GRAFANA_REMOTE_WRITE_URL')
        grafana_user = os.environ.get('GRAFANA_USER')
        grafana_token = os.environ.get('GRAFANA_API_TOKEN')

        if not all([grafana_url, grafana_user, grafana_token]):
            logging.error("CRITICAL ERROR: Missing Grafana configuration.")
            return

        current_time_ms = int(datetime.datetime.now(datetime.timezone.utc).timestamp() * 1000)
        timeseries_list = []

        if temperature is not None:
            ts1 = build_timeseries('iot_temperature_celsius', {'device': device_id}, temperature, current_time_ms)
            timeseries_list.append(ts1)
            
        if humidity is not None:
            ts2 = build_timeseries('iot_humidity_percentage', {'device': device_id}, humidity, current_time_ms)
            timeseries_list.append(ts2)

        if not timeseries_list:
            logging.info("No temperature or humidity data found in message.")
            return

        # Serialize and Compress data for Prometheus
        uncompressed_payload = build_write_request(timeseries_list)
        compressed_payload = snappy.compress(uncompressed_payload)

        headers = {
            "Content-Encoding": "snappy",
            "Content-Type": "application/x-protobuf",
            "X-Prometheus-Remote-Write-Version": "0.1.0",
            "User-Agent": "AzureFunction/1.0"
        }

        logging.info("Attempting to send compressed protobuf to Grafana...")
        response = requests.post(
            grafana_url,
            headers=headers,
            auth=(grafana_user, grafana_token),
            data=compressed_payload,
            timeout=10
        )

        if response.status_code >= 400:
             logging.error(f"FAILED to send data. Grafana Error {response.status_code}: {response.text}")
        else:
             logging.info(f"SUCCESS: Telemetry sent to Grafana! (Status {response.status_code})")

    except Exception as e:
        logging.error(f"Unexpected error: {e}")
        
    logging.info("==== FINISHED EVENT PROCESSING ====\n")