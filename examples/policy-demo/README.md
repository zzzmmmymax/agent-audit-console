# Policy demo

Point `AGENT_AUDIT_POLICY_FILE` at `policy.yaml`, then run:

```shell
audit policy validate --file policy.yaml
audit policy simulate --action git_push --command git,push,origin,main
audit policy test --tests policy.tests.yaml
```

The cases demonstrate allow, approval, and deny without executing an action.
