# TUI Architecture

This package is intentionally split by responsibility so renderer behavior stays deterministic and easy to extend.

## File Map

- `model.go`:
  - Bubble Tea model state (`Model`)
  - entrypoints (`NewModel`, `Init`, `Update`, `View`)
- `runtime.go`:
  - component resolution (`resolveComponents`)
  - layout/resize propagation (`resizeForLayout`, `resizeNode`)
  - node rendering (`renderNode`, panel rendering)
- `focus.go`:
  - focus mode and panel navigation
  - filter-mode input handling and viewport scrolling
- `layout.go`:
  - row/column wrapping and split strategies
  - preferred-height calculations
- `types.go`:
  - shared UI constants and lightweight internal types
- `table.go`:
  - table column policies and width heuristics
  - truncation/formatting helpers and table styles
- `chart_meta.go`:
  - compact metadata summaries and legends
- `charts.go`:
  - chart renderers per family (`bar`, `line`, `scatter`, `waveline`, `streamline`, `sparkline`, `heatmap`, `timeseries`, `ohlc`)
- `helpers.go`:
  - shared utility helpers used across modules

## Design Rules

- The spec controls data and structure.
- The renderer enforces a default structure for generated dashboards:
  - charts and logs never mix in the same grid
  - charts render in 3-per-row grids
  - logs (tables) render in 2-per-row grids
  - when both exist, tabs switch between Charts and Logs
- The renderer controls look-and-feel and responsive layout.
- Unsupported or malformed nodes degrade gracefully in-panel.
- Rendering is deterministic for the same spec and terminal size.
