package red

import (
	"errors"
	"fmt"
	"log"
	"log/slog"
	"os"
)

type Credentials struct {
	User     string
	Password string
}

type Alias = Credentials

type Embedded struct {
	Credentials
	Note string
}

type Nested struct {
	Items []Credentials
	ByKey map[string]Credentials
}

type Hidden struct {
	inner Credentials
}

type Box[T any] struct {
	Value T
}

type Token struct{}

func (Token) String() string { return "[redacted]" }

type Unexported struct {
	token Token
}

type APIToken string

func direct(c Credentials) {
	fmt.Printf("%v", c)                              // want `red.Credentials exposes sensitive type red.Credentials to %v`
	fmt.Printf("%+v", c)                             // want `to %v`
	fmt.Printf("%#v", c)                             // want `to %#v`
	fmt.Printf("%s", c)                              // want `to %s`
	fmt.Println(c)                                   // want `to %v`
	_ = fmt.Sprint(c)                                // want `to %v`
	_ = fmt.Sprintf("%v", &c)                        // want `\*red.Credentials exposes sensitive type red.Credentials`
	_ = fmt.Errorf("login failed: %w", c)            // want `to %w`
	fmt.Fprintln(os.Stderr, c)                       // want `to %v`
	fmt.Fprintf(os.Stderr, "user %s: %v", c.User, c) // want `to %v`
	_ = fmt.Appendf(nil, "%v", c)                    // want `to %v`
}

func aliased(a Alias, e Embedded, n Nested, h Hidden) {
	fmt.Println(a)            // want `red.Alias exposes sensitive type red.Credentials`
	fmt.Println(e)            // want `red.Embedded exposes sensitive type red.Credentials`
	fmt.Println(n)            // want `red.Nested exposes sensitive type red.Credentials`
	fmt.Println(h)            // want `red.Hidden exposes sensitive type red.Credentials`
	fmt.Println(Unexported{}) // want `red.Unexported exposes sensitive type red.Token`
}

func generic(b Box[Credentials], p Box[*Credentials], s []Box[Embedded]) {
	fmt.Printf("%v\n", b) // want `red.Box\[red.Credentials\] exposes sensitive type red.Credentials`
	fmt.Println(s)        // want `exposes sensitive type red.Credentials`
	fmt.Println(p)
}

func named(t APIToken) {
	fmt.Println(t) // want `red.APIToken exposes sensitive type red.APIToken`
}

func logging(c Credentials, e Embedded) {
	log.Printf("login %v", c)                      // want `to %v`
	log.Println(c)                                 // want `to %v`
	log.Default().Print(e)                         // want `red.Embedded exposes sensitive type red.Credentials`
	slog.Info("login", "creds", c)                 // want `to slog`
	slog.Any("creds", e)                           // want `to slog`
	slog.Default().Warn("login", slog.Any("c", c)) // want `to slog`
	slog.With("creds", c).Error("failed")          // want `to slog`
	slog.Info("login", slog.String("user", c.User))
	_ = errors.New("fixed")
}

func indexed(c Credentials) {
	fmt.Printf("%[1]s %[1]v", c)  // want `to %s` `to %v`
	fmt.Printf("%*d %v", 3, 4, c) // want `to %v`
	fmt.Printf("%d %T %v", 1, c, "x")
}

// LogValueToken redacts for slog; fmt never calls LogValue and prints the field.
type LogValueToken struct{ Raw string }

func (LogValueToken) LogValue() slog.Value { return slog.StringValue("[redacted]") }

// GoStringSecret redacts for %#v; fmt calls GoString for no other verb.
type GoStringSecret struct{ Raw string }

func (GoStringSecret) GoString() string { return "GoStringSecret{}" }

// StringPassword redacts for fmt and log; a slog JSON handler marshals the field instead.
type StringPassword struct{ Raw string }

func (StringPassword) String() string { return "[redacted]" }

type WrapsLogValue struct{ Inner LogValueToken }

func wrongMethodForTheSink(l LogValueToken, g GoStringSecret, s StringPassword, w WrapsLogValue) {
	fmt.Println(l)         // want `red.LogValueToken exposes sensitive type red.LogValueToken to %v and implements no redacting String, Error or Format method`
	fmt.Printf("%s", l)    // want `to %s`
	fmt.Printf("%v", g)    // want `to %v`
	fmt.Println(g)         // want `to %v`
	fmt.Printf("%#v", s)   // want `to %#v and implements no redacting GoString or Format method`
	slog.Info("x", "s", s) // want `red.StringPassword exposes sensitive type red.StringPassword to slog and implements no redacting LogValue or Error method`
	slog.Info("x", "g", g) // want `to slog`
	fmt.Println(w)         // want `red.WrapsLogValue exposes sensitive type red.LogValueToken to %v`
	slog.Info("x", "w", w) // want `to slog`
	fmt.Println(s)
	fmt.Printf("%#v", g)
	slog.Info("x", "l", l)
}
