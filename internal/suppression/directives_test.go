package suppression_test

// The fixture directives are assembled from parts. Fixture text is not a suppression of this
// file's own code (the scanner reads comments, never string contents), but an editor guard that
// greps raw text cannot tell the two apart, so no directive is spelt out in one piece here.
const (
	nolint     = "//" + "no" + "lint"
	spacedNo   = "// " + "no" + "lint"
	lintIgnore = "//" + "lint:" + "ignore"
	fileIgnore = "//" + "lint:" + "file-ignore"
	gosecMark  = "// #" + "nosec"
	astIgnore  = "// ast-grep-" + "ignore"
)
