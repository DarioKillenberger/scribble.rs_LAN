# Agentic Coding

Use this guide to make agent-driven work predictable, reviewable, and easy to validate.

## Operating Loop

1. Define the behavior change and completion criteria before editing.
2. Identify the owning package and the shared contracts involved.
3. Run or name the focused baseline test.
4. Make the smallest coherent change.
5. Run focused tests, then broader tests according to risk.
6. Update docs if behavior, config, commands, or contracts changed.

## Task Decomposition

Split work by ownership boundaries:

- API parsing or HTTP status behavior: `internal/api`.
- Game rules, scoring, turns, guesses, drawing state, players: `internal/game`.
- Lobby registry, stats, cleanup: `internal/state`.
- SSR page data, templates, embedded resources, browser behavior: `internal/frontend`.
- Localized text: `internal/translations`.
- Config defaults or environment parsing: `internal/config`.

Good agent-sized units have one dominant risk:

- Add validation for one lobby setting and test its parser.
- Add one WebSocket event and update both server and browser handling.
- Adjust one scoring rule and add focused tests around score calculation.
- Add one translation key across translation maps and run translation tests.

Avoid broad mixed units such as "refactor lobby handling" unless the first task is only to map current behavior and propose a sequence.

## Completion Criteria Templates

For API changes:

- Request parsing validates bad input.
- HTTP status and error text are intentional.
- Cookie/header compatibility is preserved.
- `internal/api` tests cover success and failure.

For game logic changes:

- State transitions are explicit.
- Disconnected, spectating, unstarted, ongoing, and game-over states are considered where relevant.
- Race-sensitive paths use lobby locking.
- `internal/game` tests cover the changed rule.

For WebSocket event changes:

- `internal/game/shared.go` contains the contract.
- `internal/game/lobby.go` handles or emits the event.
- Browser JS handles the new or changed payload.
- Event ordering and visibility rules are preserved.

For frontend changes:

- Templates still render through Go tests.
- Embedded files stay under the existing `go:embed` paths.
- No bundler or generated asset pipeline is introduced casually.
- Text changes use translations instead of hard-coded strings when user-visible.

## Review Checklist

- Does the change expose hidden lobby state?
- Can an unauthenticated or wrong-session caller affect a lobby?
- Does the change work with `ROOT_PATH`?
- Could WebSocket messages arrive in a different order?
- Is a lobby lock needed for the read or mutation?
- Does cleanup still remove empty lobbies without deleting active ones?
- Are disconnected players and reserved slots handled?
- Are public API and SSR paths kept consistent?
- Are translations, tests, and docs updated where relevant?

## Agent Handoff Format

When handing work to another agent or future session, include:

- Goal and non-goals.
- Files already inspected.
- Package owner for the next change.
- Tests run and exact result.
- Known risks or unverified assumptions.

Example:

```text
Goal: Add one new lobby setting for X.
Inspected: internal/api/createparse.go, internal/config/config.go, internal/game/shared.go.
Next owner: internal/api for parsing, internal/game for storage if client-visible.
Tests run: go test ./internal/api passed.
Risk: SSR form defaults not updated yet.
```

## Model Routing Guidance

Use a smaller/faster model for narrow edits like renames, translation additions, or parser test cases. Use a stronger implementation model for multi-file behavior changes. Use the strongest reasoning model for lobby lifecycle, WebSocket ordering, concurrency, or architecture review.

