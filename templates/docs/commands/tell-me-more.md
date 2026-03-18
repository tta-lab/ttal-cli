---
name: tell-me-more
description: "Elaborate on a concept just mentioned — explain from knowledge, not search"
argument-hint: "[topic]"
claude-code: {}
---

# Tell Me More

## Overview

When the user asks "tell me more about X", they want you to explain something you just said in more depth. This typically happens during PR reviews, triage, or technical discussions when you used a term or concept the user wants to understand better.

**This is NOT a research skill.** Don't web search or look things up. Explain from your own knowledge and the conversation context.

## How to Respond

1. **Identify what X refers to** — it's usually something you just said or referenced
2. **Explain it clearly** — assume the user is technical but unfamiliar with this specific concept
3. **Connect it to context** — relate it back to the current discussion (the PR, the code, the triage)
4. **Keep it concise** — 2-4 paragraphs max, not a lecture
5. **Use examples** — concrete examples from the current codebase or discussion help most

## What NOT to Do

- Don't web search or query external sources — use your own knowledge
- Don't give a textbook definition — relate it to what you're working on
- Don't over-explain — if the user wants more, they'll ask again
- Don't lose the thread — after explaining, return to the task at hand
