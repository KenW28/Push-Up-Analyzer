import argparse
import os
from typing import List

import numpy as np
import pandas as pd


def _feature_columns(df: pd.DataFrame) -> List[str]:
    cols = [c for c in df.columns if any(c.endswith(suffix) for suffix in [
        "_mean", "_std", "_min", "_max", "_range"
    ])]
    return sorted(cols)


def _metrics(y_true, y_pred) -> dict:
    tp = int(((y_true == 1) & (y_pred == 1)).sum())
    tn = int(((y_true == 0) & (y_pred == 0)).sum())
    fp = int(((y_true == 0) & (y_pred == 1)).sum())
    fn = int(((y_true == 1) & (y_pred == 0)).sum())

    accuracy = (tp + tn) / max(tp + tn + fp + fn, 1)
    precision = tp / max(tp + fp, 1)
    recall = tp / max(tp + fn, 1)
    f1 = 2 * precision * recall / max(precision + recall, 1e-9)
    return {
        "accuracy": accuracy,
        "precision": precision,
        "recall": recall,
        "f1": f1,
        "tp": tp,
        "fp": fp,
        "tn": tn,
        "fn": fn,
    }


def main():
    parser = argparse.ArgumentParser(description="Train a baseline classifier with leave-one-session-out.")
    parser.add_argument("--windows", default="data/data_processing/processed/windows.csv")
    parser.add_argument("--out", default="data/data_processing/processed/baseline_report.md")
    args = parser.parse_args()

    df = pd.read_csv(args.windows)
    if "label_pushup" not in df.columns:
        raise SystemExit("Missing label_pushup column. Run build_windows.py first.")

    feature_cols = _feature_columns(df)
    if not feature_cols:
        raise SystemExit("No feature columns found.")

    # Drop rows with missing labels or features
    df = df.dropna(subset=["label_pushup"] + feature_cols)

    try:
        from sklearn.pipeline import Pipeline
        from sklearn.preprocessing import StandardScaler
        from sklearn.linear_model import LogisticRegression
    except Exception as exc:
        raise SystemExit(
            "scikit-learn is required. Install with: pip install scikit-learn"
        ) from exc

    sessions = sorted(df["session_id"].unique().tolist())
    if len(sessions) < 2:
        raise SystemExit("Need at least 2 sessions for leave-one-session-out validation.")

    lines = []
    lines.append("# Baseline Model Report")
    lines.append("")
    lines.append("Model: StandardScaler + LogisticRegression")
    lines.append("")

    all_metrics = []
    for test_session in sessions:
        train_df = df[df["session_id"] != test_session]
        test_df = df[df["session_id"] == test_session]

        X_train = train_df[feature_cols].values
        y_train = train_df["label_pushup"].values
        X_test = test_df[feature_cols].values
        y_test = test_df["label_pushup"].values

        if len(np.unique(y_train)) < 2 or len(np.unique(y_test)) < 2:
            # Skip if only one class in train or test
            metrics = {
                "session": test_session,
                "accuracy": np.nan,
                "precision": np.nan,
                "recall": np.nan,
                "f1": np.nan,
                "tp": 0,
                "fp": 0,
                "tn": 0,
                "fn": 0,
                "note": "Skipped (single class in train or test)",
            }
            all_metrics.append(metrics)
            continue

        clf = Pipeline([
            ("scaler", StandardScaler()),
            ("model", LogisticRegression(max_iter=200, class_weight="balanced")),
        ])
        clf.fit(X_train, y_train)
        y_pred = clf.predict(X_test)

        metrics = _metrics(y_test, y_pred)
        metrics["session"] = test_session
        metrics["note"] = ""
        all_metrics.append(metrics)

    report_df = pd.DataFrame(all_metrics)

    lines.append("## Per-Session Results (Leave-One-Session-Out)")
    lines.append("")
    lines.append(report_df[[
        "session",
        "accuracy",
        "precision",
        "recall",
        "f1",
        "tp",
        "fp",
        "tn",
        "fn",
        "note",
    ]].to_markdown(index=False))

    valid = report_df[report_df["note"] == ""]
    if not valid.empty:
        avg = valid[["accuracy", "precision", "recall", "f1"]].mean().to_dict()
        lines.append("")
        lines.append("## Average Metrics (valid sessions only)")
        lines.append("")
        lines.append("- accuracy: {:.3f}".format(avg["accuracy"]))
        lines.append("- precision: {:.3f}".format(avg["precision"]))
        lines.append("- recall: {:.3f}".format(avg["recall"]))
        lines.append("- f1: {:.3f}".format(avg["f1"]))

    lines.append("")
    lines.append("Notes:")
    lines.append("- This is a baseline. Labels are heuristic and may be noisy.")
    lines.append("- Use more sessions and refined labels for production.")

    os.makedirs(os.path.dirname(args.out), exist_ok=True)
    with open(args.out, "w", encoding="utf-8") as f:
        f.write("\n".join(lines))

    print(f"Wrote baseline report to {args.out}")


if __name__ == "__main__":
    main()
