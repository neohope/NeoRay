# Reflection Tool — Practical Examples

Concrete scenarios showing when and how to use the reflection tool effectively.

## Diagnosis

### "Why can't you search the web?"
```
→ reflection(action="check", key="web_config.enable")
  → False
→ "Web search is disabled. Add web.enable: true to your config to enable it."
```

### "Why did you stop?"
```
→ reflection(action="check", key="max_iterations")
  → 40
→ reflection(action="check", key="_last_usage")
  → {"prompt_tokens": 62000, "completion_tokens": 3000}
→ "I hit the iteration limit (40). The task was complex. I can ask the user if they want to increase it."
```

### "What model are you running?"
```
→ reflection(action="check", key="model")
  → 'anthropic/claude-sonnet-4-20250514'
```

## Adaptive Behavior

### Large codebase analysis
```
→ reflection(action="check")
  → context_window_tokens: 65536
→ reflection(action="set", key="context_window_tokens", value=131072)
  → "Set context_window_tokens = 131072 (was 65536)"
→ "I've expanded my context window to handle this large codebase."
```

### Switching to a faster model for repetitive tasks
```
→ reflection(action="set", key="model", value="anthropic/claude-haiku-4-5-20251001")
  → "Set model = 'anthropic/claude-haiku-4-5-20251001' (was 'anthropic/claude-sonnet-4-20250514')"
→ "Switched to a faster model for these batch tasks."
```

## Cross-Turn Memory

### Remembering user preferences
```
# Turn 1: user says "keep it brief"
→ reflection(action="set", key="user_style", value="concise")
  → "Set scratchpad.user_style = 'concise'"

# Turn 3: new topic
→ reflection(action="check", key="user_style")
  → 'concise'
  (adjusts response style accordingly)
```

### Tracking project context
```
→ reflection(action="set", key="active_branch", value="feat/auth")
→ reflection(action="set", key="test_framework", value="pytest")
→ reflection(action="set", key="has_docker", value=true)
```

## Budget Awareness

### Token-conscious behavior
```
→ reflection(action="check", key="_last_usage")
  → {"prompt_tokens": 58000, "completion_tokens": 12000}
→ "I've consumed ~70k tokens. I'll keep my remaining responses focused."
```
