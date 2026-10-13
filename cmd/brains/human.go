package main

import (
	"fmt"
	"sort"
)

// printHumanOK prints stable human lines to stdout (for --human).
// Respects NO_COLOR: we use no color codes at all, so NO_COLOR is trivially honored.
func printHumanOK(cmd string, kv map[string]string) {
	fmt.Printf("ok: %s\n", cmd)
	keys := make([]string, 0, len(kv))
	for k := range kv {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		fmt.Printf("%s: %s\n", k, kv[k])
	}
}
