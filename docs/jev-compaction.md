# Jev compaction

OwnCode uses Compact Engine v0.3.0 with independent result reduction.
Completed read results can shrink separately from protected results in the same turn.
Call records, conversation text, attachments, and opaque provider content remain intact.
Writable worker results stay protected. Fresh explore/review reports remain eligible.
Resumed worker calls stay protected when their role cannot be established from the call.

Scoring uses two concurrent requests, a byte limit, and an o200k_base token estimate.
The estimate does not equal provider billing. Original results remain in the Jev archive.
Completion notices include reduction counts, protected groups, rejected groups,
planned requests, and scoring duration.

## Optional summary fallback

Open Settings > Context > Jev summary fallback to enable this option.
It defaults to false. The corresponding global configuration field is:

```json
{"compaction":{"jev":{"summaryFallback":true}}}
```

When Jev returns a failed scoring pass, an unmet budget, or less than 25% token savings,
OwnCode can use its configured summary model. It summarizes the original active history.
Cancellation never starts a fallback. Missing credentials and invalid input remain errors.
A missing summary provider leaves the original Jev outcome unchanged.
The summary path makes another model request and can incur provider costs.
