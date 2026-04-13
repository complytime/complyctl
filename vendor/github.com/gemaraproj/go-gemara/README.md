# go-gemara

[![Go Reference](https://pkg.go.dev/badge/github.com/gemaraproj/go-gemara.svg)](https://pkg.go.dev/github.com/gemaraproj/go-gemara)
[![License](https://img.shields.io/badge/license-Apache%202.0-blue.svg)](LICENSE)
[![Go Version](https://img.shields.io/badge/go%20version-1.23+-00ADD8.svg)](https://go.dev/)
[![CI](https://github.com/gemaraproj/go-gemara/actions/workflows/ci.yml/badge.svg)](https://github.com/gemaraproj/go-gemara/actions/workflows/ci.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/gemaraproj/go-gemara)](https://goreportcard.com/report/github.com/gemaraproj/go-gemara)

Go SDK for parsing and converting Gemara documents.

## Overview

This repository provides Go types and utilities for working with [Gemara](https://gemara.openssf.org/) documents.
The Go types are generated from CUE schemas published in the [Gemara CUE module](https://registry.cue.works/docs/github.com/gemaraproj/gemara@v0) (`github.com/gemaraproj/gemara@v0`) available in the [CUE Central Registry](https://registry.cue.works/).

## Installation

```bash
go get github.com/gemaraproj/go-gemara
```

## Usage

### CLI Tool

The `oscalexport` command-line tool converts Gemara documents to OSCAL format.

#### Building the CLI

```bash
make build
```

This builds binaries to `./bin/` directory.

#### Converting a Control Catalog

```bash
./bin/oscalexport catalog ./path/to/catalog.yaml --output ./catalog.json
```

#### Converting a Guidance Catalog

```bash
./bin/oscalexport guidance ./path/to/guidance.yaml \
    --catalog-output ./guidance.json \
    --profile-output ./profile.json
```

### Library Usage

#### Loading Gemara Documents

```go
package main

import (
    "github.com/gemaraproj/go-gemara"
    "github.com/gemaraproj/go-gemara/fetcher"
)

func main() {
    f := &fetcher.File{}

    // Load a Guidance Catalog
    guidance, err := gemara.Load[gemara.GuidanceCatalog](f, "path/to/guidance.yaml")
    if err != nil {
        panic(err)
    }

    // Load a Control Catalog
    catalog, err := gemara.Load[gemara.ControlCatalog](f, "path/to/catalog.yaml")
    if err != nil {
        panic(err)
    }

    _ = guidance
    _ = catalog
}
```

#### Converting to OSCAL

```go
package main

import (
    "github.com/gemaraproj/go-gemara"
    "github.com/gemaraproj/go-gemara/fetcher"
    "github.com/gemaraproj/go-gemara/gemaraconv"
)

func main() {
    f := &fetcher.File{}

    // Convert Control Catalog to OSCAL
    catalog, err := gemara.Load[gemara.ControlCatalog](f, "path/to/catalog.yaml")
    if err != nil {
        panic(err)
    }

    oscalCatalog, err := gemaraconv.ControlCatalog(catalog).ToOSCAL()
    if err != nil {
        panic(err)
    }

    // Convert Guidance Catalog to OSCAL
    guidance, err := gemara.Load[gemara.GuidanceCatalog](f, "path/to/guidance.yaml")
    if err != nil {
        panic(err)
    }

    _, oscalProfile, err := gemaraconv.GuidanceCatalog(guidance).ToOSCAL("relative/path/to/catalog.json")
    if err != nil {
        panic(err)
    }

    _ = oscalCatalog
    _ = oscalProfile
}
```

#### Converting to SARIF

```go
package main

import (
    "github.com/gemaraproj/go-gemara"
    "github.com/gemaraproj/go-gemara/fetcher"
    "github.com/gemaraproj/go-gemara/gemaraconv"
)

func main() {
    f := &fetcher.File{}

    // Load Control Catalog (required for SARIF conversion)
    catalog, err := gemara.Load[gemara.ControlCatalog](f, "path/to/catalog.yaml")
    if err != nil {
        panic(err)
    }

    // Convert EvaluationLog to SARIF
    evaluationLog := &gemara.EvaluationLog{
        // ... populate evaluation log ...
    }

    sarifBytes, err := gemaraconv.EvaluationLog(evaluationLog).ToSARIF("file:///path/to/artifact.md", catalog)
    if err != nil {
        panic(err)
    }

    _ = sarifBytes
}
```

## Development

### Building

```bash
make build
```

### Testing

```bash
# Run all tests
make test

# Run tests with coverage
make testcov

# Check coverage threshold
make coverage-check
```

### Linting

```bash
make lint
```

## License

Licensed under the Apache License, Version 2.0. See [LICENSE](LICENSE) for details.
