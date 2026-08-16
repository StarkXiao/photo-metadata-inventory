# Reproduction

Baseline: `base_bug_001`.

Run:

```sh
go test ./... -run TestParseTIFFFlagsLatitudeMetadata -count=20
```

Expected: a photo containing GPS latitude metadata is marked as having GPS data.

Actual: every run reports `gps=false`.
