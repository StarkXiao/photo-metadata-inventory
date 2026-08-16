# Reproduction

Baseline: `base_bug_005`.

Run:

```sh
go test ./... -run TestWriteCSVPreservesGPSFlag -count=20
```

Expected: the CSV GPS column matches the photo metadata value.

Actual: every run writes the opposite boolean value.
