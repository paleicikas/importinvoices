package main

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: go run check_coverage.go coverage_output.txt")
		os.Exit(1)
	}

	file, err := os.Open(os.Args[1])
	if err != nil {
		fmt.Printf("Error opening file: %v\n", err)
		os.Exit(1)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	failed := false
	for scanner.Scan() {
		line := scanner.Text()
		// Example line: ok  	github.com/paleicikas/importinvoices/server/internal/worker	1.760s	coverage: 74.5% of statements
		// Or: ?   	github.com/paleicikas/importinvoices/server/internal/domain	[no test files]
		if !strings.Contains(line, "coverage:") {
			if strings.Contains(line, "[no test files]") {
				pkg := strings.Fields(line)[1]
				// We only care if it's not domain
				if !strings.HasSuffix(pkg, "/domain") {
					fmt.Printf("Package %s has no test files, want 100.0%% coverage\n", pkg)
					failed = true
				}
			}
			continue
		}

		parts := strings.Fields(line)
		// parts: [ok, pkg, duration, coverage:, XX.X%, of, statements]
		if len(parts) < 5 {
			continue
		}

		pkg := parts[1]
		covStr := strings.TrimSuffix(parts[4], "%")
		
		if strings.HasSuffix(pkg, "/domain") {
			continue
		}

		cov, err := strconv.ParseFloat(covStr, 64)
		if err != nil {
			continue
		}

		if cov < 100.0 {
			fmt.Printf("Package %s has only %.1f%% coverage, want 100.0%%\n", pkg, cov)
			failed = true
		}
	}

	if failed {
		os.Exit(1)
	}
	fmt.Println("All packages have 100% coverage!")
}
