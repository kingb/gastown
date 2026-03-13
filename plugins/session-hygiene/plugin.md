+++
name = "session-hygiene"
description = "Clean up zombie tmux sessions and orphaned dog sessions"
version = 1

[gate]
type = "manual"
duration = "30m"

[tracking]
labels = ["plugin:session-hygiene", "category:cleanup"]
digest = true

[execution]
timeout = "5m"
notify_on_failure = true
severity = "low"
+++

# Session Hygiene

Identifies and kills zombie tmux sessions (wrong prefix, no registered rig)
and orphaned dog sessions (tmux session exists but dog not in kennel).

## Step 1: Get known session IDs

Fetch authoritative session IDs from `gt session list` — these are the exact
tmux session names Gas Town manages (rig agents + polecats):

```bash
SESSION_JSON=$(gt session list --json 2>/dev/null)
if [ $? -ne 0 ] || [ -z "$SESSION_JSON" ]; then
  echo "SKIP: could not get session list"
  exit 0
fi

# Extract session IDs into a newline-delimited list
KNOWN_SESSIONS=$(echo "$SESSION_JSON" | jq -r '.[].session_id // empty' 2>/dev/null)
if [ -z "$KNOWN_SESSIONS" ]; then
  echo "SKIP: no sessions found in registry"
  exit 0
fi
```

## Step 2: List tmux sessions

```bash
SESSIONS=$(tmux list-sessions -F '#{session_name}' 2>/dev/null)
if [ $? -ne 0 ] || [ -z "$SESSIONS" ]; then
  echo "No tmux sessions running"
  exit 0
fi

SESSION_COUNT=$(echo "$SESSIONS" | wc -l | tr -d ' ')
```

## Step 3: Identify zombie sessions

A session is legitimate if it exactly matches a known session ID from
`gt session list` or belongs to the `hq-*` namespace (town-level agents:
deacon, dogs, mayor — not tracked in `gt session list`).

```bash
ZOMBIES=()
RIG_SESSION_COUNT=0

while IFS= read -r SESSION; do
  [ -z "$SESSION" ] && continue

  # Allow hq-* sessions (town-level agents: deacon, dogs, mayor)
  case "$SESSION" in
    hq-*) continue ;;
  esac

  # This is a rig-level session — count it for the wipeout guard
  RIG_SESSION_COUNT=$((RIG_SESSION_COUNT + 1))

  # Check against known session IDs (exact match)
  VALID=false
  while IFS= read -r KNOWN; do
    if [ "$SESSION" = "$KNOWN" ]; then
      VALID=true
      break
    fi
  done <<< "$KNOWN_SESSIONS"

  if [ "$VALID" = "false" ]; then
    ZOMBIES+=("$SESSION")
  fi
done <<< "$SESSIONS"

# Total-wipeout guard: if every non-hq session would be killed,
# our detection is broken — not the sessions. Abort and escalate.
if [ "${#ZOMBIES[@]}" -gt 0 ] && [ "${#ZOMBIES[@]}" -eq "$RIG_SESSION_COUNT" ]; then
  echo "ABORT: all $RIG_SESSION_COUNT rig sessions flagged as zombies — detection is broken, not sessions"
  gt escalate "session-hygiene: total-wipeout guard triggered" \
    -s HIGH \
    --reason "All $RIG_SESSION_COUNT non-hq sessions would be killed. This indicates the zombie detection logic is broken (gt session list may be stale). Flagged sessions: ${ZOMBIES[*]}"
  exit 0
fi
```

## Step 4: Kill zombie sessions

```bash
KILLED=0
for ZOMBIE in "${ZOMBIES[@]}"; do
  echo "Killing zombie session: $ZOMBIE"
  tmux kill-session -t "$ZOMBIE" 2>/dev/null && KILLED=$((KILLED + 1))
done
```

## Step 5: Check for orphaned dog sessions

Dog sessions follow the pattern `hq-dog-<name>`. Cross-reference against
the kennel to find sessions for dogs that no longer exist:

```bash
DOG_JSON=$(gt dog list --json 2>/dev/null || echo "[]")
KNOWN_DOGS=$(echo "$DOG_JSON" | jq -r '.[].name // empty' 2>/dev/null)

ORPHANED=0
while IFS= read -r SESSION; do
  [ -z "$SESSION" ] && continue

  # Match hq-dog-* pattern
  case "$SESSION" in
    hq-dog-*)
      DOG_NAME="${SESSION#hq-dog-}"

      # Check if this dog exists in the kennel
      FOUND=false
      while IFS= read -r DOG; do
        if [ "$DOG_NAME" = "$DOG" ]; then
          FOUND=true
          break
        fi
      done <<< "$KNOWN_DOGS"

      if [ "$FOUND" = "false" ]; then
        echo "Killing orphaned dog session: $SESSION (dog '$DOG_NAME' not in kennel)"
        tmux kill-session -t "$SESSION" 2>/dev/null && ORPHANED=$((ORPHANED + 1))
      fi
      ;;
  esac
done <<< "$SESSIONS"
```

## Record Result

```bash
SUMMARY="Checked $SESSION_COUNT sessions: $KILLED zombie(s) killed, $ORPHANED orphaned dog session(s) killed, ${#ZOMBIES[@]} zombie(s) found"
echo "$SUMMARY"
```

On success:
```bash
bd create "session-hygiene: $SUMMARY" -t chore --ephemeral \
  -l type:plugin-run,plugin:session-hygiene,result:success \
  -d "$SUMMARY" --silent 2>/dev/null || true
```

On failure:
```bash
bd create "session-hygiene: FAILED" -t chore --ephemeral \
  -l type:plugin-run,plugin:session-hygiene,result:failure \
  -d "Session hygiene failed: $ERROR" --silent 2>/dev/null || true

gt escalate "Plugin FAILED: session-hygiene" \
  --severity low \
  --reason "$ERROR"
```
