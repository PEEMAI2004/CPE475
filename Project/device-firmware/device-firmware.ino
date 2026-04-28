#include <WiFi.h>
#include <WiFiManager.h>
#include <WiFiClientSecure.h>
#include <HTTPClient.h>
#include <PubSubClient.h>
#include <ArduinoJson.h>
#include <LittleFS.h>
#include <Wire.h>
#include <BH1750FVI.h>
#include <SimpleDHT.h>
#include <time.h>
#include "config.h"

// --- BH1750 Settings ---
BH1750FVI::eDeviceAddress_t DEVICEADDRESS = BH1750FVI::k_DevAddress_L; 
BH1750FVI::eDeviceMode_t DEVICEMODE = BH1750FVI::k_DevModeContHighRes;
BH1750FVI LightSensor(BH1750_ADDR_PIN, DEVICEADDRESS, DEVICEMODE);

// --- DHT11 Settings ---
SimpleDHT11 dht11(DHT11_PIN);

WiFiClientSecure secureClient;
PubSubClient mqttClient(secureClient);

String device_id = "";
String manager_host = "";
String mqtt_host = "";
int bootstrap_port = String(DEFAULT_BOOTSTRAP_PORT).toInt();
int mqtt_port = String(DEFAULT_MQTT_PORT).toInt();
String ca_crt = "";
String client_crt = "";
String client_key = "";

unsigned long lastMsg = 0;
unsigned long mqtt_backoff_ms = MQTT_RECONNECT_INITIAL_BACKOFF_MS;

// --- Change Detection & Heartbeat ---
uint16_t last_lux = 0;
int last_temp = 0;
int last_hum = 0;
int last_soil = 0;
unsigned long last_send_time = 0;

void syncTime() {
  configTime(GMT_OFFSET_SEC, DAYLIGHT_OFFSET_SEC, NTP_SERVER_1, NTP_SERVER_2);
  Serial.print("Waiting for NTP time sync: ");
  time_t now = time(nullptr);
  while (now < 8 * 3600 * 2) {
    delay(500);
    Serial.print(".");
    now = time(nullptr);
  }
  Serial.println("");
  struct tm timeinfo;
  gmtime_r(&now, &timeinfo);
  Serial.print("Current time: "); Serial.print(asctime(&timeinfo));
}

bool loadConfig() {
  if (!LittleFS.exists("/device_id.txt") || !LittleFS.exists("/ca.crt") || 
      !LittleFS.exists("/client.crt") || !LittleFS.exists("/client.key") ||
      !LittleFS.exists("/manager_host.txt")) {
    return false;
  }
  File f;
  f = LittleFS.open("/device_id.txt", "r"); device_id = f.readString(); f.close();
  device_id.trim(); // Remove any potential newlines or spaces
  f = LittleFS.open("/manager_host.txt", "r"); manager_host = f.readString(); f.close();
  manager_host.trim();
  f = LittleFS.open("/ca.crt", "r"); ca_crt = f.readString(); f.close();
  f = LittleFS.open("/client.crt", "r"); client_crt = f.readString(); f.close();
  f = LittleFS.open("/client.key", "r"); client_key = f.readString(); f.close();
  
  if (LittleFS.exists("/mqtt_host.txt")) {
    f = LittleFS.open("/mqtt_host.txt", "r"); mqtt_host = f.readString(); f.close();
  }
  if (LittleFS.exists("/b_port.txt")) {
    f = LittleFS.open("/b_port.txt", "r"); bootstrap_port = f.readString().toInt(); f.close();
  }
  if (LittleFS.exists("/m_port.txt")) {
    f = LittleFS.open("/m_port.txt", "r"); mqtt_port = f.readString().toInt(); f.close();
  }
  
  return true;
}

void saveString(const char* path, String data) {
  File f = LittleFS.open(path, "w");
  if (f) { f.print(data); f.close(); }
}

