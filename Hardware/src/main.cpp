#include <Wire.h>
#include <Arduino_BMI270_BMM150.h>
#include <Adafruit_VL53L1X.h>

#define IRQ_PIN 2
#define XSHUT_PIN 3

Adafruit_VL53L1X vl53 = Adafruit_VL53L1X(XSHUT_PIN, IRQ_PIN);

// Forward declaration
void handleSerial();
bool sampleSensors(float &tofRaw, float &tofSmooth, float &ax, float &ay, float &az, float &gx, float &gy, float &gz);
void emitEvent(const char *name);
void emitEventValue(const char *name, float value);
const char *stateName();

enum State { IDLE, ARMING, COUNTDOWN, RECORDING, END_HOLD };
State state = IDLE;

const unsigned long SAMPLE_MS = 100;
unsigned long lastSample = 0;
const unsigned long ARMING_MS = 2000;
const unsigned long COUNTDOWN_MS = 5000;
const unsigned long SESSION_MS = 60000;
const unsigned long END_HOLD_MS = 2000;
const unsigned long END_HOLD_TIMEOUT_MS = 5000;
const float STABLE_RANGE_MM = 25.0f;
const float TOF_EMA_ALPHA = 0.30f;

unsigned long phaseStartMs = 0;
float baselineTof = 0.0f;
bool baselineLocked = false;
bool csvHeaderPrinted = false;

float tofEma = 0.0f;
bool tofEmaInit = false;
float stableMin = 100000.0f;
float stableMax = -100000.0f;

bool stopRequested = false;

void resetStabilityWindow(float currentValue) {
  stableMin = currentValue;
  stableMax = currentValue;
}

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

  if (state == IDLE) {
    delay(10);
    return;
  }

  if (millis() - lastSample < SAMPLE_MS) return;
  lastSample = millis();

  float tofRaw = 0, tofSmooth = 0;
  float ax = 0, ay = 0, az = 0, gx = 0, gy = 0, gz = 0;
  if (!sampleSensors(tofRaw, tofSmooth, ax, ay, az, gx, gy, gz)) {
    return;
  }

  stableMin = min(stableMin, tofSmooth);
  stableMax = max(stableMax, tofSmooth);

  if (state == ARMING) {
    if (millis() - phaseStartMs >= ARMING_MS) {
      float span = stableMax - stableMin;
      if (span <= STABLE_RANGE_MM) {
        baselineTof = tofSmooth;
        baselineLocked = true;
        emitEventValue("BASELINE_LOCKED_MM", baselineTof);
        state = COUNTDOWN;
        phaseStartMs = millis();
        emitEvent("COUNTDOWN_START");
      } else {
        emitEventValue("HOLD_STILL_SPAN_MM", span);
        phaseStartMs = millis();
      }
      resetStabilityWindow(tofSmooth);
    }
    return;
  }

  if (state == COUNTDOWN) {
    if (millis() - phaseStartMs >= COUNTDOWN_MS) {
      state = RECORDING;
      phaseStartMs = millis();
      csvHeaderPrinted = false;
      emitEvent("RECORDING_START");
    }
    return;
  }

  if (state == RECORDING) {
    if (!csvHeaderPrinted) {
      Serial.println("timestamp_ms,tof_mm,ax,ay,az,gx,gy,gz");
      Serial.println("RECORDING");
      csvHeaderPrinted = true;
    }

    Serial.print(millis());
    Serial.print(",");
    Serial.print(tofRaw, 2);
    Serial.print(",");
    Serial.print(ax, 4); Serial.print(",");
    Serial.print(ay, 4); Serial.print(",");
    Serial.print(az, 4); Serial.print(",");
    Serial.print(gx, 4); Serial.print(",");
    Serial.print(gy, 4); Serial.print(",");
    Serial.println(gz, 4);

    if (stopRequested || (millis() - phaseStartMs >= SESSION_MS)) {
      state = END_HOLD;
      phaseStartMs = millis();
      stopRequested = false;
      resetStabilityWindow(tofSmooth);
      emitEvent("END_HOLD_START");
    }
    return;
  }

  if (state == END_HOLD) {
    if (millis() - phaseStartMs >= END_HOLD_MS) {
      float span = stableMax - stableMin;
      if (span <= STABLE_RANGE_MM) {
        emitEventValue("SESSION_STOPPED_CLEAN_SPAN_MM", span);
        Serial.println("STOPPED");
        state = IDLE;
        baselineLocked = false;
        tofEmaInit = false;
      } else if (millis() - phaseStartMs >= END_HOLD_TIMEOUT_MS) {
        emitEventValue("SESSION_STOPPED_TIMEOUT_SPAN_MM", span);
        Serial.println("STOPPED");
        state = IDLE;
        baselineLocked = false;
        tofEmaInit = false;
      }
    }
  }
}

void handleSerial() {
  if (!Serial.available()) return;

  String cmd = Serial.readStringUntil('\n');
  cmd.trim();

  if (cmd.equalsIgnoreCase("start")) {
    state = ARMING;
    phaseStartMs = millis();
    baselineLocked = false;
    csvHeaderPrinted = false;
    stopRequested = false;
    emitEvent("START_CMD");
    emitEvent("ARMING_START");
  } else if (cmd.equalsIgnoreCase("stop")) {
    if (state == RECORDING) {
      stopRequested = true;
      emitEvent("STOP_CMD");
    } else {
      state = IDLE;
      emitEvent("STOP_CMD_IDLE");
      Serial.println("STOPPED");
    }
  }
}

bool sampleSensors(float &tofRaw, float &tofSmooth, float &ax, float &ay, float &az, float &gx, float &gy, float &gz) {
  if (!vl53.dataReady()) return false;

  int16_t tof = vl53.distance();
  if (tof == -1) {
    emitEventValue("TOF_ERROR", (float)vl53.vl_status);
    vl53.clearInterrupt();
    return false;
  }
  vl53.clearInterrupt();

  tofRaw = (float)tof;
  if (!tofEmaInit) {
    tofEma = tofRaw;
    tofEmaInit = true;
    resetStabilityWindow(tofEma);
  } else {
    tofEma = (TOF_EMA_ALPHA * tofRaw) + ((1.0f - TOF_EMA_ALPHA) * tofEma);
  }
  tofSmooth = tofEma;

  ax = ay = az = gx = gy = gz = 0.0f;
  if (IMU.accelerationAvailable()) {
    IMU.readAcceleration(ax, ay, az);
  }
  if (IMU.gyroscopeAvailable()) {
    IMU.readGyroscope(gx, gy, gz);
  }
  return true;
}

void emitEvent(const char *name) {
  Serial.print("EVENT,");
  Serial.print(millis());
  Serial.print(",");
  Serial.print(name);
  Serial.print(",");
  Serial.println(stateName());
}

void emitEventValue(const char *name, float value) {
  Serial.print("EVENT,");
  Serial.print(millis());
  Serial.print(",");
  Serial.print(name);
  Serial.print(",");
  Serial.print(value, 2);
  Serial.print(",");
  Serial.println(stateName());
}

const char *stateName() {
  switch (state) {
    case IDLE: return "IDLE";
    case ARMING: return "ARMING";
    case COUNTDOWN: return "COUNTDOWN";
    case RECORDING: return "RECORDING";
    case END_HOLD: return "END_HOLD";
    default: return "UNKNOWN";
  }
}
