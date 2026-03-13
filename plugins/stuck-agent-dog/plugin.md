+++
name = "stuck-agent-dog"
description = "Context-aware stuck/crashed agent detection and restart for polecats and deacons"
version = 1

[gate]
type = "cooldown"
duration = "5m"

[tracking]
labels = ["plugin:stuck-agent-dog", "category:health"]
digest = true

[execution]
timeout = "5m"
notify_on_failure = true
severity = "high"
+++

# Stuck Agent Dog

Detects stuck or crashed polecats and deacons by inspecting tmux session context
before taking action. Unlike the daemon's blind kill-and-restart approach, this
plugin checks whether an agent is truly unresponsive before restarting.

**Design principle**: The daemon should NEVER kill workers. It detects and logs.
This plugin (running as a Dog agent with AI judgment) makes the restart decision
after inspecting tmux pane output for signs of life.

Reference: WAR-ROOM-SERIAL-KILLER.md, commit f3d47a96.

## Step 1: Enumerate agents to check

Gather all polecats from `gt session list`. We check both crashed sessions
(session dead, work on hook) and stuck sessions (session alive but agent hung).
Skip witness/refinery — they have their own health monitoring.

```bash
echo "=== Stuck Agent Dog: Checking agent health ==="

TOWN_ROOT="$HOME/gt"

# Get all sessions from gt session list (authoritative source)
SESSION_JSON=$(gt session list --json 2>/dev/null)
if [ $? -ne 0 ] || [ -z "$SESSION_JSON" ]; then
  echo "SKIP: could not get session list"
  exit 0
fi

# Parse into tab-delimited entries: session_id \t rig \t polecat
# Filter to crew/polecat sessions only (skip witness, refinery)
POLECAT_ENTRIES=$(echo "$SESSION_JSON" | jq -r '
  .[] | select(.polecat | test("^(crew-|polecat-)")) |
  [.session_id, .rig, .polecat] | @tsv
' 2>/dev/null)
```

## Step 2: Check polecat health

For each polecat session from `gt session list`, check its tmux session status.
A polecat is a concern if:
- It has hooked work (hook_bead is set)
- Its tmux session is dead OR the agent process is dead

```bash
CRASHED=()
STUCK=()
HEALTHY=0

while IFS=$'\t' read -r SESSION_ID RIG PCAT_NAME; do
  [ -z "$SESSION_ID" ] && continue

  # Resolve the polecat's beads path (crew-* or legacy polecats/*)
  case "$PCAT_NAME" in
    crew-*) PCAT_BEADS_PATH="$RIG/$PCAT_NAME" ;;
    *)      PCAT_BEADS_PATH="$RIG/polecats/$PCAT_NAME" ;;
  esac

  # Check if session exists
  if ! tmux has-session -t "$SESSION_ID" 2>/dev/null; then
    # Session dead — check if it has hooked work
    HOOK_BEAD=$(bd show "$PCAT_BEADS_PATH" --json 2>/dev/null \
      | jq -r '.hook_bead // empty' 2>/dev/null)

    if [ -n "$HOOK_BEAD" ]; then
      # Check agent_state to avoid interfering with active spawning
      AGENT_STATE=$(bd show "$PCAT_BEADS_PATH" --json 2>/dev/null \
        | jq -r '.agent_state // empty' 2>/dev/null)
      if [ "$AGENT_STATE" = "spawning" ]; then
        echo "  SKIP $SESSION_ID: agent_state=spawning (sling in progress)"
        continue
      fi
      CRASHED+=("$SESSION_ID|$RIG|$PCAT_NAME|$HOOK_BEAD")
      echo "  CRASHED: $SESSION_ID (hook=$HOOK_BEAD)"
    fi
  else
    # Session alive — check for agent process liveness
    # Capture last 5 lines of pane output to check for signs of life
    PANE_OUTPUT=$(tmux capture-pane -t "$SESSION_ID" -p -S -5 2>/dev/null || echo "")

    # Check if agent process is running in the session
    PANE_PID=$(tmux list-panes -t "$SESSION_ID" -F '#{pane_pid}' 2>/dev/null | head -1)
    if [ -n "$PANE_PID" ]; then
      # Check if Claude or another agent process is a descendant
      AGENT_ALIVE=$(pgrep -P "$PANE_PID" -f 'claude|node|anthropic' 2>/dev/null | head -1)
      if [ -z "$AGENT_ALIVE" ]; then
        # Agent process dead but session alive — zombie session
        HOOK_BEAD=$(bd show "$PCAT_BEADS_PATH" --json 2>/dev/null \
          | jq -r '.hook_bead // empty' 2>/dev/null)
        if [ -n "$HOOK_BEAD" ]; then
          STUCK+=("$SESSION_ID|$RIG|$PCAT_NAME|$HOOK_BEAD|agent_dead")
          echo "  ZOMBIE: $SESSION_ID (agent dead, session alive, hook=$HOOK_BEAD)"
        fi
      else
        HEALTHY=$((HEALTHY + 1))
      fi
    else
      HEALTHY=$((HEALTHY + 1))
    fi
  fi
done <<< "$POLECAT_ENTRIES"

echo ""
echo "Health summary: ${#CRASHED[@]} crashed, ${#STUCK[@]} stuck, $HEALTHY healthy"
```

