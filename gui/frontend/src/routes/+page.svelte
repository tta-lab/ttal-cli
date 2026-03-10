<script>
import { ChatService } from "../../bindings/github.com/tta-lab/ttal-cli/gui";

let userName = '';
let daemonOnline = false;
let statusMessage = 'Connecting...';

async function init() {
  try {
    userName = await ChatService.GetUserName();
    daemonOnline = await ChatService.IsDaemonRunning();
    statusMessage = daemonOnline ? 'Daemon online' : 'Daemon offline — read-only mode';
  } catch (err) {
    statusMessage = `Error: ${err}`;
  }
}

init();
</script>

<div class="container">
  <h1>ttal Chat</h1>
  {#if userName}
    <p>Logged in as <strong>{userName}</strong></p>
  {/if}
  <p class="status" class:online={daemonOnline} class:offline={!daemonOnline}>
    {statusMessage}
  </p>
  <p class="hint">Connect to an agent by selecting a contact once the full UI is built.</p>
</div>

<style>
  .status.online  { color: #4ade80; }
  .status.offline { color: #f87171; }
  .hint { color: #94a3b8; font-size: 0.875rem; }
</style>
