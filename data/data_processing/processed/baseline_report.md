# Baseline Model Report

Model: StandardScaler + LogisticRegression

## Per-Session Results (Leave-One-Session-Out)

| session   |   accuracy |   precision |     recall |         f1 |   tp |   fp |   tn |   fn | note                                    |
|:----------|-----------:|------------:|-----------:|-----------:|-----:|-----:|-----:|-----:|:----------------------------------------|
| session1  |   0.884956 |           1 |   0.858696 |   0.923977 |   79 |    0 |   21 |   13 |                                         |
| session2  |   1        |           1 |   1        |   1        |   42 |    0 |    2 |    0 |                                         |
| session3  |   0.95     |           1 |   0.942029 |   0.970149 |   65 |    0 |   11 |    4 |                                         |
| session4  | nan        |         nan | nan        | nan        |    0 |    0 |    0 |    0 | Skipped (single class in train or test) |

## Average Metrics (valid sessions only)

- accuracy: 0.945
- precision: 1.000
- recall: 0.934
- f1: 0.965

Notes:
- This is a baseline. Labels are heuristic and may be noisy.
- Use more sessions and refined labels for production.