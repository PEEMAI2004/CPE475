#include "DHT.h"
#include <SPI.h>
#include <TFT_eSPI.h>

#define DHTPIN D5     
#define DHTTYPE DHT11

DHT dht(DHTPIN, DHTTYPE);
TFT_eSPI tft = TFT_eSPI();  

void drawInterface();
void updateDisplayValues(float temp, float hum);

void setup() {
  Serial.begin(115200);
  
  // Initialize Hardware
  dht.begin();
  tft.begin();
  tft.setRotation(3); // Landscape mode
  
  // Draw the static UI
  drawInterface();
}

void loop() {
  // Read Data
  // float temperature = dht.readTemperature();
  // float humidity = dht.readHumidity();

  // For Mock Data
  float temperature = 3.14;
  float humidity = 22.7;

  // Check for errors
  if (isnan(humidity) || isnan(temperature)) {
    Serial.println("Failed to read from DHT11!");
    // Optional: Draw error on screen if needed
    return;
  }

  // Debug to Serial
  Serial.print("T: "); Serial.print(temperature);
  Serial.print(" | H: "); Serial.println(humidity);

  // Update the screen
  updateDisplayValues(temperature, humidity);

  delay(1000); 
}

void drawInterface() {
  tft.fillScreen(TFT_BLACK); 
  
  // Title
  tft.setTextColor(TFT_WHITE);
  tft.setTextSize(2);
  tft.setCursor(80, 20);
  tft.print("Wio Monitor");
  
  // Separator Line
  tft.drawLine(20, 50, 300, 50, TFT_BLUE);

  // Labels
  tft.setTextColor(TFT_LIGHTGREY);
  tft.setTextSize(2);
  
  tft.setCursor(40, 80);
  tft.print("Temperature:");

  tft.setCursor(40, 160);
  tft.print("Humidity:");
}

void updateDisplayValues(float temp, float hum) {
  
  // --- Update Temperature ---
  // Clear previous number (draw black box over it)
  tft.fillRect(40, 110, 200, 30, TFT_BLACK); 
  
  // Write new number
  tft.setTextColor(TFT_ORANGE);
  tft.setTextSize(3);
  tft.setCursor(40, 110);
  tft.print(temp);
  tft.setTextSize(2);
  tft.print(" C");

  // --- Update Humidity ---
  // Clear previous number
  tft.fillRect(40, 190, 200, 30, TFT_BLACK);
  
  // Write new number
  tft.setTextColor(TFT_CYAN);
  tft.setTextSize(3);
  tft.setCursor(40, 190);
  tft.print(hum);
  tft.setTextSize(2);
  tft.print(" %");
}