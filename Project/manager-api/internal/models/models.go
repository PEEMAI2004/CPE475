package models

import (
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type Profile struct {
	ID             int      `json:"id"`
	Name           string   `json:"name"`
	SoilInnerLow   *float64 `json:"soil_inner_low"`
	SoilInnerHigh  *float64 `json:"soil_inner_high"`
	SoilOuterLow   *float64 `json:"soil_outer_low"`
	SoilOuterHigh  *float64 `json:"soil_outer_high"`
	TempInnerLow   *float64 `json:"temp_inner_low"`
	TempInnerHigh  *float64 `json:"temp_inner_high"`
	TempOuterLow   *float64 `json:"temp_outer_low"`
	TempOuterHigh  *float64 `json:"temp_outer_high"`
	HumInnerLow    *float64 `json:"hum_inner_low"`
	HumInnerHigh   *float64 `json:"hum_inner_high"`
	HumOuterLow    *float64 `json:"hum_outer_low"`
	HumOuterHigh   *float64 `json:"hum_outer_high"`
	LightInnerLow  *float64 `json:"light_inner_low"`
	LightInnerHigh *float64 `json:"light_inner_high"`
	LightOuterLow  *float64 `json:"light_outer_low"`
	LightOuterHigh *float64 `json:"light_outer_high"`
}

type Device struct {
	DeviceID  string `json:"device_id"`
	ProfileID int    `json:"profile_id"`
	Online    bool   `json:"online"`
	Health    string `json:"health"`
}

type User struct {
	ID        int       `json:"id"`
	Email     string    `json:"email"`
	Name      string    `json:"name"`
	Role      string    `json:"role"`
	CreatedAt time.Time `json:"created_at"`
}

type InfrastructureNode struct {
	ID          int       `json:"id"`
	Name        string    `json:"name"`
	Type        string    `json:"type"` // Kept for DB compatibility, but defaults to "Local Node"
	SiteID      int       `json:"site_id"`
	Address     string    `json:"address"`
	MQTTAddress string    `json:"mqtt_address"`
	Token       string    `json:"token"`
	CreatedAt   time.Time `json:"created_at"`
}

type EnrolledDevice struct {
	DeviceID  string    `json:"device_id"`
	AuthToken string    `json:"auth_token"`
	CreatedAt time.Time `json:"created_at"`
}

type Claims struct {
	Email string `json:"email"`
	Role  string `json:"role"`
	jwt.RegisteredClaims
}

type MachineIdentity struct {
	ID   string
	Type string // "node" or "device"
}
