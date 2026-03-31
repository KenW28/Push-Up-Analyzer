import argparse
import json
import os
from typing import List

import pandas as pd


def _feature_columns(df: pd.DataFrame) -> List[str]:
    cols = [c for c in df.columns if any(c.endswith(suffix) for suffix in [
        "_mean", "_std", "_min", "_max", "_range"
    ])]
    return sorted(cols)


def main():
    parser = argparse.ArgumentParser(description="Train a model on windowed features.")
    parser.add_argument("--windows", default="data/data_processing/processed/windows.csv")
    parser.add_argument("--out-model", default="data/data_processing/processed/model.joblib")
    parser.add_argument("--out-features", default="data/data_processing/processed/model_features.json")
    args = parser.parse_args()

    df = pd.read_csv(args.windows)
    if "label_pushup" not in df.columns:
        raise SystemExit("Missing label_pushup column. Run build_windows.py first.")

    feature_cols = _feature_columns(df)
    if not feature_cols:
        raise SystemExit("No feature columns found.")

    df = df.dropna(subset=["label_pushup"] + feature_cols)

    try:
        from sklearn.pipeline import Pipeline
        from sklearn.preprocessing import StandardScaler
        from sklearn.linear_model import LogisticRegression
        import joblib
    except Exception as exc:
        raise SystemExit(
            "scikit-learn is required. Install with: pip install scikit-learn"
        ) from exc

    X = df[feature_cols].values
    y = df["label_pushup"].values

    clf = Pipeline([
        ("scaler", StandardScaler()),
        ("model", LogisticRegression(max_iter=500, class_weight="balanced")),
    ])
    clf.fit(X, y)

    os.makedirs(os.path.dirname(args.out_model), exist_ok=True)
    joblib.dump(clf, args.out_model)

    with open(args.out_features, "w", encoding="utf-8") as f:
        json.dump(feature_cols, f, indent=2)

    print(f"Saved model to {args.out_model}")
    print(f"Saved feature list to {args.out_features}")


if __name__ == "__main__":
    main()
