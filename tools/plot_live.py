import sys
import serial
import csv
import atexit
import os
from datetime import datetime
from collections import deque
import matplotlib.pyplot as plt
import matplotlib.animation as animation

if len(sys.argv) < 2:
    print("Usage: python3 plot_live.py /dev/cu.usbmodemXXXX [baud] [csv_path]")
    sys.exit(1)

port = sys.argv[1]
baud = int(sys.argv[2]) if len(sys.argv) > 2 else 115200
csv_path = sys.argv[3] if len(sys.argv) > 3 else f"pushup_data_{datetime.now().strftime('%Y%m%d_%H%M%S')}.csv"
csv_dir = os.path.dirname(csv_path)
if csv_dir:
    os.makedirs(csv_dir, exist_ok=True)
events_path = csv_path.rsplit(".", 1)[0] + ".events.csv"

ser = serial.Serial(port, baud, timeout=1)
ser.reset_input_buffer()

window = 200
t = deque(maxlen=window)
tof = deque(maxlen=window)
ax = deque(maxlen=window)
ay = deque(maxlen=window)
az = deque(maxlen=window)
gx = deque(maxlen=window)
gy = deque(maxlen=window)
gz = deque(maxlen=window)

recording = False

csv_file = open(csv_path, "w", newline="")
csv_writer = csv.writer(csv_file)
csv_writer.writerow(["host_ts", "device_ts_s", "tof_mm", "ax", "ay", "az", "gx", "gy", "gz"])
events_file = open(events_path, "w", newline="")
events_writer = csv.writer(events_file)
events_writer.writerow(["host_ts", "device_ts_ms", "event", "value", "state"])
print(f"Recording CSV to: {csv_path}")
print(f"Recording events to: {events_path}")

def _close_csv():
    try:
        csv_file.flush()
        csv_file.close()
    except Exception:
        pass
    try:
        events_file.flush()
        events_file.close()
    except Exception:
        pass

atexit.register(_close_csv)

def parse(line):
    if not line:
        return None
    if line.startswith("timestamp") or line in ("RECORDING", "STOPPED"):
        return None
    parts = line.split(",")
    if len(parts) != 8:
        return None
    try:
        ts, tofmm, axv, ayv, azv, gxv, gyv, gzv = parts
        return (float(ts) / 1000.0, float(tofmm),
                float(axv), float(ayv), float(azv),
                float(gxv), float(gyv), float(gzv))
    except:
        return None

def parse_event(line):
    if not line.startswith("EVENT,"):
        return None
    parts = line.split(",")
    if len(parts) < 4:
        return None
    device_ts_ms = parts[1]
    event_name = parts[2]
    # Accept either EVENT,ts,event,state or EVENT,ts,event,value,state
    if len(parts) == 4:
        value = ""
        state_name = parts[3]
    else:
        value = parts[3]
        state_name = parts[4]
    return device_ts_ms, event_name, value, state_name

fig, (ax1, ax2, ax3) = plt.subplots(3, 1, figsize=(8, 8), sharex=True)
line_tof, = ax1.plot([], [], label="ToF (mm)")
line_ax, = ax2.plot([], [], label="ax")
line_ay, = ax2.plot([], [], label="ay")
line_az, = ax2.plot([], [], label="az")
line_gx, = ax3.plot([], [], label="gx")
line_gy, = ax3.plot([], [], label="gy")
line_gz, = ax3.plot([], [], label="gz")

ax1.set_ylabel("ToF mm")
ax2.set_ylabel("Accel g")
ax3.set_ylabel("Gyro dps")
ax3.set_xlabel("Time (s)")
ax1.legend(loc="upper right")
ax2.legend(loc="upper right")
ax3.legend(loc="upper right")

def on_key(event):
    global recording
    if event.key == 'g':
        ser.write(b"start\n")
        recording = True
        print("Sent: start")
    elif event.key == 'x':
        ser.write(b"stop\n")
        recording = False
        print("Sent: stop")
    elif event.key == ' ':
        if recording:
            ser.write(b"stop\n")
            recording = False
            print("Sent: stop")
        else:
            ser.write(b"start\n")
            recording = True
            print("Sent: start")
    elif event.key == 'q':
        print("Quitting")
        plt.close(fig)

fig.canvas.mpl_connect('key_press_event', on_key)

def update(_):
    line = ser.readline().decode(errors="ignore").strip()
    if line.startswith("EVENT,"):
        ev = parse_event(line)
        if ev:
            global recording
            device_ts_ms, event_name, value, state_name = ev
            events_writer.writerow([datetime.now().isoformat(), device_ts_ms, event_name, value, state_name])
            events_file.flush()
            print(f"[EVENT] {event_name} value={value} state={state_name}")
            if event_name == "RECORDING_START":
                recording = True
            if event_name.startswith("SESSION_STOPPED"):
                recording = False
        return line_tof, line_ax, line_ay, line_az, line_gx, line_gy, line_gz

    if line == "RECORDING":
        recording = True
        return line_tof, line_ax, line_ay, line_az, line_gx, line_gy, line_gz
    if line == "STOPPED":
        recording = False
        return line_tof, line_ax, line_ay, line_az, line_gx, line_gy, line_gz

    parsed = parse(line)
    if parsed:
        ts, tofmm, axv, ayv, azv, gxv, gyv, gzv = parsed
        t.append(ts)
        tof.append(tofmm)
        ax.append(axv)
        ay.append(ayv)
        az.append(azv)
        gx.append(gxv)
        gy.append(gyv)
        gz.append(gzv)

        line_tof.set_data(t, tof)
        line_ax.set_data(t, ax)
        line_ay.set_data(t, ay)
        line_az.set_data(t, az)
        line_gx.set_data(t, gx)
        line_gy.set_data(t, gy)
        line_gz.set_data(t, gz)

        ax1.relim(); ax1.autoscale_view()
        ax2.relim(); ax2.autoscale_view()
        ax3.relim(); ax3.autoscale_view()

        if recording:
            csv_writer.writerow([
                datetime.now().isoformat(),
                f"{ts:.3f}",
                f"{tofmm:.2f}",
                f"{axv:.4f}",
                f"{ayv:.4f}",
                f"{azv:.4f}",
                f"{gxv:.4f}",
                f"{gyv:.4f}",
                f"{gzv:.4f}",
            ])
            csv_file.flush()

    return line_tof, line_ax, line_ay, line_az, line_gx, line_gy, line_gz

ani = animation.FuncAnimation(fig, update, interval=50, blit=False)
plt.tight_layout()
plt.show()
