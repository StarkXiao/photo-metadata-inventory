# Evaluation Packaging Notes

This is a pure Go command-line application for recursively auditing image metadata.

## Standard Commands

```bash
go mod download
go build ./...
go test ./...
go run . -path /path/to/photos -csv inventory.csv -safe
```

## Container Build

The Docker image retains the Go toolchain and downloads module dependencies during its build.

```bash
chmod +x build_benzhi_docker.sh
./build_benzhi_docker.sh photo-metadata-inventory linux/amd64
./build_benzhi_docker.sh photo-metadata-inventory linux/arm64
docker run -it photo-metadata-inventory:latest
```

Inside the container, run `go build ./...` and `go test ./...` to verify the project.
