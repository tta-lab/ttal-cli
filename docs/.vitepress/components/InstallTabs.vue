<template>
  <div class="install-tabs">
    <div class="tab-bar">
      <button
        v-for="tab in tabs"
        :key="tab.id"
        class="tab-btn"
        :class="{ active: activeTab === tab.id }"
        @click="activeTab = tab.id"
      >
        {{ tab.label }}
      </button>
    </div>
    <div class="code-block">
      <button class="copy-btn" @click="copy(activeContent)" :class="{ copied }">
        {{ copied ? 'Copied!' : 'Copy' }}
      </button>
      <pre><code>{{ activeContent }}</code></pre>
    </div>
    <div class="post-install">
      <div class="post-install-label">After install:</div>
      <div class="code-block">
        <button class="copy-btn" @click="copy(postInstall)" :class="{ copied: copiedPost }">
          {{ copiedPost ? 'Copied!' : 'Copy' }}
        </button>
        <pre><code>{{ postInstall }}</code></pre>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, computed } from 'vue'

const tabs = [
  { id: 'macos', label: 'macOS' },
  { id: 'linux', label: 'Linux' },
  { id: 'source', label: 'From source' },
]

const activeTab = ref('macos')
const copied = ref(false)
const copiedPost = ref(false)

const content = {
  macos: `brew tap tta-lab/ttal\nbrew install ttal`,
  linux: `go install github.com/tta-lab/ttal-cli@latest`,
  source: `git clone https://github.com/tta-lab/ttal-cli.git\ncd ttal-cli && make install`,
}

const postInstall = `ttal init --scaffold basic    # Quick setup with 2 agents\nttal daemon install            # Start the daemon`

const activeContent = computed(() => content[activeTab.value])

function copy(text) {
  navigator.clipboard.writeText(text)
  if (text === postInstall) {
    copiedPost.value = true
    setTimeout(() => { copiedPost.value = false }, 2000)
  } else {
    copied.value = true
    setTimeout(() => { copied.value = false }, 2000)
  }
}
</script>

<style scoped>
.install-tabs {
  margin: 2rem 0;
  max-width: 600px;
}

.tab-bar {
  display: flex;
  gap: 0;
  margin-bottom: 0;
  border-radius: 8px 8px 0 0;
  overflow: hidden;
  border: 1px solid #2a2a2a;
  border-bottom: none;
}

.tab-btn {
  flex: 1;
  background: #1a1a1a;
  color: #888;
  border: none;
  padding: 0.6rem 1rem;
  font-size: 0.85rem;
  cursor: pointer;
  transition: background 0.15s, color 0.15s;
  font-family: ui-monospace, 'SF Mono', 'Cascadia Code', monospace;
}

.tab-btn:hover {
  background: #222;
  color: #ccc;
}

.tab-btn.active {
  background: #0a0a0a;
  color: #e0e0e0;
  font-weight: 600;
}

.code-block {
  position: relative;
  background: #0a0a0a;
  border: 1px solid #2a2a2a;
  border-radius: 0 0 8px 8px;
  padding: 1.25rem 1.5rem;
}

.code-block pre {
  margin: 0;
  padding: 0;
  background: transparent;
  border: none;
}

.code-block code {
  font-family: ui-monospace, 'SF Mono', 'Cascadia Code', monospace;
  font-size: 0.875rem;
  color: #e0e0e0;
  white-space: pre;
}

.copy-btn {
  position: absolute;
  top: 0.75rem;
  right: 0.75rem;
  background: #2a2a2a;
  color: #888;
  border: 1px solid #3a3a3a;
  border-radius: 4px;
  padding: 0.25rem 0.6rem;
  font-size: 0.75rem;
  cursor: pointer;
  transition: background 0.15s, color 0.15s;
}

.copy-btn:hover {
  background: #3a3a3a;
  color: #e0e0e0;
}

.copy-btn.copied {
  color: #27c93f;
  border-color: #27c93f;
}

.post-install {
  margin-top: 1.5rem;
}

.post-install-label {
  font-size: 0.8rem;
  color: var(--vp-c-text-3);
  margin-bottom: 0.5rem;
  font-weight: 500;
  text-transform: uppercase;
  letter-spacing: 0.06em;
}

.post-install .code-block {
  border-radius: 8px;
}
</style>
