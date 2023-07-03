# slogdriver

[![GoDoc](https://godoc.org/github.com/jussi-kalliokoski/slogdriver?status.svg)](https://godoc.org/github.com/jussi-kalliokoski/slogdriver)
[![CI status](https://github.com/jussi-kalliokoski/slogdriver/workflows/CI/badge.svg)](https://github.com/jussi-kalliokoski/slogdriver/actions)

Stackdriver Logging / GCP Cloud Logging handler for go the [slog](https://pkg.go.dev/log/slog) package for structured logging introduced in the standard library of go 1.21.

NOTE: slogdriver requires go 1.21.

## Design Goals

- Improved performance compared to using the `JSONHandler` with `ReplaceAttr` to achieve the same purpose. This is achieved by using [goldjson](https://github.com/jussi-kalliokoski/goldjson) under the hood.
- Batteries included, e.g. builtin support for labels and traces. The trace information still needs to be provided separately as the library is agnostic as to which telemetry libraries (or versions) you choose to use. It is still highly advised to use [OpenTelemetry](https://opentelemetry.io/docs/instrumentation/go/).
- Minimal dependencies.