## Step 3: Check deacon health

The deacon session is `hq-deacon`. Check heartbeat staleness.

```bash
echo ""
echo "=== Deacon Health ==="

DEACON_SESSION="hq-deacon"
DEACON_ISSUE=""

if ! tmux has-session -t "$DEACON_SESSION" 2>/dev/null; then
  echo "  CRASHED: Deacon session is dead"
  DEACON_ISSUE="crashed"
else
  # Check deacon heartbeat file
  HEARTBEAT_FILE="$TOWN_ROOT/deacon/.deacon-heartbeat"
  if [ -f "$HEARTBEAT_FILE" ]; then
    HEARTBEAT_TIME=$(stat -f %m "$HEARTBEAT_FILE" 2>/dev/null || stat -c %Y "$HEARTBEAT_FILE" 2>/dev/null)
    NOW=$(date +%s)
    HEARTBEAT_AGE=$(( NOW - HEARTBEAT_TIME ))

    if [ "$HEARTBEAT_AGE" -gt 600 ]; then
      echo "  STUCK: Deacon heartbeat stale (${HEARTBEAT_AGE}s old, >10m threshold)"
      DEACON_ISSUE="stuck_heartbeat_${HEARTBEAT_AGE}s"
    else
      echo "  OK: Deacon heartbeat ${HEARTBEAT_AGE}s old"
    fi
  else
    echo "  WARN: No heartbeat file found"
  fi
fi
```

## Step 4: Inspect context before acting (AI judgment)

**This is the key difference from daemon blind-kill.** For each crashed or stuck
agent, inspect the tmux pane context to determine if restart is appropriate.

**You (the dog agent) must evaluate each case:**

For CRASHED agents (session dead, work on hook):
- This is almost always a legitimate crash needing restart
- Exception: if the polecat just ran `gt done` and the hook hasn't cleared yet
- Check bead status: if the root wisp is closed, the polecat completed normally

For STUCK agents (session alive, agent dead):
- Kill the zombie session, then restart
- Exception: if pane output shows the agent is in a long-running build/test

For DEACON stuck (stale heartbeat):
- Capture pane output: `tmux capture-pane -t hq-deacon -p -S -20`
- If output shows active work (recent timestamps, command output), the heartbeat
  file may just be stale — nudge instead of kill
- If output shows no recent activity, restart is warranted

**Decision framework:**
1. If agent is clearly dead (no process, no output) → restart
2. If agent shows recent activity in pane → nudge first, check again next cycle
3. If agent has been stuck for >15 minutes with no pane activity → restart
4. If mass death detected (>3 crashes in same cycle) → escalate, don't restart