bool performBootstrap(String host, int b_port, String token) {
  host.trim();
  token.trim();
  HTTPClient http;
  WiFiClient plainClient;
  WiFiClientSecure httpsClient;
  String url;
  
  if (host.startsWith("http://") || host.startsWith("https://")) {
    url = host + "/api/enrollment/bootstrap";
  } else {
    String scheme = (b_port == 443) ? "https://" : "http://";
    url = scheme + host + ":" + String(b_port) + "/api/enrollment/bootstrap";
  }

  Serial.println("Bootstrapping to: " + url);

  if (url.startsWith("https://")) {
    httpsClient.setCACert(ROOT_CA_CERT); 
    http.begin(httpsClient, url);
  } else {
    http.begin(plainClient, url);
  }
  
  http.addHeader("Content-Type", "application/json");
  String payload = "{\"auth_token\":\"" + token + "\"}";
  
  int httpResponseCode = http.POST(payload);
  if (httpResponseCode == 200 || httpResponseCode == 201) {
    String response = http.getString();
    JsonDocument doc;
    deserializeJson(doc, response);
    
    device_id = doc["device_id"].as<String>();
    ca_crt = doc["ca.crt"].as<String>();
    client_crt = doc["client.crt"].as<String>();
    client_key = doc["client.key"].as<String>();
    manager_host = host;
    
    if (manager_host.startsWith("https://")) manager_host.remove(0, 8);
    else if (manager_host.startsWith("http://")) manager_host.remove(0, 7);
    if (manager_host.endsWith("/")) manager_host.remove(manager_host.length()-1);
    
    saveString("/device_id.txt", device_id);
    saveString("/manager_host.txt", manager_host);
    saveString("/ca.crt", ca_crt);
    saveString("/client.crt", client_crt);
    saveString("/client.key", client_key);
    saveString("/b_port.txt", String(b_port));
    saveString("/m_port.txt", String(mqtt_port));
    
    Serial.println("Bootstrap successful!");
    http.end();
    return true;
  } else {
    Serial.print("Bootstrap failed. HTTP Code: "); Serial.println(httpResponseCode);
    Serial.println(http.getString());
    http.end();
    return false;
  }
}

void handleManualReset() {
  Serial.println("!!! Manual Reset Triggered. Wiping configuration and WiFi settings...");
  WiFiManager wm;
  wm.resetSettings();
  LittleFS.format();
  Serial.println("Wipe complete. Restarting...");
  delay(1000);
  ESP.restart();
}

void checkResetButton() {
  if (digitalRead(RESET_BUTTON_PIN) == LOW) {
    unsigned long holdStart = millis();
    Serial.println("Reset button pressed. Keep holding for 5 seconds to reset...");
    while (digitalRead(RESET_BUTTON_PIN) == LOW) {
      if (millis() - holdStart > RESET_HOLD_TIME_MS) {
        handleManualReset();
      }
      delay(100);
    }
  }
}

void setup() {
  Serial.begin(115200);
  pinMode(RESET_BUTTON_PIN, INPUT_PULLUP);
  
  if (!LittleFS.begin(true)) { Serial.println("LittleFS Mount Failed"); return; }

  Serial.println("--- PotBuddy Booting ---");
  checkResetButton();

  bool isConfigured = loadConfig();
  WiFiManager wm;
  
  if (!isConfigured) {
    WiFiManagerParameter custom_host("host", "Manager Host (URL or IP)", DEFAULT_MANAGER_HOST, 60);
    WiFiManagerParameter custom_b_port("b_port", "Bootstrap Port (8081/443)", DEFAULT_BOOTSTRAP_PORT, 6);
    WiFiManagerParameter custom_mqtt_host("mqtt_host", "MQTT Host (Empty = Manager)", "", 60);
    WiFiManagerParameter custom_m_port("m_port", "MQTT mTLS Port", DEFAULT_MQTT_PORT, 6);
    WiFiManagerParameter custom_token("token", "Device Auth Token", "", 64);
    
    wm.addParameter(&custom_host);
    wm.addParameter(&custom_b_port);
    wm.addParameter(&custom_mqtt_host);
    wm.addParameter(&custom_m_port);
    wm.addParameter(&custom_token);
    
    if (!wm.autoConnect(WIFI_SETUP_AP_NAME)) { delay(3000); ESP.restart(); }
    
    String host_val = custom_host.getValue();
    int b_port = String(custom_b_port.getValue()).toInt();
    mqtt_host = custom_mqtt_host.getValue();
    mqtt_port = String(custom_m_port.getValue()).toInt();
    String token = custom_token.getValue();
    
    if (host_val != "" && token != "") {
      saveString("/mqtt_host.txt", mqtt_host);
      if (!performBootstrap(host_val, b_port, token)) {
        wm.resetSettings(); LittleFS.format(); delay(3000); ESP.restart();
      }
    }
  } else {
    if (!wm.autoConnect(WIFI_SETUP_AP_NAME)) { delay(3000); ESP.restart(); }
  }
  
  syncTime(); // Required for mTLS certificate validation
  
  String target_mqtt = (mqtt_host != "") ? mqtt_host : manager_host;
  Serial.println("Setting up mTLS for " + target_mqtt);
  
  secureClient.setCACert(ca_crt.c_str());
  secureClient.setCertificate(client_crt.c_str());
  secureClient.setPrivateKey(client_key.c_str());
  
  mqttClient.setServer(target_mqtt.c_str(), mqtt_port);
  Wire.begin(I2C_SDA_PIN, I2C_SCL_PIN);
  LightSensor.begin();  
  pinMode(SOIL_PIN, INPUT);
  Serial.println("Secure Telemetry Ready!");
}

