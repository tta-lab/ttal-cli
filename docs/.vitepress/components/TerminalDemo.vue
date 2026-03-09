<template>
  <div class="terminal-demo">
    <div class="terminal-chrome">
      <div class="terminal-dots">
        <span class="dot dot-red"></span>
        <span class="dot dot-yellow"></span>
        <span class="dot dot-green"></span>
      </div>
      <div class="terminal-title">ttal</div>
    </div>
    <div class="terminal-body">
      <div class="terminal-line">
        <span class="prompt">$</span>
        <span class="cmd"> ttal task execute a3f8b2c1</span>
      </div>
      <div class="terminal-line output">  Spawning worker: <span class="hl-cyan">fix-auth-middleware</span></div>
      <div class="terminal-line output">  Session: <span class="hl-gray">w-a3f8b2c1-fix-auth-middleware</span></div>
      <div class="terminal-line output">  Branch: <span class="hl-gray">worker/fix-auth-middleware</span></div>
      <div class="terminal-line output">  <span class="hl-green">✓</span> Worker spawned</div>
      <div class="terminal-line spacer"></div>
      <div class="terminal-line">
        <span class="prompt">$</span>
        <span class="cmd"> ttal pr create "Fix auth middleware token validation"</span>
      </div>
      <div class="terminal-line output">  PR <span class="hl-cyan">#47</span> created → <span class="hl-gray">github.com/acme/api/pull/47</span></div>
      <div class="terminal-line output">  <span class="hl-green">✓</span> Reviewer spawned in 'review' window</div>
      <div class="terminal-line spacer"></div>
      <div class="terminal-line comment">  <span class="hl-gray"># 6 reviewers analyze code in parallel...</span></div>
      <div class="terminal-line output">  <span class="hl-green">✓</span> Code quality: <span class="hl-green">LGTM</span></div>
      <div class="terminal-line output">  <span class="hl-green">✓</span> Type design: <span class="hl-green">no issues</span></div>
      <div class="terminal-line output">  <span class="hl-green">✓</span> Silent failures: <span class="hl-green">clean</span></div>
      <div class="terminal-line output">  <span class="hl-green">✓</span> Test coverage: <span class="hl-green">adequate</span></div>
      <div class="terminal-line spacer"></div>
      <div class="terminal-line">
        <span class="prompt">$</span>
        <span class="cmd"> ttal pr merge</span>
      </div>
      <div class="terminal-line output">  PR <span class="hl-cyan">#47</span> merged <span class="hl-gray">(squash)</span></div>
      <div class="terminal-line output">  <span class="hl-green">✓</span> Worker cleaned up</div>
      <div class="terminal-line output">  <span class="hl-green">✓</span> Task marked done</div>
    </div>
  </div>
</template>

<style scoped>
.terminal-demo {
  max-width: 700px;
  margin: 2rem auto;
  border-radius: 12px;
  overflow: hidden;
  box-shadow: 0 20px 60px rgba(0, 0, 0, 0.5);
  font-family: ui-monospace, 'SF Mono', 'Cascadia Code', 'Fira Code', monospace;
  font-size: 0.875rem;
  line-height: 1.7;
}

.terminal-chrome {
  background: #2a2a2a;
  padding: 10px 16px;
  display: flex;
  align-items: center;
  gap: 12px;
  border-bottom: 1px solid #1a1a1a;
}

.terminal-dots {
  display: flex;
  gap: 6px;
}

.dot {
  width: 12px;
  height: 12px;
  border-radius: 50%;
}

.dot-red { background: #ff5f56; }
.dot-yellow { background: #ffbd2e; }
.dot-green { background: #27c93f; }

.terminal-title {
  flex: 1;
  text-align: center;
  color: #888;
  font-size: 0.8rem;
  letter-spacing: 0.05em;
}

.terminal-body {
  background: #0a0a0a;
  padding: 20px 24px;
}

.terminal-line {
  display: block;
  color: #e0e0e0;
  white-space: pre;
}

.terminal-line.spacer {
  height: 0.5rem;
}

.prompt {
  color: #646cff;
  font-weight: 700;
  user-select: none;
}

.cmd {
  color: #f0f0f0;
}

.output {
  color: #a0a0a0;
}

.comment {
  color: #555;
}

.hl-green {
  color: #27c93f;
}

.hl-cyan {
  color: #41d1ff;
}

.hl-gray {
  color: #888;
}
</style>
