// Wio Terminal → Azure IoT Hub (simple telemetry only)
// Libraries needed: PubSubClient, DHT sensor library

#include <rpcWiFi.h>
#include <WiFiClientSecure.h>
#include <PubSubClient.h>
#include "DHT.h"
#include "config.h"

#define DHTPIN D5
#define DHTTYPE DHT11

DHT dht(DHTPIN, DHTTYPE);
WiFiClientSecure wifiClient;
PubSubClient mqtt(wifiClient);

static char host[128], deviceId[64];
static char telemetryTopic[128];
static char mqttUsername[256];
static int msgId = 0;
static unsigned long nextSend = 0;

// Parse host and deviceId from connection string
void parseConnStr(const char* cs) {
  auto get = [](const char* s, const char* k, char* out) {
    const char* p = strstr(s, k) + strlen(k);
    const char* e = strchr(p, ';');
    if (e) { strncpy(out, p, e - p); out[e - p] = '\0'; }
    else strcpy(out, p);
  };
  get(cs, "HostName=", host);
  get(cs, "DeviceId=", deviceId);
  snprintf(telemetryTopic, sizeof(telemetryTopic), "devices/%s/messages/events/", deviceId);
  snprintf(mqttUsername, sizeof(mqttUsername), "%s/%s/?api-version=2021-04-12", host, deviceId);
}

void setup() {
  Serial.begin(115200);
  while (!Serial);
  dht.begin();
  parseConnStr(IOT_HUB_CONNECTION_STRING);

  // WiFi
  Serial.print("WiFi...");
  WiFi.begin(WIFI_SSID, WIFI_PASSWORD);
  while (WiFi.status() != WL_CONNECTED) { delay(500); Serial.print("."); }
  Serial.println(WiFi.localIP());

  // MQTT
  mqtt.setServer(host, 8883);
  mqtt.setBufferSize(1024);

  while (!mqtt.connected()) {
    Serial.print("MQTT...");
    if (mqtt.connect(deviceId, mqttUsername, IOT_SAS_TOKEN)) {
      Serial.println("connected!");
    } else {
      Serial.print("fail("); Serial.print(mqtt.state()); Serial.println(") retry 5s");
      delay(5000);
    }
  }
}

void loop() {
  if (millis() > nextSend) {
    if (!mqtt.connected()) {
      if (mqtt.connect(deviceId, mqttUsername, IOT_SAS_TOKEN))
        Serial.println("MQTT reconnected");
    }

    // float t = dht.readTemperature(), h = dht.readHumidity();
    float t = 21, h = 10;
    if (!isnan(t) && !isnan(h)) {
      msgId++;
      char payload[128];
      snprintf(payload, sizeof(payload),
        "{\"messageId\":%d,\"deviceId\":\"Device_0\",\"temperature\":%.2f,\"humidity\":%.2f}",
        msgId, t, h);
      Serial.print("Send: "); Serial.println(payload);
      mqtt.publish(telemetryTopic, payload);
    }
    nextSend = millis() + TELEMETRY_FREQUENCY_MS;
  }
  mqtt.loop();
  delay(10);
}