#ifndef CONFIG_H
#define CONFIG_H

// --- Hardware Pins ---
#define I2C_SDA_PIN         21
#define I2C_SCL_PIN         22
#define BH1750_ADDR_PIN     13
#define DHT11_PIN           27
#define SOIL_PIN            36
#define RESET_BUTTON_PIN    0

// --- Time Settings (NTP) ---
#define NTP_SERVER_1        "pool.ntp.org"
#define NTP_SERVER_2        "time.nist.gov"
#define GMT_OFFSET_SEC      (7 * 3600)
#define DAYLIGHT_OFFSET_SEC 0

// --- Application Settings ---
#define TELEMETRY_INTERVAL_MS   5000
#define RESET_HOLD_TIME_MS      5000

// --- Default Provisioning Settings (WiFiManager) ---
#define DEFAULT_MANAGER_HOST    "your-manager-host.com"
#define DEFAULT_BOOTSTRAP_PORT  "8081"
#define DEFAULT_MQTT_PORT       "8883"
#define WIFI_SETUP_AP_NAME      "PotBuddy-Setup"

#endif
