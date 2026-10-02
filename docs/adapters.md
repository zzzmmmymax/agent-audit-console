# Agent adapters

`internal/adapters.AgentAdapter` is the product boundary for agent identity and
action normalization. An adapter exposes a stable descriptor and converts an
adapter-neutral `Action` into the core `events.Event` model. Product-specific
fields belong in evidence or actor metadata; storage and hash-chain code do not
import vendor packages.

The built-in registry contains:

- `codex`: CLI, MCP, filesystem, and Git capture;
- `claude-code`: CLI, MCP, filesystem, and Git capture;
- `cursor`: MCP, filesystem, and Git capture;
- `custom`: CLI and MCP capture for self-developed agents.

CLI runs select an adapter with `audit run --agent NAME`. MCP `start_run`
accepts the same registered names, and subsequent actions use the adapter saved
on the run. Unknown names fail closed instead of silently misclassifying an
agent. A future adapter implements the interface and registers itself without
changing SQLite, policy, rollback, API, or Web contracts.

The project deliberately remains on MCP plus the local Web console. A dedicated
Codex sidebar or Audit Card should only be added after a stable official host
extension API exists.
