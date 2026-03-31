import argparse
import glob
import os
import re
from pathlib import Path

from typing import Dict, Optional

import numpy as np
import pandas as pd

SENSOR_COLUMNS = ["tof_mm", "ax", "ay", "az", "gx", "gy", "gz"]
NUMERIC_COLUMNS = ["device_ts_s"] + SENSOR_COLUMNS


def _session_id_from_path(path: str) -> str:
    name = Path(path).stem
    match = re.search(r"session\d+", name)
    return match.group(0) if match else name


def _load_events(events_glob: str):
    baseline_map = {}
    recording_start_map = {}
    if not events_glob:
        return baseline_map, recording_start_map

    for path in sorted(glob.glob(events_glob)):
        session_id = _session_id_from_path(path)
        try:
            events = pd.read_csv(path)
        except Exception:
            continue
        if "host_ts" in events.columns:
            events["host_ts"] = pd.to_datetime(events["host_ts"], errors="coerce")
        if "event" not in events.columns:
            continue

        baseline = events.loc[events["event"] == "BASELINE_LOCKED_MM", "value"]
        baseline = pd.to_numeric(baseline, errors="coerce").dropna()
        if not baseline.empty:
            baseline_map[session_id] = float(baseline.iloc[-1])

        rec_start = events.loc[events["event"] == "RECORDING_START", "host_ts"]
        rec_start = rec_start.dropna()
        if not rec_start.empty:
            recording_start_map[session_id] = rec_start.iloc[0]

    return baseline_map, recording_start_map


def _resample_segment(segment: pd.DataFrame, sample_hz: float):
    if segment.empty:
        return segment
    t_min = segment.index.min()
    t_max = segment.index.max()
    if t_max <= t_min:
        return segment
    step = 1.0 / sample_hz
    new_t = np.arange(t_min, t_max + 1e-9, step)

    segment = segment.reindex(segment.index.union(new_t)).sort_index()
    segment[NUMERIC_COLUMNS] = segment[NUMERIC_COLUMNS].interpolate(
        method="linear",
        limit_direction="both",
    )
    segment = segment.loc[new_t]
    segment.index.name = "t_s"

    if "host_ts" in segment.columns:
        host_start = segment["host_ts"].iloc[0]
        if pd.notna(host_start):
            segment["host_ts"] = host_start + pd.to_timedelta(
                segment.index - segment.index[0], unit="s"
            )
    return segment


def _prepare_session(path: str, sample_hz: Optional[float], max_gap_s: Optional[float],
                     recording_start_map: Dict[str, pd.Timestamp]):
    df = pd.read_csv(path)
    if df.empty:
        return df

    for col in NUMERIC_COLUMNS:
        if col in df.columns:
            df[col] = pd.to_numeric(df[col], errors="coerce")

    if "host_ts" in df.columns:
        df["host_ts"] = pd.to_datetime(df["host_ts"], errors="coerce")

    session_id = _session_id_from_path(path)
    rec_start = recording_start_map.get(session_id)
    if rec_start is not None and "host_ts" in df.columns:
        df = df[df["host_ts"] >= rec_start]

    df = df.dropna(subset=["device_ts_s"]).sort_values("device_ts_s")
    df = df.drop_duplicates(subset=["device_ts_s"], keep="first")

    df["t_s"] = df["device_ts_s"] - df["device_ts_s"].iloc[0]
    df["dt_s"] = df["device_ts_s"].diff()

    if max_gap_s is None:
        df["segment_id"] = 0
    else:
        df["segment_id"] = (df["dt_s"] > max_gap_s).cumsum()

    if sample_hz:
        segments = []
        for seg_id, seg in df.groupby("segment_id"):
            seg = seg.set_index("t_s")
            seg = _resample_segment(seg, sample_hz)
            seg["segment_id"] = seg_id
            segments.append(seg.reset_index())
        df = pd.concat(segments, ignore_index=True) if segments else df

    df["session_id"] = session_id
    return df


def _add_features(df: pd.DataFrame) -> pd.DataFrame:
    df = df.copy()
    df["a_mag"] = np.sqrt(df["ax"] ** 2 + df["ay"] ** 2 + df["az"] ** 2)
    df["g_mag"] = np.sqrt(df["gx"] ** 2 + df["gy"] ** 2 + df["gz"] ** 2)

    df["tof_mm_diff"] = df.groupby(["session_id", "segment_id"])["tof_mm"].diff().fillna(0)
    df["a_mag_diff"] = df.groupby(["session_id", "segment_id"])["a_mag"].diff().fillna(0)
    df["g_mag_diff"] = df.groupby(["session_id", "segment_id"])["g_mag"].diff().fillna(0)
    return df


def main():
    parser = argparse.ArgumentParser(description="Prepare pushup session samples for modeling.")
    parser.add_argument("--input-glob", default="data/session*.csv")
    parser.add_argument("--events-glob", default="data/session*.events.csv")
    parser.add_argument("--out-dir", default="data/data_processing/processed")
    parser.add_argument("--sample-hz", type=float, default=10.0,
                        help="Resample to a uniform rate. Set to 0 to disable.")
    parser.add_argument("--max-gap-s", type=float, default=1.0,
                        help="Split segments when device timestamp gaps exceed this.")
    args = parser.parse_args()

    baseline_map, recording_start_map = _load_events(args.events_glob)
    sample_hz = args.sample_hz if args.sample_hz and args.sample_hz > 0 else None

    sessions = []
    for path in sorted(glob.glob(args.input_glob)):
        if path.endswith(".events.csv"):
            continue
        df = _prepare_session(path, sample_hz, args.max_gap_s, recording_start_map)
        if not df.empty:
            sessions.append(df)

    if not sessions:
        raise SystemExit("No session files found. Check --input-glob.")

    combined = pd.concat(sessions, ignore_index=True)
    combined = _add_features(combined)

    if "host_ts" in combined.columns:
        combined["host_ts"] = combined["host_ts"].dt.strftime("%Y-%m-%dT%H:%M:%S.%f")

    os.makedirs(args.out_dir, exist_ok=True)
    out_csv = os.path.join(args.out_dir, "samples.csv")
    out_parquet = os.path.join(args.out_dir, "samples.parquet")

    combined.to_csv(out_csv, index=False)
    try:
        combined.to_parquet(out_parquet, index=False)
    except Exception:
        pass

    print(f"Wrote {len(combined)} rows to {out_csv}")
    if baseline_map:
        print(f"Baseline values loaded for: {', '.join(sorted(baseline_map))}")


if __name__ == "__main__":
    main()
