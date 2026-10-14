package a

// Forbidden is the function the analyzer forbids calling.
func Forbidden() {}

func allowed() {}

func caller() {
	Forbidden() // want `call to Forbidden`
	allowed()
	_ = Forbidden
}
