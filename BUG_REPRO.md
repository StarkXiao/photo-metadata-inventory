# Reproduction

Baseline: `base_bug_002`.

Run:

```sh
go test ./... -run TestParseTIFF -count=20
```

Expected: the EXIF original capture timestamp is returned.

Actual: every run returns a zero capture time.
