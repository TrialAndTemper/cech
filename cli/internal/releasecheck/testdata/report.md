# Release check of 0123456789abcdef0123456789abcdef01234567

`cech eval` 0.1.0 wrote this report from its template.
It ran on commit `0123456789abcdef0123456789abcdef01234567` in the image `ghcr.io/trialandtemper/cech-eval@sha256:0000000000000000000000000000000000000000000000000000000000000000`, whose ID is `1111111111111111111111111111111111111111111111111111111111111111`.

## claude-code

- Harness version: 2.1.332.
- Model: `claude-haiku-5-5`.
- Credential: a subscription.
- Runs made or reused: 681.
- Result: not every bar passed.
- Usage over the 680 finished runs with telemetry: 12000 input tokens, 3400000 read from the cache, 90000 written to it, and 21000 output tokens, at a cost of $0.84.

### Bars

| Plugin | Case | Bar | Result | Range | Runs read |
| --- | --- | --- | --- | --- | --- |
| `cech-example` | trailer | content passes with the plugin | pass | 82% to 100% | 60 |
| `cech-example` | trailer | no drop since the latest final release | pass | -12 to +8 points | 60 and 60 |
| `cech-example` | trigger cases | routed skill loads when it should | no result | 61% to 95% | 500 |

Bars that passed, but whose range allows a drop smaller than the threshold:

- `cech-example` trailer, no drop since the latest final release: -12 to +8 points.

### Arms

| Plugin | Case | Arm | Runs | Passed | Errored | Pass rate range |
| --- | --- | --- | --- | --- | --- | --- |
| `cech-example` | trailer | with | 60 | 58 | 0 | 82% to 100% |
| `cech-example` | trailer | without | 60 | 3 | 1 | 0% to 18% |
| `cech-example` | trailer | latest-release | 60 | 59 | 0 | 85% to 100% |

### Graders

| Plugin | Case | Arm | Grader | Passed |
| --- | --- | --- | --- | --- |
| `cech-example` | trailer | with | `trailer-line` | 58 of 60 |
| `cech-example` | trailer | with | `file-written` | 60 of 60 |
| `cech-example` | trailer | without | `trailer-line` | 3 of 60 |
| `cech-example` | trailer | without | `file-written` | 60 of 60 |
| `cech-example` | trailer | latest-release | `trailer-line` | 59 of 60 |
| `cech-example` | trailer | latest-release | `file-written` | 60 of 60 |

### What each skill version costs

| Plugin | Version | Description size (characters) | Tokens added | Run pairs read | Run pairs left out |
| --- | --- | --- | --- | --- | --- |
| `cech-example` | this commit | 85 | 112 | 60 | none |
| `cech-example` | 0.1.0 | 80 | 104 | 60 | none |

### Why runs errored

| Reason | Runs |
| --- | --- |
| the run took longer than 10 minutes | 1 |
| git add: a \| b failed | 1 |

### Cases without a result

- cech-example/control
