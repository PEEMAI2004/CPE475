CREATE TABLE profiles (
    id SERIAL PRIMARY KEY,
    name VARCHAR(50) UNIQUE NOT NULL,
    soil_inner_low FLOAT, soil_inner_high FLOAT, 
    soil_outer_low FLOAT, soil_outer_high FLOAT,
    temp_inner_low FLOAT, temp_inner_high FLOAT, 
    temp_outer_low FLOAT, temp_outer_high FLOAT,
    hum_inner_low FLOAT, hum_inner_high FLOAT, 
    hum_outer_low FLOAT, hum_outer_high FLOAT,
    light_inner_low FLOAT, light_inner_high FLOAT, 
    light_outer_low FLOAT, light_outer_high FLOAT
);

CREATE TABLE device_profiles (
    device_id VARCHAR(50) PRIMARY KEY,
    profile_id INT REFERENCES profiles(id) ON DELETE CASCADE
);

CREATE TABLE users (
    id SERIAL PRIMARY KEY,
    email VARCHAR(255) UNIQUE NOT NULL,
    name VARCHAR(255),
    role VARCHAR(50) NOT NULL CHECK (role IN ('Super Admin', 'Site Admin', 'Viewer')),
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE user_sites (
    user_id INT REFERENCES users(id) ON DELETE CASCADE,
    site_id INT NOT NULL, -- Logical site ID (0, 1, etc.)
    PRIMARY KEY (user_id, site_id)
);

-- Insert a default baseline profile covering standard plants
INSERT INTO profiles (name, 
  soil_inner_low, soil_inner_high, soil_outer_low, soil_outer_high,
  temp_inner_low, temp_inner_high, temp_outer_low, temp_outer_high,
  hum_inner_low, hum_inner_high, hum_outer_low, hum_outer_high,
  light_inner_low, light_inner_high, light_outer_low, light_outer_high
) VALUES (
  'default',
  1500, 2500, 1000, 3000,
  18, 30, 15, 35,
  40, 70, 30, 80,
  2000, 50000, 500, 80000
);

-- Note: In production, the first Super Admin should be added via a migration or initial setup script.
-- For this environment, we'll assume the user will be added via the database or a setup tool.
