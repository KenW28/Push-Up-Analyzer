# Data Processing

Goal: turn raw session CSVs into clean, uniform samples and windowed features for a pushup classifier. [G1]

## Reference Map

- [G1] Overall objective of the pipeline
- [P1] Sample preparation (cleaning + resampling)
- [P2] Feature enrichment (magnitude + diffs)
- [P3] Windowing + heuristic labels
- [R1] Data quality report
- [M1] Baseline model training
- [M2] Final model training (all data)
- [R2] Rep detection from model predictions
- [R3] Hybrid rep detection (classifier + peak detection)

## Pipeline

1. Prepare samples (clean + resample + feature deltas). [P1]
2. Add features (magnitude + deltas) for modeling. [P2]
3. Build windowed features with heuristic labels. [P3]

## Usage

### Step 1: Prepare Samples [P1]

```bash
python data/data_processing/prepare_samples.py \
  --input-glob "data/session*.csv" \
  --events-glob "data/session*.events.csv" \
  --out-dir "data/data_processing/processed" \
  --sample-hz 10 \
  --max-gap-s 1.0
```

Outputs:
- `data/data_processing/processed/samples.csv`
- `data/data_processing/processed/samples.parquet` (if parquet support is installed)

### Step 2: Build Windows + Labels [P3]

```bash
python data/data_processing/build_windows.py \
  --samples "data/data_processing/processed/samples.csv" \
  --events-glob "data/session*.events.csv" \
  --out "data/data_processing/processed/windows.csv" \
  --window-s 2.0 \
  --step-s 0.5 \
  --label-threshold-mm 80
```

Outputs:
- `data/data_processing/processed/windows.csv`

### Step 3: Data Quality Report [R1]

```bash
python data/data_processing/report_summary.py \
  --windows "data/data_processing/processed/windows.csv" \
  --out "data/data_processing/processed/report.md"
```

Outputs:
- `data/data_processing/processed/report.md`

### Step 4: Baseline Model (Leave-One-Session-Out) [M1]

```bash
python data/data_processing/train_baseline.py \
  --windows "data/data_processing/processed/windows.csv" \
  --out "data/data_processing/processed/baseline_report.md"
```

Outputs:
- `data/data_processing/processed/baseline_report.md`

### Step 5: Train Final Model (All Data) [M2]

```bash
python data/data_processing/train_model.py \
  --windows "data/data_processing/processed/windows.csv" \
  --out-model "data/data_processing/processed/model.joblib" \
  --out-features "data/data_processing/processed/model_features.json"
```

Outputs:
- `data/data_processing/processed/model.joblib`
- `data/data_processing/processed/model_features.json`

### Step 6: Predict Reps From Windows [R2]

```bash
python data/data_processing/predict_reps.py \
  --windows "data/data_processing/processed/windows.csv" \
  --model "data/data_processing/processed/model.joblib" \
  --features "data/data_processing/processed/model_features.json" \
  --out-events "data/data_processing/processed/rep_events.csv" \
  --out-report "data/data_processing/processed/rep_report.md" \
  --threshold 0.5 \
  --merge-gap-s 0.6 \
  --min-run-s 0.4
```

Outputs:
- `data/data_processing/processed/rep_events.csv`
- `data/data_processing/processed/rep_report.md`

### Step 7: Hybrid Rep Detection (Classifier + Peaks) [R3]

```bash
python data/data_processing/rep_detector_hybrid.py \
  --samples "data/data_processing/processed/samples.csv" \
  --windows "data/data_processing/processed/windows.csv" \
  --model "data/data_processing/processed/model.joblib" \
  --features "data/data_processing/processed/model_features.json" \
  --out-events "data/data_processing/processed/rep_events_hybrid.csv" \
  --out-report "data/data_processing/processed/rep_report_hybrid.md" \
  --threshold 0.5 \
  --min-depth-mm 80 \
  --min-interval-s 0.6 \
  --smooth-window-s 0.3
```

Outputs:
- `data/data_processing/processed/rep_events_hybrid.csv`
- `data/data_processing/processed/rep_report_hybrid.md`

## Notes

- Baseline values are read from `*.events.csv` when present. If missing, the script uses the 90th percentile of `tof_mm` as a fallback baseline. [P1]
- `label-threshold-mm` is a heuristic. Tune it so that true pushups are labeled as `1` and non-pushup windows are `0`. [P3]
- Session gaps greater than `--max-gap-s` are split into segments to avoid interpolating across large pauses. [P1]
- Baseline model metrics are for sanity-checking only; do not treat them as final performance. [M1]
- `predict_reps.py` groups consecutive positive windows into rep events. Adjust `--merge-gap-s` and `--min-run-s` to tune rep counting. [R2]
- `rep_detector_hybrid.py` detects local minima in `tof_mm` within predicted pushup regions. Tune `--min-depth-mm` and `--min-interval-s` to match your form and pace. [R3]
