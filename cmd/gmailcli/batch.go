package main

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"strings"
)

// readIDsFromStdin reads message/label IDs from stdin, one per line
func readIDsFromStdin() ([]string, error) {
	var ids []string
	scanner := bufio.NewScanner(os.Stdin)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line != "" {
			ids = append(ids, line)
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading stdin: %w", err)
	}

	if len(ids) == 0 {
		return nil, fmt.Errorf("no IDs received from stdin")
	}

	return ids, nil
}

// batchProcessor handles batch operations with progress reporting
type batchProcessor struct {
	total     int
	processed int
	errors    []error
	verbose   bool
}

func newBatchProcessor(total int, verbose bool) *batchProcessor {
	return &batchProcessor{
		total:   total,
		verbose: verbose,
		errors:  []error{},
	}
}

// process executes fn for each ID with progress reporting
func (bp *batchProcessor) process(ctx context.Context, ids []string, fn func(context.Context, string) error) error {
	for i, id := range ids {
		if err := fn(ctx, id); err != nil {
			bp.errors = append(bp.errors, fmt.Errorf("ID %s: %w", id, err))
			if bp.verbose {
				fmt.Fprintf(os.Stderr, "Warning: failed to process %s: %v\n", id, err)
			}
		}
		bp.processed++

		// Show progress for large batches
		if bp.verbose && len(ids) > 10 && (i+1)%10 == 0 {
			fmt.Fprintf(os.Stderr, "Progress: %d/%d\n", i+1, len(ids))
		}
	}

	return nil
}

// report prints final batch processing report
func (bp *batchProcessor) report(w io.Writer) {
	fmt.Fprintf(w, "Processed %d/%d items\n", bp.processed-len(bp.errors), bp.total)
	if len(bp.errors) > 0 {
		fmt.Fprintf(w, "Errors: %d\n", len(bp.errors))
		if bp.verbose {
			for _, err := range bp.errors {
				fmt.Fprintf(os.Stderr, "  - %v\n", err)
			}
		}
	}
}
