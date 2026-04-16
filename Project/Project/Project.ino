/*
  BH1750 + DHT11 + Analog Soil Moisture Sensor for ESP32 WROOM 32
  Integrated with MQTT for Telemetry
*/

#include <Wire.h>
#include <BH1750FVI.h>
#include <SimpleDHT.h>
#include <WiFi.h>
#include <PubSubClient.h>
#include "config.h"

// --- BH1750 Settings ---
const int SDA_PIN = 21;
const int SCL_PIN = 22;
uint8_t ADDRESSPIN = 13; 
BH1750FVI::eDeviceAddress_t DEVICEADDRESS = BH1750FVI::k_DevAddress_L; 
BH1750FVI::eDeviceMode_t DEVICEMODE = BH1750FVI::k_DevModeContHighRes;
BH1750FVI LightSensor(ADDRESSPIN, DEVICEADDRESS, DEVICEMODE);

// --- DHT11 Settings ---
int pinDHT11 = 27; 
SimpleDHT11 dht11(pinDHT11);

// --- Soil Moisture Settings ---
const int SOIL_PIN = 36; // Analog pin connected to the sensor

// --- MQTT and WiFi Clients ---
WiFiClient espClient;
PubSubClient client(espClient);
unsigned long lastMsg = 0;

void setup_wifi() {
  delay(10);
  Serial.println();
  Serial.print("Connecting to ");
  Serial.println(WIFI_SSID);

  WiFi.mode(WIFI_STA);
  WiFi.begin(WIFI_SSID, WIFI_PASSWORD);

  while (WiFi.status() != WL_CONNECTED) {
    delay(500);
    Serial.print(".");
  }

  Serial.println("");
  Serial.println("WiFi connected");
  Serial.println("IP address: ");
  Serial.println(WiFi.localIP());
}

void reconnect() {
  // Loop until we're reconnected
  while (!client.connected()) {
    Serial.print("Attempting MQTT connection...");
    // Attempt to connect
    if (client.connect(MQTT_CLIENT_ID, MQTT_USER, MQTT_PASS)) {
      Serial.println("connected");
    } else {
      Serial.print("failed, rc=");
      Serial.print(client.state());
      Serial.println(" try again in 5 seconds");
      // Wait 5 seconds before retrying
      delay(5000);
    }
  }
}

void setup() 
{
  Serial.begin(115200);
  while (!Serial && millis() < 3000) {
    delay(10);
  }

  // Start I2C and Light Sensor
  Wire.begin(SDA_PIN, SCL_PIN);
  LightSensor.begin();  

  // Setup the soil pin to read data
  pinMode(SOIL_PIN, INPUT);
  
  // Setup WiFi and MQTT
  setup_wifi();
  client.setServer(MQTT_BROKER, MQTT_PORT);

  Serial.println("Sensors Running!");
  Serial.println("-------------------------");
}

void loop()
{
  if (!client.connected()) {
    reconnect();
  }
  client.loop();

  unsigned long now = millis();
  if (now - lastMsg > TELEMETRY_FREQUENCY_MS) {
    lastMsg = now;

    // 1. Read Light Level
    uint16_t lux = LightSensor.GetLightIntensity();
    
    // 2. Read DHT11
    byte temperature = 0;
    byte humidity = 0;
    int err = SimpleDHTErrSuccess;
    err = dht11.read(&temperature, &humidity, NULL);
    
    // 3. Read Soil Moisture
    int soilMoistureRaw = analogRead(SOIL_PIN);

    // --- Print Everything to Serial Monitor ---
    Serial.print("Light: ");
    Serial.print(lux);
    Serial.print(" lux  |  ");

    if (err != SimpleDHTErrSuccess) {
      Serial.print("DHT Err!  |  ");
    } else {
      Serial.print("Temp: ");
      Serial.print((int)temperature);
      Serial.print("°C  |  Air Hum: ");
      Serial.print((int)humidity);
      Serial.print("%  |  ");
    }

    Serial.print("Soil Raw: ");
    Serial.println(soilMoistureRaw);

    // --- MQTT Publish ---
    char msg[128];
    if (err == SimpleDHTErrSuccess) {
      snprintf(msg, 128, "{\"light\": %d, \"temp\": %d, \"hum\": %d, \"soil\": %d}", 
               lux, (int)temperature, (int)humidity, soilMoistureRaw);
    } else {
      snprintf(msg, 128, "{\"light\": %d, \"soil\": %d}", 
               lux, soilMoistureRaw);
    }
    
    Serial.print("Publish message: ");
    Serial.println(msg);
    client.publish(MQTT_TOPIC, msg);
  }
}