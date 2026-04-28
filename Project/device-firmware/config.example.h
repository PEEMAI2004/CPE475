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

// --- MQTT Backoff Settings ---
#define MQTT_RECONNECT_INITIAL_BACKOFF_MS 2000
#define MQTT_RECONNECT_MAX_BACKOFF_MS     60000

// --- Default Provisioning Settings (WiFiManager) ---
#define DEFAULT_MANAGER_HOST    "your-manager-host.com"
#define DEFAULT_BOOTSTRAP_PORT  "8081"
#define DEFAULT_MQTT_PORT       "8883"
#define WIFI_SETUP_AP_NAME      "PotBuddy-Setup"

// --- Security (CA Pinning) ---
// Root CA certificate of the Manager API to verify its identity during bootstrapping.
const char* ROOT_CA_CERT = 
"-----BEGIN CERTIFICATE-----\n"
"MIIDPTCCAiWgAwIBAgIUdtZxlftHCy9OCYMTB1vtUfxnd+0wDQYJKoZIhvcNAQEL\n"
"BQAwLjERMA8GA1UECgwIUG90QnVkZHkxGTAXBgNVBAMMEFBvdEJ1ZGR5IFJvb3Qg\n"
"Q0EwHhcNMjYwNDI0MDM1OTU1WhcNMzYwNDIxMDM1OTU1WjAuMREwDwYDVQQKDAhQ\n"
"b3RCdWRkeTEZMBcGA1UEAwwQUG90QnVkZHkgUm9vdCBDQTCCASIwDQYJKoZIhvcN\n"
"AQEBBQADggEPADCCAQoCggEBANRq87autJ0WZyUI8R8cn1CXeDfddL2I76+WZhZD\n"
"ZOFg2RMNYie44YNpwpOm0YYwWZZQgElDbo+UPLrsMy20Xd3DnTTSlw/ieiEUH8Ht\n"
"OsJvdDGkhTPRsYELeoQyNr62BGNLPUC3mHknriT5xhSTOc5ZB8vNe5jLaxpjtmQH\n"
"/PiXyhCZ9U4zVf8d569Voz+MxO+bGApE4fLKkwqGnhHzykeTd50aAOmU2WtfpbaS\n"
"SKVrDW1sVcNIaACV/iBRZtTBaWTwoSFihRcVVmAH7p6OytqCnV948/i9eapLu+zH\n"
"w+O48V0p1tYjMNmsrMoZefgzSc8m9l+rGMPp/Xdr5NUVonECAwEAAaNTMFEwHQYD\n"
"VR0OBBYEFEHNdUvqu9hC8/RgZPUDRHDW7rvDMB8GA1UdIwQYMBaAFEHNdUvqu9hC\n"
"8/RgZPUDRHDW7rvDMA8GA1UdEwEB/wQFMAMBAf8wDQYJKoZIhvcNAQELBQADggEB\n"
"ACjha1zzlu0lOqOoiDrV4zENWzj7fTjAWBcKRdGUMJ9it0j4xuto/b6DrlKlzo3U\n"
"iKiLvPkXPmcv37llGnBj8Z/YAytV7LR3S8z6CXzkiJ5zhU9lqV3mC9vSkk4+ULVt\n"
"/ccTufutfoCPRFYfbnx5inEMku6RdIAD0L4R3ZZpNd0W6ALGD0xL6oN9nlMJgkx+\n"
"Szvb0RkrTJMLdjarQGRGPaaCHQj44YhZRVuLfYOaHTC8vuCAVGWF0n9e5jC2y+aV\n"
"9cohTafX4+u4rA6o2aGYg1hNv3cSoqL2Si8w4JYbTQTLkC8GORyGToBjO9t+120U\n"
"3Vt9iARhmc5zgnI0Ll7LUmk=\n"
"-----END CERTIFICATE-----";

#endif
