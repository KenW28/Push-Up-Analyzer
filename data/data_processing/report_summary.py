import argparse
import os
import pandas as pd

KEY_COLUMNS = [
    "session_id",
    "segment_id",
    "window_start_s",
    "window_end_s",
    "depth_mm",
    "label_pushup",
    "baseline_mm",
    "baseline_source",
]


def _class_balance(df: pd.DataFrame) -> pd.DataFrame:
    counts = df["label_pushup"].value_counts(dropna=False).rename("count")
    pct = (counts / counts.sum() * 100).rename("percent")
    out = pd.concat([counts, pct], axis=1).reset_index().rename(columns={"index": "label"})
    return out


def _class_balance_by_session(df: pd.DataFrame) -> pd.DataFrame:
    counts = df.groupby(["session_id", "label_pushup"]).size().rename("count").reset_index()
    totals = counts.groupby("session_id")["count"].transform("sum")
    counts["percent"] = counts["count"] / totals * 100
    return counts


def _example_rows(df: pd.DataFrame, label: int, n: int = 3) -> pd.DataFrame:
    subset = df[df["label_pushup"] == label].copy()
    if subset.empty:
        return subset
    subset = subset.sort_values("depth_mm", ascending=(label == 0))
    return subset.head(n)[KEY_COLUMNS]


def main():
    parser = argparse.ArgumentParser(description="Summarize windowed dataset for modeling.")
    parser.add_argument("--windows", default="data/data_processing/processed/windows.csv")
    parser.add_argument("--out", default="data/data_processing/processed/report.md")
    args = parser.parse_args()

    df = pd.read_csv(args.windows)

    overall = _class_balance(df)
    by_session = _class_balance_by_session(df)

    example_pos = _example_rows(df, label=1, n=3)
    example_neg = _example_rows(df, label=0, n=3)

    lines = []
    lines.append("# Windowed Data Report")
    lines.append("")
    lines.append("## Overall Class Balance")
    lines.append("")
    lines.append(overall.to_markdown(index=False))
    lines.append("")
    lines.append("## Class Balance By Session")
    lines.append("")
    lines.append(by_session.to_markdown(index=False))
    lines.append("")
    lines.append("## Example Pushup Windows (label=1)")
    lines.append("")
    if example_pos.empty:
        lines.append("No positive windows found.")
    else:
        lines.append(example_pos.to_markdown(index=False))
    lines.append("")
    lines.append("## Example Non-Pushup Windows (label=0)")
    lines.append("")
    if example_neg.empty:
        lines.append("No negative windows found.")
    else:
        lines.append(example_neg.to_markdown(index=False))
    lines.append("")
    lines.append("Notes:")
    lines.append("- `depth_mm` is computed as `baseline_mm - min(tof_mm)` within each window.")
    lines.append("- Labels are heuristic; validate with manual review before training a final model.")

    os.makedirs(os.path.dirname(args.out), exist_ok=True)
    with open(args.out, "w", encoding="utf-8") as f:
        f.write("\n".join(lines))

    print(f"Wrote report to {args.out}")


if __name__ == "__main__":
    main()
