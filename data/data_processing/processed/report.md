# Windowed Data Report

## Overall Class Balance

|   label_pushup |   count |   percent |
|---------------:|--------:|----------:|
|              1 |     230 |   87.1212 |
|              0 |      34 |   12.8788 |

## Class Balance By Session

| session_id   |   label_pushup |   count |   percent |
|:-------------|---------------:|--------:|----------:|
| session1     |              0 |      21 |  18.5841  |
| session1     |              1 |      92 |  81.4159  |
| session2     |              0 |       2 |   4.54545 |
| session2     |              1 |      42 |  95.4545  |
| session3     |              0 |      11 |  13.75    |
| session3     |              1 |      69 |  86.25    |
| session4     |              1 |      27 | 100       |

## Example Pushup Windows (label=1)

| session_id   |   segment_id |   window_start_s |   window_end_s |   depth_mm |   label_pushup |   baseline_mm | baseline_source   |
|:-------------|-------------:|-----------------:|---------------:|-----------:|---------------:|--------------:|:------------------|
| session3     |            0 |              8.5 |           10.5 |     436.99 |              1 |        495.49 | baseline_locked   |
| session3     |            0 |              9   |           11   |     436.99 |              1 |        495.49 | baseline_locked   |
| session3     |            0 |              9.5 |           11.5 |     436.99 |              1 |        495.49 | baseline_locked   |

## Example Non-Pushup Windows (label=0)

| session_id   |   segment_id |   window_start_s |   window_end_s |   depth_mm |   label_pushup |   baseline_mm | baseline_source   |
|:-------------|-------------:|-----------------:|---------------:|-----------:|---------------:|--------------:|:------------------|
| session1     |            0 |              1.5 |            3.5 |   -4.33333 |              0 |           441 | p90               |
| session1     |            0 |              2   |            4   |   -2       |              0 |           441 | p90               |
| session1     |            0 |              2.5 |            4.5 |   -2       |              0 |           441 | p90               |

Notes:
- `depth_mm` is computed as `baseline_mm - min(tof_mm)` within each window.
- Labels are heuristic; validate with manual review before training a final model.