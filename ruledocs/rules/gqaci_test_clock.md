# gqaci_test_clock

## What it flags

Any use, as a call or a function stored in a variable, of `time.Now`, `time.Since` or `time.Until`
in a `_test.go` file. Production files are not judged.

## Why

A test that reads the real clock depends on when it runs. It passes on an idle machine and fails
under load, across midnight, around a daylight-saving change or a leap second, and the failure it
had cannot be reproduced. Assertions on elapsed time are the commonest source of flaky tests.

## How to fix correctly

Inject the clock where the code under test reads it, and hand the test a fixed instant:

```go
type Service struct{ now func() time.Time }

func TestExpiry(t *testing.T) {
	fixed := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	svc := Service{now: func() time.Time { return fixed }}
	// ...
}
```

A test that really measures real time (a timeout against a live process) is a licensed exception,
see the allow-list below.

## Limits

Reading the clock through other functions (`time.Sleep`, `time.After`, `time.NewTimer`) is not
judged. Test helpers in non-test files are not judged.

## Allow-list

Which test files may read the clock is a project decision, kept in the project record: an
exception with `covers = "findings"`, the rule `gqaci_test_clock` and the path (or glob) of the
file, with its justification and a review date. There is no comment form and no configuration key.

## Suppression

Blocking. The analyzer has no inline suppression route; the only exception is a project-record
entry with `covers = "findings"` and the rule `gqaci_test_clock`, which names the path it licenses and gives
the justification.
