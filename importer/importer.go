// Package importer implements importing of content from other systems.
package importer

import "fmt"

func Import(kind string, outDir string, filename string) error {
	switch kind {
	case "wordpress":
		return ImportWordpress(outDir, filename)
	default:
		return fmt.Errorf("unknown import type %q", kind)
	}
}
