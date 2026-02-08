#include <Wire.h>
#include <Arduino_BMI270_BMM150.h>
#include <Adafruit_VL53L1X.h>

#define IRQ_PIN 2
#define XSHUT_PIN 3

Adafruit_VL53L1X vl53 = Adafruit_VL53L1X(XSHUT_PIN, IRQ_PIN);

// Forward declaration
void handleSerial();

enum State { IDLE, RECORDING };
State state = IDLE;

const unsigned long SAMPLE_MS = 100;
unsigned long lastSample = 0;

void setup() {
  Serial.begin(115200);
  while (!Serial) { delay(10); }

  Serial.println(F("Pressle sensor logger"));
  Serial.println(F("Type: start / stop"));

  Wire.begin();
  Wire.setClock(400000);

  if (!IMU.begin()) {
    Serial.println("IMU not found");
    while (1) { delay(10); }
  }

  if (!vl53.begin(0x29, &Wire)) {
    Serial.print(F("Error on init of VL sensor: "));
    Serial.println(vl53.vl_status);
    while (1) { delay(10); }
  }
  Serial.println(F("VL53L1X sensor OK!"));

  if (!vl53.startRanging()) {
    Serial.print(F("Couldn't start ranging: "));
    Serial.println(vl53.vl_status);
    while (1) { delay(10); }
  }
  Serial.println(F("Ranging started"));

  vl53.setTimingBudget(50);
  Serial.print(F("Timing budget (ms): "));
  Serial.println(vl53.getTimingBudget());
}

void loop() {
  handleSerial();

  if (state != RECORDING) {
    delay(10);
    return;
  }

  if (millis() - lastSample < SAMPLE_MS) return;
  lastSample = millis();

  if (!vl53.dataReady()) return;

  int16_t tof = vl53.distance();
  if (tof == -1) {
    Serial.print(F("TOF error: "));
    Serial.println(vl53.vl_status);
    vl53.clearInterrupt();
    return;
  }
  vl53.clearInterrupt();

  float ax = 0, ay = 0, az = 0, gx = 0, gy = 0, gz = 0;

  if (IMU.accelerationAvailable()) {
    IMU.readAcceleration(ax, ay, az);
  }
  if (IMU.gyroscopeAvailable()) {
    IMU.readGyroscope(gx, gy, gz);
  }

  Serial.print(millis());
  Serial.print(",");
  Serial.print(tof);
  Serial.print(",");
  Serial.print(ax, 4); Serial.print(",");
  Serial.print(ay, 4); Serial.print(",");
  Serial.print(az, 4); Serial.print(",");
  Serial.print(gx, 4); Serial.print(",");
  Serial.print(gy, 4); Serial.print(",");
  Serial.println(gz, 4);
}

void handleSerial() {
  if (!Serial.available()) return;

  String cmd = Serial.readStringUntil('\n');
  cmd.trim();

  if (cmd.equalsIgnoreCase("start")) {
    state = RECORDING;
    Serial.println("timestamp_ms,tof_mm,ax,ay,az,gx,gy,gz");
    Serial.println("RECORDING");
  } else if (cmd.equalsIgnoreCase("stop")) {
    state = IDLE;
    Serial.println("STOPPED");
  }
}
