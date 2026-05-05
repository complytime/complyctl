---
description: Go testing patterns for the replicator project
tags: [testing, go, patterns]
---

# Testing Patterns

Go testing conventions for replicator.

## Framework

Standard library `testing` package only. No testify, gomega, or external
assertion libraries. Use `t.Errorf` / `t.Fatalf` directly.

## Test Naming

`TestXxx_Description` — e.g., `TestCreateCell_Defaults`, `TestReadyCell_PriorityOrder`.

## Isolation Patterns

### Database Tests

```go
store := db.OpenMemory()
defer store.Close()
```

Every test gets its own in-memory SQLite database. No shared state.

### Filesystem Tests

```go
dir := t.TempDir()
// dir is automatically cleaned up
```

### HTTP Tests

```go
srv := httptest.NewServer(handler)
defer srv.Close()
```

### Git Tests

```go
if testing.Short() {
    t.Skip("skipping git test in short mode")
}
dir := t.TempDir()
// exec.Command("git", "init", dir)
```

## Parity Tests

Build tag: `//go:build parity`

Compare Go response shapes against TypeScript fixtures in
`test/parity/fixtures/`. Run with:

```bash
go test -tags parity ./test/parity/ -count=1 -v
```

## Assertions

Use direct comparisons:

```go
if got != want {
    t.Errorf("FunctionName() = %v, want %v", got, want)
}
```

For slices and structs, use `reflect.DeepEqual` or compare field by field.
