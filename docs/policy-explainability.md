# Policy explainability

Every new evaluation records the effective rule metadata, matched rule IDs, policy schema version, and a SHA-256 fingerprint inside the existing event JSON. No database column or destructive migration is required. Older events remain valid and display the fingerprint as unavailable.

The fingerprint is derived from canonical semantic JSON, not YAML bytes. Whitespace, indentation, LF/CRLF, and YAML key order do not change it. Rule evaluation and trace ordering are deterministic.

```shell
audit policy explain RUN_ID ACTION_ID
```

The output separates the recorded decision from a current, explicitly `RE-EVALUATED` simulation. If fingerprints differ it reports that policy changed; the system does not claim to reconstruct a historical diff because full policy snapshots are not retained. Full traces are returned for simulation/debugging but are not stored on every event to avoid timeline bloat.

Reload failure retains the last-known-good in-memory policy and changes `audit doctor` policy health to WARN. A process with no valid initial policy fails to start rather than allowing all actions.
