# TUS CLI v1 (Legacy)

This directory contains the original implementation of the TUS CLI before the refactoring.

## ⚠️ Legacy Version

This is the **legacy version** kept for reference. For current usage, please use the main directory.

## Features (v1)

- Custom HTTP implementation
- Manual state management with `.tusc_state_*.json` files
- Complex file hashing strategies
- Concurrent upload detection
- Custom retry logic with exponential backoff
- Manual flag parsing

## Build v1

```bash
cd v1
go build -o tusc-v1 main.go
```

## Why We Moved Away

While v1 was functional, it had several issues:

- **Complex**: ~800 lines of code with intricate state management
- **Maintenance Heavy**: Custom HTTP client and retry logic
- **Error Prone**: Manual state file handling could lead to conflicts
- **Hard to Extend**: Tightly coupled components

## Migration to Current Version

The current version (in parent directory) addresses these issues by:

- Using official TUS Go client library
- Leveraging urfave/cli framework
- Automatic state management
- 70% code reduction
- Better error handling

## Historical Context

This v1 implementation was a learning exercise that helped understand:

- TUS protocol internals
- Resumable upload challenges
- State management complexity
- The value of using proven libraries

---

**For current usage, please use the main directory, not this v1 folder.**