void reconnect() {
  String target_mqtt = (mqtt_host != "") ? mqtt_host : manager_host;
  while (!mqttClient.connected()) {
    checkResetButton();
    Serial.print("Connecting MQTT to "); Serial.print(target_mqtt);
    if (mqttClient.connect(device_id.c_str())) { 
      Serial.println(" - connected"); 
      mqtt_backoff_ms = MQTT_RECONNECT_INITIAL_BACKOFF_MS; // Reset backoff on success
    }
    else { 
      Serial.print(" - failed, rc="); Serial.println(mqttClient.state()); 
      Serial.print("Next attempt in "); Serial.print(mqtt_backoff_ms / 1000); Serial.println(" seconds...");
      
      unsigned long start_wait = millis();
      while (millis() - start_wait < mqtt_backoff_ms) {
        checkResetButton();
        delay(100);
      }
      
      // Increase backoff for next time, capped at max
      mqtt_backoff_ms *= 2;
      if (mqtt_backoff_ms > MQTT_RECONNECT_MAX_BACKOFF_MS) {
        mqtt_backoff_ms = MQTT_RECONNECT_MAX_BACKOFF_MS;
      }
    }
  }
}

void loop() {
  checkResetButton();
  if (!mqttClient.connected()) reconnect();
  mqttClient.loop();

  unsigned long now = millis();
  if (now - lastMsg > TELEMETRY_INTERVAL_MS) {
    lastMsg = now;

    // Read sensors
    uint16_t lux = LightSensor.GetLightIntensity();
    byte temperature = 0, humidity = 0;
    int err = dht11.read(&temperature, &humidity, NULL);
    int soil = analogRead(SOIL_PIN);

    // Change detection logic
    bool changed = (lux != last_lux) || (abs(soil - last_soil) > 50); // Small threshold for analog soil noise
    if (err == SimpleDHTErrSuccess) {
        changed = changed || ((int)temperature != last_temp) || ((int)humidity != last_hum);
    }

    // Heartbeat logic (ensure we don't time out in the dashboard)
    bool heartbeat = (now - last_send_time > MAX_SILENT_INTERVAL_MS);

    if (changed || heartbeat) {
        last_lux = lux;
        last_soil = soil;
        if (err == SimpleDHTErrSuccess) {
            last_temp = (int)temperature;
            last_hum = (int)humidity;
        }
        last_send_time = now;

        JsonDocument doc;
        doc["device_id"] = device_id;
        doc["light"] = lux;
        if (err == SimpleDHTErrSuccess) { doc["temp"] = (int)temperature; doc["hum"] = (int)humidity; }
        doc["soil"] = soil;

        char msg[256];
        serializeJson(doc, msg);
        String topic = "potbuddy/" + device_id + "/raw";
        mqttClient.publish(topic.c_str(), msg);

        Serial.print("Published (reason="); 
        Serial.print(changed ? "change" : "heartbeat"); 
        Serial.print("): "); 
        Serial.println(msg);
    }
  }
}
