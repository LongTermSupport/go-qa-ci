# gqaci_sensitive_format

## What it flags

A value whose type is, or holds, a type with a sensitive name, passed to a printing function with
no redaction in between. "Holds" follows aliases, embedded and named fields, slices, arrays, map
keys and values, a pointer at the top level, and the type arguments of an instantiated generic
type. A name is sensitive when it contains one of the configured fragments (by default password,
secret, token, credential, apikey, privatekey), ignoring case and underscores.

The sinks are `fmt` Print, Sprint, Fprint, Append and Errorf functions (operands formatted by
`%v`, `%+v`, `%#v`, `%s`, `%w`, `%q`, `%x`, `%X` or `%d`, and every operand of a Print-style call), every `log` function
and `*log.Logger` method, and every operand of a `log/slog` call that is passed as an `any`.

Whether a type redacts depends on the sink, because each sink calls different methods:

| Sink | Redacting method (in the method set of the value's type) |
| --- | --- |
| `fmt` and `log`, verbs `%v`, `%+v`, `%s`, `%w`, `%q`, `%x`, `%X` and every Print-style operand | `String() string`, `Error() string` or `Format(fmt.State, rune)` |
| `fmt` and `log`, verb `%#v` | `GoString() string` or `Format(fmt.State, rune)` |
| `fmt` and `log`, verb `%d` | `Format(fmt.State, rune)` only |
| `log/slog` | `LogValue() slog.Value`, `Error() string`, `MarshalJSON() ([]byte, error)` or `MarshalText() ([]byte, error)` |

A type listed in `-redacting` redacts at every sink. An embedded type's method is promoted, so it
redacts the outer type too. fmt does not call `LogValue`, and calls `GoString` only for `%#v`. slog
never calls `String`: a JSON handler marshals the fields (through `MarshalJSON` or `MarshalText`
when the type has one), so a type with only `String` still emits its password. slog also resolves only the top-level value, so a redacting type held in a field of
another value still prints in full through slog. fmt calls the methods of a value it reaches through
an exported field, but not through an unexported one, so a redacting type stored in an unexported
field still prints in full and is flagged.

## Why

`%v` and `%+v` print every field. A password, token or key that reaches a log line, an error message
or a panic is disclosed to everyone who reads it, and a log line is far more widely readable than
the variable that held the secret.

## How to fix correctly

Give the sensitive type a method that prints a fixed placeholder, so every use of it is safe and
no call site has to remember:

```go
type Credentials struct {
	User     string
	Password string
}

// String redacts the password: it is the method fmt and log use for %v and %s.
func (c Credentials) String() string { return c.User + ":[redacted]" }

// GoString does the same for %#v.
func (c Credentials) GoString() string { return c.String() }

// LogValue does the same for log/slog, which never calls String.
func (c Credentials) LogValue() slog.Value { return slog.StringValue(c.String()) }
```

Alternatively print only the safe fields (`c.User`), or, for a type you do not own, list it with
`-redacting` once you have confirmed it redacts itself. Do not rename the type to dodge the name
pattern.

## Limits

An operand typed as an interface, or as a type parameter, is not followed, because its dynamic
type is unknown at the call. Verbs other than the ones above (`%b`, `%o`, `%e`, `%t` and so on)
are not checked. For fmt and log, a pointer nested inside a value prints as an address and is not
followed; slog's JSON handler does marshal through such a pointer, so a sensitive value reached
that way through slog is a missed finding.

## Suppression

Blocking. The analyzer has no inline suppression route; the only exception is a project-record
entry with `covers = "findings"` and the rule `gqaci_sensitive_format`, which names the path it
licenses and gives the justification.
