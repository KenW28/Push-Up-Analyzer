import argparse
import json
import os
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


def _build_rep_events(rows: pd.DataFrame, merge_gap_s: float, min_run_s: float):
    events = []
    rep_index = 0
    current = None

    for _, row in rows.iterrows():
        if row["pred_label"] != 1:
            continue
        start = float(row["window_start_s"])
        end = float(row["window_end_s"])
        depth = float(row.get("depth_mm", 0.0))

        if current is None:
            current = {
                "start": start,
                "end": end,
                "peak_depth": depth,
                "count": 1,
            }
            continue

        if start <= current["end"] + merge_gap_s:
            current["end"] = max(current["end"], end)
            current["peak_depth"] = max(current["peak_depth"], depth)
            current["count"] += 1
        else:
            duration = current["end"] - current["start"]
            if duration >= min_run_s:
                events.append({
                    "rep_index": rep_index,
                    "start_s": current["start"],
                    "end_s": current["end"],
                    "duration_s": duration,
                    "peak_depth_mm": current["peak_depth"],
                    "num_windows": current["count"],
                })
                rep_index += 1
            current = {
                "start": start,
                "end": end,
                "peak_depth": depth,
                "count": 1,
            }

    if current is not None:
        duration = current["end"] - current["start"]
        if duration >= min_run_s:
            events.append({
                "rep_index": rep_index,
                "start_s": current["start"],
                "end_s": current["end"],
                "duration_s": duration,
                "peak_depth_mm": current["peak_depth"],
                "num_windows": current["count"],
            })

    return events


def main():
    parser = argparse.ArgumentParser(description="Predict pushup reps from windows using a trained model.")
    parser.add_argument("--windows", default="data/data_processing/processed/windows.csv")
    parser.add_argument("--model", default="data/data_processing/processed/model.joblib")
    parser.add_argument("--features", default="data/data_processing/processed/model_features.json")
    parser.add_argument("--out-events", default="data/data_processing/processed/rep_events.csv")
    parser.add_argument("--out-report", default="data/data_processing/processed/rep_report.md")
    parser.add_argument("--threshold", type=float, default=0.5)
    parser.add_argument("--merge-gap-s", type=float, default=0.6)
    parser.add_argument("--min-run-s", type=float, default=0.4)
    args = parser.parse_args()

    df = pd.read_csv(args.windows)

    try:
        import joblib
    except Exception as exc:
        raise SystemExit(
            "joblib is required (usually installed with scikit-learn)."
        ) from exc

    feature_cols = _load_features(args.features, df)
    missing = [c for c in feature_cols if c not in df.columns]
    if missing:
        raise SystemExit(f"Missing feature columns: {missing}")

    model = joblib.load(args.model)
    X = df[feature_cols].values

    if hasattr(model, "predict_proba"):
        probs = model.predict_proba(X)[:, 1]
    else:
        probs = model.predict(X)
    df["pred_prob"] = probs
    df["pred_label"] = (df["pred_prob"] >= args.threshold).astype(int)

    events_rows = []
    report_lines = []
    report_lines.append("# Rep Detection Report")
    report_lines.append("")

    for (session_id, segment_id), rows in df.groupby(["session_id", "segment_id"]):
        rows = rows.sort_values("window_start_s")
        events = _build_rep_events(rows, args.merge_gap_s, args.min_run_s)
        for ev in events:
            events_rows.append({
                "session_id": session_id,
                "segment_id": int(segment_id),
                **ev,
            })
        report_lines.append(f"## {session_id} / segment {segment_id}")
        report_lines.append("")
        report_lines.append(f"- predicted reps: {len(events)}")
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
