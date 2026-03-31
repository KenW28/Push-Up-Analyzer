import argparse
import glob
import os
import re
from pathlib import Path

from typing import Dict

import numpy as np
import pandas as pd


FEATURE_COLUMNS = [
    "tof_mm",
    "ax",
    "ay",
    "az",
    "gx",
    "gy",
    "gz",
    "a_mag",
    "g_mag",
    "tof_mm_diff",
    "a_mag_diff",
    "g_mag_diff",
]


def _session_id_from_path(path: str) -> str:
    name = Path(path).stem
    match = re.search(r"session\d+", name)
    return match.group(0) if match else name


def _load_baselines(events_glob: str) -> Dict[str, float]:
    baseline_map = {}
    if not events_glob:
        return baseline_map

    for path in sorted(glob.glob(events_glob)):
        session_id = _session_id_from_path(path)
        try:
            events = pd.read_csv(path)
        except Exception:
            continue
        if "event" not in events.columns or "value" not in events.columns:
            continue
        baseline = events.loc[events["event"] == "BASELINE_LOCKED_MM", "value"]
        baseline = pd.to_numeric(baseline, errors="coerce").dropna()
        if not baseline.empty:
            baseline_map[session_id] = float(baseline.iloc[-1])
    return baseline_map


def _window_stats(window: pd.DataFrame) -> dict:
    stats = {}
    for col in FEATURE_COLUMNS:
        if col not in window.columns:
            continue
        series = window[col]
        stats[f"{col}_mean"] = float(series.mean())
        stats[f"{col}_std"] = float(series.std(ddof=0))
        stats[f"{col}_min"] = float(series.min())
        stats[f"{col}_max"] = float(series.max())
        stats[f"{col}_range"] = float(series.max() - series.min())
    return stats


def build_windows(df: pd.DataFrame, window_s: float, step_s: float,
                  min_samples: int, baseline_map: Dict[str, float],
                  label_threshold_mm: float):
    rows = []
    for (session_id, segment_id), group in df.groupby(["session_id", "segment_id"]):
        if group.empty:
            continue
        group = group.sort_values("t_s")
        t_min = group["t_s"].min()
        t_max = group["t_s"].max()
        if t_max - t_min < window_s:
            continue

        baseline = baseline_map.get(session_id)
        if baseline is None:
            baseline = float(group["tof_mm"].quantile(0.9))
            baseline_source = "p90"
        else:
            baseline_source = "baseline_locked"

        start = t_min
        while start + window_s <= t_max + 1e-9:
            end = start + window_s
            window = group[(group["t_s"] >= start) & (group["t_s"] < end)]
            if len(window) >= min_samples:
                stats = _window_stats(window)
                min_tof = float(window["tof_mm"].min())
                depth_mm = float(baseline - min_tof)
                label_pushup = 1 if depth_mm >= label_threshold_mm else 0

                row = {
                    "session_id": session_id,
                    "segment_id": int(segment_id),
                    "window_start_s": float(start),
                    "window_end_s": float(end),
                    "baseline_mm": float(baseline),
                    "baseline_source": baseline_source,
                    "depth_mm": depth_mm,
                    "label_pushup": label_pushup,
                }
                row.update(stats)
                rows.append(row)
            start += step_s

    return pd.DataFrame(rows)


def main():
    parser = argparse.ArgumentParser(description="Build windowed features for pushup classification.")
    parser.add_argument("--samples", default="data/data_processing/processed/samples.csv")
    parser.add_argument("--events-glob", default="data/session*.events.csv")
    parser.add_argument("--out", default="data/data_processing/processed/windows.csv")
    parser.add_argument("--window-s", type=float, default=2.0)
    parser.add_argument("--step-s", type=float, default=0.5)
    parser.add_argument("--min-samples", type=int, default=10)
    parser.add_argument("--label-threshold-mm", type=float, default=80.0)
    args = parser.parse_args()

    if args.samples.endswith(".parquet"):
        samples = pd.read_parquet(args.samples)
    else:
        samples = pd.read_csv(args.samples)

    if "t_s" not in samples.columns:
        raise SystemExit("Missing t_s column. Run prepare_samples.py first.")

    baseline_map = _load_baselines(args.events_glob)

    windows = build_windows(
        samples,
        window_s=args.window_s,
        step_s=args.step_s,
        min_samples=args.min_samples,
        baseline_map=baseline_map,
        label_threshold_mm=args.label_threshold_mm,
    )

    os.makedirs(os.path.dirname(args.out), exist_ok=True)
    windows.to_csv(args.out, index=False)
    print(f"Wrote {len(windows)} windows to {args.out}")


if __name__ == "__main__":
    main()
