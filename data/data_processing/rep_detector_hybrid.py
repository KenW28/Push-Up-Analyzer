import argparse
import glob
import json
import os
from pathlib import Path
from typing import List

import numpy as np
import pandas as pd


def _feature_columns(df: pd.DataFrame) -> List[str]:
    cols = [c for c in df.columns if any(c.endswith(suffix) for suffix in [
        "_mean", "_std", "_min", "_max", "_range"
    ])]
    return sorted(cols)


def _load_features(path: str, df: pd.DataFrame) -> List[str]:
    if os.path.exists(path):
        with open(path, "r", encoding="utf-8") as f:
            return json.load(f)
    return _feature_columns(df)


def _load_baselines(events_glob: str) -> dict:
    baseline_map = {}
    if not events_glob:
        return baseline_map

    for path in sorted(glob.glob(events_glob)):
        session_id = Path(path).stem.split(".")[0]
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


def _estimate_hz(t_s: np.ndarray) -> float:
    if len(t_s) < 2:
        return 10.0
    dt = np.diff(t_s)
    dt = dt[dt > 0]
    if len(dt) == 0:
        return 10.0
    median_dt = float(np.median(dt))
    if median_dt <= 0:
        return 10.0
    return 1.0 / median_dt


def _smooth_signal(values: np.ndarray, window: int) -> np.ndarray:
    if window <= 1:
        return values
    series = pd.Series(values)
    return series.rolling(window=window, center=True, min_periods=1).mean().to_numpy()


def _find_local_minima(t_s: np.ndarray, tof: np.ndarray, active_mask: np.ndarray,
                       baseline_mm: float, min_depth_mm: float, min_interval_s: float,
                       smooth_window_s: float) -> List[dict]:
    hz = _estimate_hz(t_s)
    window = max(3, int(round(smooth_window_s * hz)))
    if window % 2 == 0:
        window += 1

    tof_smooth = _smooth_signal(tof, window)

    minima = []
    last_rep_time = -1e9

    for i in range(1, len(tof_smooth) - 1):
        if not active_mask[i]:
            continue
        left = tof_smooth[i - 1]
        right = tof_smooth[i + 1]
        center = tof_smooth[i]
        if center > left or center > right:
            continue
        if center == left and center == right:
            continue
        depth = baseline_mm - tof_smooth[i]
        if depth < min_depth_mm:
            continue
        t = t_s[i]
        if t - last_rep_time < min_interval_s:
            continue
        minima.append({
            "t_s": float(t),
            "tof_mm": float(tof_smooth[i]),
            "depth_mm": float(depth),
        })
        last_rep_time = t

    return minima


def main():
    parser = argparse.ArgumentParser(description="Hybrid rep detector (classifier + peak detection).")
    parser.add_argument("--samples", default="data/data_processing/processed/samples.csv")
    parser.add_argument("--windows", default="data/data_processing/processed/windows.csv")
    parser.add_argument("--model", default="data/data_processing/processed/model.joblib")
    parser.add_argument("--features", default="data/data_processing/processed/model_features.json")
    parser.add_argument("--out-events", default="data/data_processing/processed/rep_events_hybrid.csv")
    parser.add_argument("--out-report", default="data/data_processing/processed/rep_report_hybrid.md")
    parser.add_argument("--threshold", type=float, default=0.5)
    parser.add_argument("--min-depth-mm", type=float, default=80.0)
    parser.add_argument("--min-interval-s", type=float, default=0.6)
    parser.add_argument("--smooth-window-s", type=float, default=0.3)
    args = parser.parse_args()

    samples = pd.read_csv(args.samples)
    windows = pd.read_csv(args.windows)

    try:
        import joblib
    except Exception as exc:
        raise SystemExit(
            "joblib is required (usually installed with scikit-learn)."
        ) from exc

    feature_cols = _load_features(args.features, windows)
    missing = [c for c in feature_cols if c not in windows.columns]
    if missing:
        raise SystemExit(f"Missing feature columns: {missing}")

    model = joblib.load(args.model)
    X = windows[feature_cols].values
    if hasattr(model, "predict_proba"):
        probs = model.predict_proba(X)[:, 1]
    else:
        probs = model.predict(X)
    windows = windows.copy()
    windows["pred_prob"] = probs
    windows["pred_label"] = (windows["pred_prob"] >= args.threshold).astype(int)

    events_rows = []
    report_lines = ["# Hybrid Rep Detection Report", ""]

    for (session_id, segment_id), w_group in windows.groupby(["session_id", "segment_id"]):
        w_group = w_group.sort_values("window_start_s")
        intervals = w_group[w_group["pred_label"] == 1][["window_start_s", "window_end_s"]].values

        s_group = samples[(samples["session_id"] == session_id) & (samples["segment_id"] == segment_id)]
        if s_group.empty:
            report_lines.append(f"## {session_id} / segment {segment_id}")
            report_lines.append("- predicted reps: 0")
            report_lines.append("")
            continue

        s_group = s_group.sort_values("t_s")
        t_s = s_group["t_s"].to_numpy()
        tof = s_group["tof_mm"].to_numpy()

        if len(intervals) == 0:
            report_lines.append(f"## {session_id} / segment {segment_id}")
            report_lines.append("- predicted reps: 0")
            report_lines.append("")
            continue

        active_mask = np.zeros_like(t_s, dtype=bool)
        for start, end in intervals:
            active_mask |= (t_s >= float(start)) & (t_s < float(end))

        baseline_mm = float(s_group["tof_mm"].quantile(0.9))
        minima = _find_local_minima(
            t_s,
            tof,
            active_mask,
            baseline_mm=baseline_mm,
            min_depth_mm=args.min_depth_mm,
            min_interval_s=args.min_interval_s,
            smooth_window_s=args.smooth_window_s,
        )

        for idx, rep in enumerate(minima):
            events_rows.append({
                "session_id": session_id,
                "segment_id": int(segment_id),
                "rep_index": idx,
                "t_s": rep["t_s"],
                "tof_mm": rep["tof_mm"],
                "depth_mm": rep["depth_mm"],
                "baseline_mm": baseline_mm,
            })

        report_lines.append(f"## {session_id} / segment {segment_id}")
        report_lines.append(f"- predicted reps: {len(minima)}")
        report_lines.append("")

    events_df = pd.DataFrame(events_rows)
    os.makedirs(os.path.dirname(args.out_events), exist_ok=True)
    events_df.to_csv(args.out_events, index=False)

    with open(args.out_report, "w", encoding="utf-8") as f:
        f.write("\n".join(report_lines))

    print(f"Wrote rep events to {args.out_events}")
    print(f"Wrote report to {args.out_report}")


if __name__ == "__main__":
    main()
