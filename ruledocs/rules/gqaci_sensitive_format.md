# gqaci_sensitive_format

## What it flags

A value whose type is, or holds, a type with a sensitive name, passed to a printing function with
no redaction in between. "Holds" follows aliases, embedded and named fields, slices, arrays, map
keys and values, a pointer at the top level, and the type arguments of an instantiated generic
type. A name is sensitive when it contains one of the configured fragments (by default password,
secret, token, credential, apikey, privatekey), ignoring case and underscores.

The sinks are `fmt` Print, Sprint, Fprint, Append and Errorf functions (operands formatted by
`%v`, `%+v`, `%#v`, `%s` or `%w`, and every operand of a Print-style call), every `log` function
and `*log.Logger` method, and every operand of a `log/slog` call that is passed as an `any`.

A type is redacting, and nothing is flagged, when its method set has `String() string`,
`GoString() string`, `Error() string`, `Format(fmt.State, rune)` or `LogValue() slog.Value`, or when
it is listed in `-redacting`. An embedded type's method is promoted, so it redacts the outer type
too. fmt does not call methods on a value it reaches through an unexported field, so a redacting
type stored in an unexported field still prints in full and is flagged.

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

// LogValue does the same for log/slog.
func (c Credentials) LogValue() slog.Value { return slog.StringValue(c.String()) }
```

Alternatively print only the safe fields (`c.User`), or, for a type you do not own, list it with
`-redacting` once you have confirmed it redacts itself. Do not rename the type to dodge the name
pattern.

## Limits

An operand typed as an interface, or as a type parameter, is not followed, because its dynamic
type is unknown at the call. Verbs other than the ones above (`%d`, `%x`, `%q`) are not checked.
A pointer nested inside a value prints as an address and is not followed.

## Suppression

Blocking. The analyzer has no inline suppression route; the only exception is a project-record
entry with `covers = "findings"` and the rule `gqaci_sensitive_format`, which names the path it
licenses and gives the justification.
