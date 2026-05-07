# Lenos website v1 — copy polish

**Author:** lyra 🦎
**For:** sage 🦢 (task d362837c)
**Status:** first pass
**Brief:** flicknote `c34523b2`
**Source verified against:** live site (https://tta-lab.github.io/lenos-website/), lenos/README.md, lenos/AGENTS.md, flicknotes 6fff27d6 / 7c7cb9c5 / 44553249
**Voice constraint:** Saint-Exupéry "perfection by subtraction." Calm, evidence-grounded, no hype words. No fabricated specifics.

Below: four surfaces. Each has the live copy, the polished copy, and notes on what changed and why.

---

## Surface 1 — Hero statement + subhead

### Statement (3-7 words)

**Live (default):** *No protocol. Just shell.* — highlight on `shell`.

**Recommendation:** keep `No protocol. Just shell.` It does the most work in the fewest syllables and the negation→assertion rhythm is the strongest of the candidates.

**Three alternatives, distinct in form:**

**A. Negation+alt, sharper terms:** *No tool calls. Just bash.* — highlight on `bash`.
Why: more direct than "protocol" (which devs sometimes parse as HTTP); "bash" is concrete where "shell" is generic. Loses a small amount of the literary register (`tool calls` is dev-jargon where `protocol` reads like prose).

**B. Positive declarative:** *Bash is the protocol.* — highlight on `Bash`.
Why: turns the negation into assertion. Good for a reader who skims past negations to find the claim. Loses the rhythm of the matched-syllable pair.

**C. Philosophical stretch:** *The shell is enough.* — highlight on `enough`.
Why: leans into the "by subtraction" thesis. Provocative; risks reading as smug. Reads better landing on the philosophy block than at the front door.

### Subhead (~25-40 words target)

**Live (38 words):**

> Lenos is an AI runtime with no tool-call layer. Models output raw bash; a sandbox runs it; output comes back. Compatible with every model that speaks shell — fewer tokens, fewer provider quirks, parallel calls for free.

**Polished (27 words):**

> An AI runtime with no tool-call layer. Models write bash; a sandbox runs it. Any model that speaks shell — fewer tokens, fewer provider quirks, parallel for free.

Changes:
- Drops `Lenos is` opener (the wordmark above already names the subject).
- `output raw bash` → `write bash` (shorter; `write` is more agentic than `output`).
- `output comes back` removed (implied by `runs it`).
- `Compatible with every model that speaks shell` → `Any model that speaks shell` (drops the verb-form-as-adjective phrasing; the dash already carries the consequence).
- `parallel calls for free` → `parallel for free` (`parallel` already implies the calls).

Reads in one breath; same four claims preserved (drops layer, model writes bash, sandbox runs it, multi-model, fewer tokens, fewer quirks, parallel free).

---

## Surface 2 — Three differentiator paragraphs

### i. Compatible with any model that speaks bash.

**Live body:**

> Lenos drops the tool-call abstraction. Models output raw shell — `cat`, `src edit`, `web search`, `rg` — and the runtime executes it in a sandbox. No provider-specific tool-call formats to track. No `tool_use_id` mismatches when the model's output drifts from the runtime's schema. No *「please use parallel tool calls」* prompts the model ignores; in bash, parallel is just `cmd1 & cmd2 & wait`. Every model that speaks shell understands lenos.

**Polished (62 words):**

> Lenos drops the tool-call layer. Models write raw shell — `cat`, `src edit`, `web search`, `rg` — and the runtime executes it in a sandbox. No provider-specific schemas to track. No `tool_use_id` mismatches when the model drifts from the schema. And no *「please use parallel tool calls」* prompts the model ignores — in bash, parallel is just `cmd1 & cmd2 & wait`.

Changes:
- `tool-call abstraction` → `tool-call layer` (shorter, more concrete).
- `output raw shell` → `write raw shell` (active verb).
- `No provider-specific tool-call formats to track` → `No provider-specific schemas to track` (`tool-call formats` was redundant with the next sentence about `tool_use_id mismatches`).
- `when the model's output drifts from the runtime's schema` → `when the model drifts from the schema` (drops two possessives; cleaner grammar; same meaning).
- Closing line `Every model that speaks shell understands lenos` removed — the title already says it. Closing on `cmd1 & cmd2 & wait` lands harder; the reader sees the alternative in shell, not in our claim about it.

**Caption (citation footnote) — keep as-is from current live:**

> † real-world examples: [anthropics/claude-code #3886](https://github.com/anthropics/claude-code/issues/3886) (missing tool_result), [ollama/ollama #12395](https://github.com/ollama/ollama/issues/12395) (Anthropic-compat backend pairing failure), [model-switching incompatibility](https://agi-xiaobai-no1.github.io/posts/model-switching-tool-use-compatibility/) (Anthropic `tooluse_xxx` ↔ OpenAI `call_xxx` drift). Every framework that maintains a tool-call schema fights this.

This is doing real work — concrete, sourced, verifiable. Don't touch.

### ii. Sandbox by default. No permission JSON.

**Live body:**

> Lenos ships with the temenos sandbox enabled by default and auto-configured. Filesystem isolation comes for free. No yolo flags to remember, no permission dialogs interrupting your flow, no hundred-line permission JSON to maintain across projects. The agent runs in a box; the box has the rights it needs; you don't think about it.

**Polished (56 words):**

> Lenos ships with the temenos sandbox enabled and auto-configured. Filesystem isolation comes for free. No `--yolo` flags to remember, no permission dialogs interrupting your flow, no hundred-line JSON to maintain across projects. The agent runs in a box; the box has the rights it needs; you don't think about it.

Changes:
- `enabled by default and auto-configured` → `enabled and auto-configured` (the title already says `by default`).
- `yolo flags` → `` `--yolo` flags `` (concrete; backticks make it land as a real flag, not a metaphor).
- `permission JSON` → `JSON` (the title already established `permission JSON`; second instance is shorter).

The closing rhythm — *runs in a box; box has the rights; you don't think about it* — is the strongest cadence on the page. Untouched.

### iii. Less, by design.

**Live body:**

> Bash output costs fewer tokens than structured tool-call JSON. The benchmark above is partly an experiment in this hypothesis. The deeper bet: optimize the most-common path, and everything else follows — simpler runtime, lower cost, fewer abstractions to debug. *「Perfection is achieved, not when there is nothing more to add, but when there is nothing left to take away.」* — Saint-Exupéry, *Terre des Hommes*, 1939.

**Polished (60 words):**

> Bash costs fewer tokens than structured tool-call JSON. The benchmark above tests that. The deeper bet is a design discipline — what's removed enables what's left. Simpler runtime, lower cost, fewer abstractions to debug. *「Perfection is achieved, not when there is nothing more to add, but when there is nothing left to take away.」* — Saint-Exupéry, *Terre des Hommes*, 1939.

Changes:
- `Bash output costs fewer tokens` → `Bash costs fewer tokens` (drops `output`; `Bash` reads as the act, not the artifact).
- `is partly an experiment in this hypothesis` → `tests that` (compresses without weakening — the page already calls the table placeholder).
- `The deeper bet: optimize the most-common path, and everything else follows` → `The deeper bet is a design discipline — what's removed enables what's left.` (this is the rebuild — `what's removed enables what's left` pre-states what Saint-Exupéry will say in his own words. The quote then arrives as the inevitable phrasing of what the prose just claimed, not as a flourish bolted on.)

Result: the quote lands as the period, not the postscript. Same two claims (token savings + design philosophy), better seam.

---

## Surface 3 — Philosophy block

**Title:** *By subtraction.* (kept)

### Paragraph 1 (drop-cap)

**Live:**

> Lenos started as a fork of Crush\* by Charmbracelet. We owe them the foundation. What we changed is less about adding and more about taking away — the tool-call layer, the permission management, the assumption that an AI agent needs its own protocol for talking to a computer.

**Polished:**

> Forks usually grow. Lenos shrank. We started with [Crush](https://github.com/charmbracelet/crush)\* by Charmbracelet — we owe them the foundation — and changed it less by adding and more by taking away: the tool-call layer, the permission management, the assumption that an AI agent needs its own protocol for talking to a computer.

The drop-cap "F" of *Forks* now carries weight that earns the visual emphasis. *Forks usually grow. Lenos shrank.* is two short sentences that stake the design move in seven words. A reader who only stops here gets the thesis.

The original `Lenos started as a fork of Crush by Charmbracelet` is true but flat — the drop-cap was decorating a context-setter. New version makes the cap do work.

### Paragraph 2

**Live:**

> What's left is small: a model, a sandboxed shell, a transcript. Sandbox isolation comes for free from temenos. Multi-model support comes for free from speaking shell instead of tool-call JSON. Lower cost comes for free from fewer tokens. Each removal made the next one possible.

**Polished:**

> What's left is small: a model, a sandboxed shell, a transcript. Sandbox isolation comes for free from temenos. Multi-model support comes for free from shell instead of JSON. Lower cost comes for free from fewer tokens. Each removal made the next one possible.

Change:
- `from speaking shell instead of tool-call JSON` → `from shell instead of JSON` (the parallel structure — three `comes for free from X` clauses — sharpens when the X clauses match in length).

### Paragraph 3 (CTA)

**Live:**

> If you've been frustrated by your runtime's tool-call edge cases, your agent's permission management, or your model's reluctance to run parallel tool calls — the docs are a fast read.

**Polished:**

> If you've ever fought your runtime's tool-call edge cases, wrestled permission management, or watched your model ignore *「use parallel tool calls」* — the docs are a fast read.

Changes:
- `frustrated by` → `fought` (the reader is the protagonist; active verb).
- `your agent's permission management` → `wrestled permission management` (drops the possessive-of-possessive; active verb).
- `your model's reluctance to run parallel tool calls` → `watched your model ignore *「use parallel tool calls」*` (concretizes the abstract failure into the actual experience — seeing the prompt get ignored).

Each clause now names a specific dev experience the reader has had. The CTA lands harder.

### λ etymology footnote — keep verbatim.

> *λ — lenos in lowercase greek; from λῆνος, the wine-press. A vat where input is gathered, transformed, and refined into output. An old name for an old idea; we like the lineage.*

Already does what it should.

---

## Surface 4 — Blog post #1

**Slug:** `2026-05-introducing-lenos.mdx`
**Title:** *Lenos — an AI runtime by subtraction*

I dropped *Introducing* from the working title. The post itself is the introduction; tagging it twice is the kind of tic the brief said to avoid. The colon-form *Lenos: an AI runtime by subtraction* is also fine; em-dash reads as more conversational.

**Word count: ~870 words.** Inside the 800-1200 target. Reading time ~4-5 minutes.

**No fabricated specifics.** Token-savings claim stays qualitative (`early runs are encouraging`); concrete numbers live on the benchmarks page once the harness ships. This was sage's hard line and the right one.

---

[drop-cap] Most AI runtimes ship a protocol. Lenos doesn't.

The shell is already there. It's universal, well-understood by every model trained on internet-scale text, and it has a forty-year track record of giving programs ways to compose. So we built a runtime where the model writes bash, a sandbox runs it, and the output comes back as text. No tool-call layer. No permission JSON. No vendor-specific schemas to chase. Every model that speaks shell understands lenos.

### Why we forked Crush

Lenos started as a fork of [Crush](https://github.com/charmbracelet/crush) by Charmbracelet. We're indebted to that foundation. Crush got the terminal experience, the session model, and the editor integration right in ways we did not want to rebuild — so we didn't.

What pushed us to fork rather than upstream was a difference of premise. Crush, like most agent runtimes, treats the tool-call protocol as the primary interface between model and machine. That premise carries cost: every provider gets its own JSON dialect, every new tool needs schema-side maintenance, and the model regularly ignores the meta-prompts that try to make tool use parallel or efficient.

Anyone who's run an agent through enough sessions has seen the symptoms. *`tool_use ids were found without tool_result blocks`* — published as a bug against Anthropic's claude-code. *`unexpected tool_use_id found in tool_result blocks`* — the same shape on OpenClaw. *`Invalid tool usage: mismatch between tool calls and tool results`* — Ollama's variant when serving an Anthropic-compatible API. Format drift on model swap, where Anthropic's `tooluse_xxx` and OpenAI's `call_xxx` refuse to reconcile mid-session. Every framework that maintains a tool-call schema fights one of these modes.

We chose not to maintain a schema. So we kept the parts of Crush we admired — the TUI, the multi-provider plumbing, the session model — and removed the parts whose costs we no longer wanted to pay. The tool-call layer is gone. The permission management is gone. What's left is small enough to read in one sitting and, we hope, simple enough to trust.

### What's different

Four claims, each verifiable.

**Compatibility comes from speaking shell.** Models that don't have native tool-call support — and there are a lot of them, especially among the open-weight class — work in lenos as well as the frontier models. There is no schema to align on, only the contract every Unix-trained model already knows: read input, write commands, observe output, repeat.

**Parallel is the default, not the prompt.** Anthropic has [publicly noted](https://docs.claude.com/en/docs/agents-and-tools/tool-use/overview) that models often ignore the *「use parallel tool calls」* instruction even when it would obviously help. In bash, parallel isn't an instruction the model needs to remember — it's `cmd1 & cmd2 & wait`. The shell does the work the prompt was failing to do.

**Token cost drops because the format is leaner.** Outputting raw shell is cheaper than emitting structured tool-call JSON. Early runs are encouraging; the [v1.0.0 benchmark harness](/benchmarks) will tell us whether the savings hold across task shapes. We'd rather show real numbers later than plausible numbers now.

**Sandbox isolation is the default, not the opt-in.** Lenos ships with the [temenos](https://github.com/tta-lab/temenos) sandbox enabled and auto-configured at install. Filesystem isolation comes for free. There is no `--yolo` flag to remember when you'd rather just trust the agent, no permission dialog interrupting flow, no hundred-line JSON to maintain across projects. The agent runs in a box. The box has the rights it needs. You don't think about it.

### What's next

The next milestone is the full Terminal Bench 2.0 run across the model surface lenos targets — gpt-5.4, deepseek-v4-pro, deepseek-v4-flash, mimo-v2.5, and mimo-v2.5-pro. The placeholder numbers on the [benchmarks page](/benchmarks) get replaced with real ones the moment that run lands.

Beyond that, the public roadmap is short and concrete:

- Cross-runtime comparison rows on the benchmarks page (lenos vs codex vs forgecode vs aider on a fixed model).
- More skills in the standard library — search, web, git, git review.
- Richer transcript tooling: structured replay, diff between runs, export to markdown.
- First-class provider integrations as more open-weight models ship 1M-context capability.
- A docs sweep so the install/usage/configuration trio reads in one sitting.

If you've ever fought your runtime's tool-call edge cases, wrestled permission management, or watched your model ignore *「use parallel tool calls」* — install lenos and try a session. The docs are a fast read.

*— the lenos team*

---

## Voice notes (for sage's review)

1. **Concrete failures over abstract complaint.** The blog post's "Why we forked Crush" section names four specific error messages with their public sources. This is the post's strongest paragraph because the reader recognizes the shape of failures they've personally hit. The brief said "evidence-grounded"; this is what that looks like in prose.

2. **Token claim stays qualitative.** *Early runs are encouraging* is the strongest honest framing for n=1-on-an-informal-bench. *We'd rather show real numbers later than plausible numbers now* makes the discipline visible and turns it into a small piece of brand evidence on its own.

3. **Closing CTA mirrors the philosophy block's CTA.** Same shape (`if you've fought X, wrestled Y, watched Z — install/docs are a fast read`), so a reader who scans both surfaces feels the same hand wrote both. Intentional.

4. **No exclamation points, no "we're excited", no emoji.** Per brief.

5. **Drop-cap discipline.** Two surfaces use a drop-cap (philosophy + blog opener). Both now have first sentences that earn the visual emphasis: *Forks usually grow. Lenos shrank.* and *Most AI runtimes ship a protocol. Lenos doesn't.* Short, declarative, contrarian.

6. **One small structural request:** the differentiator iii. Saint-Exupéry quote currently sits inside the body paragraph. It works as polished, but if the design ever has space for it, pulling that quote into a true epigraph (italic, set-off, with attribution below) would let the body run a touch shorter and let the quote do its real work. Filed as future polish, not v1 blocker.

---

*delivered: 2026-05-06 — lyra 🦎*
