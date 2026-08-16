# Photo Metadata Inventory

A dependency-free Go command-line tool for auditing JPEG, PNG, HEIC, and HEIF photo metadata.

## Run

```sh
go run . -path /path/to/photos -csv inventory.csv -safe
```

Options:

- `-path`: directory scanned recursively (default: current directory)
- `-csv`: optional output path for per-photo CSV results
- `-safe`: print privacy and archival recommendations

The report groups photos by capture month and camera, then flags missing capture times and embedded GPS data. JPEG APP1 EXIF and PNG `eXIf` metadata are read directly. HEIC/HEIF EXIF is detected from its embedded TIFF payload where present; unsupported or malformed metadata is recorded in the CSV instead of stopping the scan.

## Build

```sh
go build -o photo-inventory .
./photo-inventory -path /path/to/photos -safe
```
