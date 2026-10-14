package clean

import (
	"fmt"
	"log"
	"log/slog"
)

type Credentials struct {
	User     string
	Password string
}

func (c Credentials) String() string { return c.User + ":[redacted]" }

func (c Credentials) GoString() string { return "Credentials{}" }

func (c Credentials) LogValue() slog.Value { return slog.StringValue(c.String()) }

// Token redacts only for slog, so it is printed only there.
type Token struct{ raw string }

func (Token) LogValue() slog.Value { return slog.StringValue("[redacted]") }

// Secret redacts only for %#v, so it is printed only with that verb.
type Secret struct{ raw string }

func (Secret) GoString() string { return "Secret{}" }

// Password redacts for every fmt verb and for log, but not for slog.
type Password struct{ raw string }

func (Password) Format(f fmt.State, _ rune) { _, _ = f.Write([]byte("[redacted]")) }

// APIKey is an error: fmt prints its message and slog does too.
type APIKey string

func (APIKey) Error() string { return "api key" }

// Wraps redacts through the String method promoted from its embedded field.
type Wraps struct {
	Credentials
	Count int
}

type Holds struct {
	Cred Credentials
	Tok  Credentials
}

type Plain struct{ Name string }

type PointerFields struct {
	Next *Credentials
}

// JSONSecret redacts for slog handlers, which marshal through MarshalJSON and MarshalText.
type JSONSecret struct{ raw string }

func (JSONSecret) MarshalJSON() ([]byte, error) { return []byte(`"[redacted]"`), nil }

type TextSecret struct{ raw string }

func (TextSecret) MarshalText() ([]byte, error) { return []byte("[redacted]"), nil }

func marshalers(j JSONSecret, t TextSecret) {
	slog.Info("x", "j", j, "t", t)
}

func ok(c Credentials, t Token, s Secret, p Password, k APIKey, w Wraps, h Holds, pl Plain, pf PointerFields) {
	fmt.Printf("%v %+v %#v %s", c, c, c, c)
	fmt.Printf("%v %s", p, k)
	fmt.Printf("%#v", s)
	fmt.Printf("%#v", p)
	fmt.Println(p, k, w, h, pl, pf)
	fmt.Printf("%d %T", 1, c)
	fmt.Println(c.User, &c)
	log.Println(c, p)
	slog.Info("login", "creds", c, "tok", t, "key", k, "user", c.User)
	slog.Any("tok", t)
	slog.Any("creds", w)
	args := []any{c}
	fmt.Println(args...)
}

func generic[T any](v T) { fmt.Println(v) }
