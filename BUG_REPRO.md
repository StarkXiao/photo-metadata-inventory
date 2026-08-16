# Reproduction

Baseline: `base_bug_003`.

Run:

```sh
go test ./... -run TestJPEGEXIFReadsAPP1Payload -count=20
```

Expected: an APP1 EXIF payload is extracted from a JPEG stream.

Actual: every run returns an empty payload.
