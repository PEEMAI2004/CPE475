/*
  BH1750 + DHT11 + Analog Soil Moisture Sensor for ESP32-C3 SuperMini
 */

#include <Wire.h>
#include <BH1750FVI.h>
#include <SimpleDHT.h>

// --- BH1750 Settings ---
const int SDA_PIN = 8;
const int SCL_PIN = 9;
uint8_t ADDRESSPIN = 2; 
BH1750FVI::eDeviceAddress_t DEVICEADDRESS = BH1750FVI::k_DevAddress_L; 
BH1750FVI::eDeviceMode_t DEVICEMODE = BH1750FVI::k_DevModeContHighRes;
BH1750FVI LightSensor(ADDRESSPIN, DEVICEADDRESS, DEVICEMODE);

// --- DHT11 Settings ---
int pinDHT11 = 2; 
SimpleDHT11 dht11(pinDHT11);

// --- Soil Moisture Settings ---
const int SOIL_PIN = 0; // Analog pin connected to the sensor

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
  
  Serial.println("Sensors Running!");
  Serial.println("-------------------------");
}

void loop()
{
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

  // Print raw soil data
  Serial.print("Soil Raw: ");
  Serial.println(soilMoistureRaw);

  delay(2000);
} }
}