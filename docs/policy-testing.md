# Policy validation and regression tests

Validate syntax and semantics before deployment:

```shell
audit policy validate --file policy.yaml
audit policy test --tests policy.tests.yaml
```

Both commands support `--json`. Validation rejects unknown fields, invalid risk or decision values, missing/duplicate IDs, malformed matchers, oversized policies, and unsupported versions. It warns for reliably detectable broad or overlapping matchers. A test case supplies an action context and expected risk/decision. Failures show expected, actual, and matched rules and return a non-zero status; a successful simulation, including a `deny` result, returns zero.

The default regression suite covers push, deletion, overwrite, network, and ordinary test commands. See `examples/policy-demo`. These commands are suitable for CI.
