# bronto-cli

Terminal dashboard renderer for Bronto specs.

## Serve dashboard

```bash
bronto serve --spec /path/to/dashboard.json
```

## Live mode (auto-only)

```bash
bronto serve --spec /path/to/dashboard.json --refresh-ms 2000
```

- `--refresh-ms` enables automatic live reload of the spec file.
- No manual refresh key is required or expected.
- Each reload is strict-validated (`spec.LoadStrict`) before applying.
- On reload failure, the current dashboard remains visible and status shows the error.

## Recommended architecture

- Keep polling logic in your Go/agent runtime.
- That runtime updates the dashboard spec file as fresh Bronto data arrives.
- `bronto-cli` runs in live mode and continuously renders new snapshots.
