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

## Native liveQuery polling

`bronto-cli` can poll Bronto directly when datasets include `liveQuery`.

Environment:

```bash
export BRONTO_API_KEY=...
export BRONTO_API_ENDPOINT=https://api.eu.bronto.io  # optional
```

Alternative (user-level config file):

`~/.bronto/config.json`

```json
{
  "api_key": "YOUR_KEY",
  "api_endpoint": "https://api.eu.bronto.io"
}
```

Optional override path:

```bash
export BRONTO_CONFIG_FILE=/path/to/config.json
```

Dataset example:

```json
{
  "kind": "categorySeries",
  "labels": ["seed"],
  "values": [0],
  "liveQuery": {
    "mode": "metrics",
    "logIds": ["fb7f985f-3558-0232-d30e-42142719a400"],
    "metricFunctions": ["COUNT(*)"],
    "groupByKeys": ["event.type"],
    "lookbackSec": 1800
  }
}
```
