package configured

import "fmt"

type Hush struct{ Value string }

type Password struct{ Value string }

type Opaque struct{ Value string }

type Holder struct{ Inner Opaque }

type Holder2 struct{ Inner Hush }

func use(h Hush, p Password, o Opaque, hd Holder, hd2 Holder2) {
	fmt.Println(h) // want `configured.Hush exposes sensitive type configured.Hush`
	fmt.Println(p)
	fmt.Println(o)
	fmt.Println(hd)
	fmt.Println(hd2) // want `configured.Holder2 exposes sensitive type configured.Hush`
}
