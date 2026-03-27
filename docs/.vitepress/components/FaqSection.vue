<script setup lang="ts">
import { ref } from 'vue'

const faqs = [
  {
    q: 'What is ttal?',
    a: 'ttal is a CLI that orchestrates coding agents into an autonomous software team. It handles agent lifecycle, task routing, Telegram messaging, PR review pipelines, and inter-agent communication — so your agents work as a coordinated team, not isolated sessions.',
  },
  {
    q: 'What coding agents does it support?',
    a: 'Claude Code (stable) as the primary runtime. Codex CLI is supported as a worker runtime. You can mix runtimes across your team — run managers on Claude Code and workers on Codex, or any combination.',
  },
  {
    q: 'Do I need Telegram?',
    a: "Telegram is the primary mobile interface — send messages, approve actions, check status, answer agent questions. Without it, you can still use ttal from the terminal via CLI and tmux sessions directly.",
  },
  {
    q: 'How is this different from just running multiple Claude Code sessions?',
    a: "ttal adds the coordination layer: task routing, agent-to-agent messaging, persistent identity with memory, automated PR review pipelines, and a daemon that keeps everything running. Without it, you're manually managing tmux sessions and copy-pasting between agents.",
  },
  {
    q: 'What does the daemon do?',
    a: "The daemon is a background process that handles Telegram polling, agent lifecycle, output watching, PR status monitoring, and message routing between agents and humans. On macOS it's managed by launchd and starts automatically on boot.",
  },
  {
    q: 'Can I run it on Linux?',
    a: 'The CLI works on Linux. The daemon currently uses launchd (macOS) for process management — systemd support is on the roadmap.',
  },
  {
    q: 'How long does setup take?',
    a: 'About 10 minutes. Clone, make install, ttal daemon install, configure your agents in config.toml, and you\'re running.',
  },
  {
    q: 'Does it work with GitHub, GitLab, or only Forgejo?',
    a: 'GitHub and Forgejo are supported natively for PR workflows (create, review, merge). GitLab support is planned.',
  },
  {
    q: "What does 'team' mean in pricing — human users or agent teams?",
    a: 'Agent limits are about AI agents (Claude Code sessions), not human users. There are no per-seat charges for humans. Free includes 2 agents; Pro and Team give unlimited agents.',
  },
  {
    q: 'Can agents talk to each other?',
    a: 'Yes, natively. Agents can message any other agent via ttal send — coordination and delegation work out of the box.',
  },
]

const openIndex = ref(null)
function toggle(i) {
  openIndex.value = openIndex.value === i ? null : i
}
</script>

<template>
  <div class="faq-section">
    <div
      v-for="(faq, i) in faqs"
      :key="i"
      class="faq-item"
      :class="{ open: openIndex === i }"
    >
      <button class="faq-question" :aria-expanded="openIndex === i" @click="toggle(i)">
        <span>{{ faq.q }}</span>
        <span class="faq-icon">{{ openIndex === i ? '−' : '+' }}</span>
      </button>
      <div class="faq-answer" v-show="openIndex === i">
        <p>{{ faq.a }}</p>
      </div>
    </div>
  </div>
</template>

<style scoped>
.faq-section {
  max-width: 48rem;
  margin: 2rem auto;
}

.faq-item {
  border-bottom: 1px solid var(--vp-c-divider);
}

.faq-item:first-child {
  border-top: 1px solid var(--vp-c-divider);
}

.faq-question {
  width: 100%;
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 1rem 0;
  background: none;
  border: none;
  cursor: pointer;
  font-size: 1rem;
  font-weight: 600;
  color: var(--vp-c-text-1);
  text-align: left;
  gap: 1rem;
  font-family: inherit;
}

.faq-question:hover {
  color: var(--vp-c-brand-1);
}

.faq-icon {
  flex-shrink: 0;
  font-size: 1.25rem;
  color: var(--vp-c-text-3);
  width: 1.5rem;
  text-align: center;
}

.faq-answer {
  padding: 0 0 1rem;
}

.faq-answer p {
  margin: 0;
  font-size: 0.95rem;
  line-height: 1.7;
  color: var(--vp-c-text-2);
}
</style>
