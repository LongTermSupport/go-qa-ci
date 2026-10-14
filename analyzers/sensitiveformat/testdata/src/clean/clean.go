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

type Token struct{ raw string }

func (Token) LogValue() slog.Value { return slog.StringValue("[redacted]") }

type Secret struct{ raw string }

func (Secret) GoString() string { return "Secret{}" }

type Password struct{ raw string }

func (Password) Format(f fmt.State, _ rune) { _, _ = f.Write([]byte("[redacted]")) }

type APIKey string

func (APIKey) Error() string { return "api key" }

// Wraps redacts through the String method promoted from its embedded field.
type Wraps struct {
	Credentials
	Count int
}

type Holds struct {
	Cred Credentials
	Tok  Token
}

type Plain struct{ Name string }

type PointerFields struct {
	Next *Credentials
}

func ok(c Credentials, t Token, s Secret, p Password, k APIKey, w Wraps, h Holds, pl Plain, pf PointerFields) {
	fmt.Printf("%v %+v %#v %s", c, c, c, c)
	fmt.Println(t, s, p, k, w, h, pl, pf)
	fmt.Printf("%d %T", 1, c)
	fmt.Println(c.User, &c)
	log.Println(c, t)
	slog.Info("login", "creds", c, "tok", t, "user", c.User)
	slog.Any("tok", t)
	args := []any{c}
	fmt.Println(args...)
}

func generic[T any](v T) { fmt.Println(v) }
