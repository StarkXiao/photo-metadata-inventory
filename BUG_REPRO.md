# Reproduction

Baseline: `base_bug_004`.

Run:

```sh
go test ./... -run TestPNGEXIFReadsEXIFChunk -count=20
```

Expected: a PNG `eXIf` chunk returns its TIFF payload.

Actual: every run returns an empty payload.
