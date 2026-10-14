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
	fmt.Printf("%#v", c)                             // want `to %v`
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
