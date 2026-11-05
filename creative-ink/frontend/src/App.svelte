<script>
  import { SystemHealthCheck } from '../wailsjs/go/main/App.js'

  let response = $state('')
  let error = $state('')

  async function checkHealth() {
    error = ''
    try {
      response = await SystemHealthCheck()
    } catch (err) {
      response = ''
      error = String(err)
    }
  }
</script>

<main>
  <h1>Creative-ink</h1>
  <button onclick={checkHealth}>System Health Check</button>
  {#if response}
    <p class="ok">{response}</p>
  {/if}
  {#if error}
    <p class="err">{error}</p>
  {/if}
</main>

<style>
  main {
    height: 100%;
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    gap: 1rem;
  }

  h1 {
    margin: 0;
    font-weight: 300;
    letter-spacing: 0.05em;
  }

  button {
    padding: 0.5rem 1rem;
    border: 1px solid #3c3c3c;
    border-radius: 4px;
    background: #2d2d2d;
    color: inherit;
    font: inherit;
    cursor: pointer;
  }

  button:hover {
    border-color: var(--accent);
  }

  .ok {
    color: #4ade80;
  }

  .err {
    color: #f87171;
  }
</style>
