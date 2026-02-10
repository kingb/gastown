# Recent Git Activity Summary

**Period:** Jan 10–19, 2026 | **265 commits** | **~20 contributors** | **Releases:** v0.3.0, v0.3.1, v0.4.0

## Key Changes by Area

### Mail & Channels (major feature push)
- Full channel system: subscribe/unsubscribe, fan-out delivery, message retention enforcement
- Queue management commands, `gt mail claim`, address resolver integration
- Group management, `ack` alias for mark-read

### Patrol System
- Daily patrol digest aggregation with idempotency
- Backoff test formula, await-signal fixes
- Deacon patrol: stranded convoy step added
- Unit test coverage for patrol.go

### Polecat / Worktree Management
- Orphan `.beads/` cleanup on `gt done`
- Stale worktree pruning on early return in `RemoveWithOptions`
- Full nuke cleanup for worktrees and branches

### Orphan / Session Management
- Automatic orphaned Claude process cleanup
- Prevent killing Claude processes in valid tmux sessions
- Session leak prevention on macOS

### CLI / UX
- New commands: `gt bead show`, `gt show`, `gt cat`, `gt ready` filtering
- Desire-path commands for agent ergonomics
- Formula label support in `gt sling`
- Claude Code prompt prefix update for v2.1+

### Infrastructure
- Namepool: auto-select theme per rig, persist custom names properly
- Config: Gas Town custom types support
- Daemon: prevent runaway refinery session spawning
- Cost tracking disabled pending Claude Code cost data exposure

## Activity Patterns
- **133 fixes vs 58 features** — heavy stabilization/hardening phase
- Busiest days: Jan 12 (52 commits), Jan 13 (46), Jan 17 (45)
- Most active directories: `internal/cmd` (259 file changes), `internal/beads` (57), `internal/polecat` (28)
- Top contributors: mayor (31), max (21), Julian Knutsen (18)

## Notable Pattern
The codebase went through a rapid feature-then-stabilize cycle: mail/channels landed as a major new subsystem, immediately followed by a wave of fixes across beads, polecat, and daemon subsystems. Three releases in one week (v0.3.0 → v0.3.1 → v0.4.0) signals fast iteration.
