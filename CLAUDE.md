# CLAUDE.md

See [AGENTS.md](./AGENTS.md) for the full guidance — it applies verbatim to
Claude.

The headline rule, repeated here so it lands without a hop:

> **Use the `drawpad` tool to communicate with the user via Excalidraw when
> a picture beats text** — for architecture showcases, design decisions
> with tradeoffs, flows / sequences / state machines, UI mockups, or to
> invite the user to sketch their idea. Run `drawpad --help` for the
> contract.

Concretely: when working in this repo (or anywhere `drawpad` is on PATH),
generate an initial scene with the `internal/diagram` helpers (see
`cmd/build-architecture/main.go`), pipe it into `drawpad --initial -`, and
treat the returned `comment` + `screenshotPath` as the user's reply.