## Step 5: Take action

For each agent requiring restart:

```bash
# For crashed polecats — notify witness to handle restart
for ENTRY in "${CRASHED[@]}"; do
  IFS='|' read -r SESSION_ID RIG PCAT HOOK <<< "$ENTRY"

  echo "Requesting restart for $RIG/$PCAT (session=$SESSION_ID, hook=$HOOK)"

  gt mail send "$RIG/witness" \
    -s "RESTART_POLECAT: $RIG/$PCAT" \
    --stdin <<BODY
Polecat $PCAT crash confirmed by stuck-agent-dog plugin.
Context-aware inspection completed — agent is genuinely dead.

session_id: $SESSION_ID
hook_bead: $HOOK
action: restart requested

Please restart this polecat session.
BODY

done

# For zombie polecats — kill zombie session first, then request restart
for ENTRY in "${STUCK[@]}"; do
  IFS='|' read -r SESSION_ID RIG PCAT HOOK REASON <<< "$ENTRY"

  echo "Killing zombie session $SESSION_ID and requesting restart"
  tmux kill-session -t "$SESSION_ID" 2>/dev/null || true

  gt mail send "$RIG/witness" \
    -s "RESTART_POLECAT: $RIG/$PCAT (zombie cleared)" \
    --stdin <<BODY
Polecat $PCAT zombie session cleared by stuck-agent-dog plugin.
Session was alive but agent process was dead.

session_id: $SESSION_ID
hook_bead: $HOOK
reason: $REASON
action: restart requested

Please restart this polecat session.
BODY

done

# For deacon issues
if [ -n "$DEACON_ISSUE" ]; then
  echo "Escalating deacon issue: $DEACON_ISSUE"
  gt escalate "Deacon $DEACON_ISSUE detected by stuck-agent-dog" \
    -s HIGH \
    --reason "Deacon issue: $DEACON_ISSUE. Context inspection completed."
fi
```

## Step 6: Mass death check

If multiple agents crashed in the same cycle, this may indicate a systemic
issue (Dolt outage, OOM, etc.). Escalate instead of blindly restarting all.

```bash
TOTAL_ISSUES=$(( ${#CRASHED[@]} + ${#STUCK[@]} ))
if [ "$TOTAL_ISSUES" -ge 3 ]; then
  echo "MASS DEATH: $TOTAL_ISSUES agents down in same cycle — escalating"
  gt escalate "Mass agent death: $TOTAL_ISSUES agents down" \
    -s CRITICAL \
    --reason "stuck-agent-dog detected $TOTAL_ISSUES agents down simultaneously.
Crashed: ${CRASHED[*]}
Stuck: ${STUCK[*]}
This may indicate a systemic issue (Dolt, OOM, infra). Investigate before mass restart."
fi
```

## Record Result

```bash
SUMMARY="Agent health check: ${#CRASHED[@]} crashed, ${#STUCK[@]} stuck, $HEALTHY healthy"
if [ -n "$DEACON_ISSUE" ]; then
  SUMMARY="$SUMMARY, deacon=$DEACON_ISSUE"
fi
echo "=== $SUMMARY ==="
```

On success (no issues or issues handled):
```bash
bd create "stuck-agent-dog: $SUMMARY" -t chore --ephemeral \
  -l type:plugin-run,plugin:stuck-agent-dog,result:success \
  -d "$SUMMARY" --silent 2>/dev/null || true
```

On failure:
```bash
bd create "stuck-agent-dog: FAILED" -t chore --ephemeral \
  -l type:plugin-run,plugin:stuck-agent-dog,result:failure \
  -d "Agent health check failed: $ERROR" --silent 2>/dev/null || true

gt escalate "Plugin FAILED: stuck-agent-dog" \
  --severity high \
  --reason "$ERROR"
```
