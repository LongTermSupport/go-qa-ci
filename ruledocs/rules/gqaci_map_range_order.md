# gqaci_map_range_order

## What it flags

A `range` over a map (any type whose underlying type is a map) in two situations, and only these:

1. The body writes output: `fmt.Print*`, `fmt.Fprint*`, `log.Print*`, or a `Write`, `WriteString`,
   `WriteByte` or `WriteRune` method of any type.
2. The body appends to a slice declared before the loop (`s = append(s, ...)`), the slice is then
   returned (or is a named result of the function) or is printed or joined (`fmt` printers,
   `strings.Join`) after the loop, and nothing after the loop sorts it (a function of `sort` or
   `slices` that is given the slice).

## Why

Go randomises the iteration order of a map on purpose. Output produced in that order differs from
run to run: golden files flake, two runs of the same command show diffs that are not changes, and a
function's result is a different permutation each call. The defect shows only occasionally, so it
survives review and most test runs.

## How to fix correctly

Collect the keys, sort them, and iterate the sorted keys:

```go
keys := slices.Sorted(maps.Keys(m))
for _, k := range keys {
	fmt.Fprintln(w, k, m[k])
}
```

or sort the slice before it leaves the function (`slices.Sort(out)`).

## Limits

To keep false findings rare the rule reads one function at a time and does not follow a slice
through other variables, fields or calls: a slice appended to in the loop and passed to another
function of the project, or stored in a struct, is not flagged. A slice that is returned and sorted
by the caller is flagged, because the function's own result is order-undefined; sort inside the
function. Writes into another map or a sum over the values are order-insensitive and are not
flagged.

## Suppression

Blocking. The analyzer has no inline suppression route; the only exception is a project-record
entry with `covers = "findings"` and the rule `gqaci_map_range_order`, which names the path it licenses and gives
the justification.
