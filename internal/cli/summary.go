package cli

import (
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/aviddiviner/go-hurl/internal/runner"
)

func eprintln(message string) {
	fmt.Fprintln(os.Stderr, message)
}

// Summary returns the text summary of the runs results, used in --test mode.
func Summary(results []*runner.HurlResult, duration time.Duration) string {
	totalFiles := len(results)
	totalRequests := 0
	successFiles := 0
	for _, r := range results {
		for _, e := range r.Entries {
			totalRequests += len(e.Calls)
		}
		if r.Success {
			successFiles++
		}
	}
	durationMs := duration.Milliseconds()
	requestsRate := 0.0
	if durationMs > 0 {
		requestsRate = 1000 * float64(totalRequests) / float64(durationMs)
	}
	successPercent := float32(100) * float32(successFiles) / float32(totalFiles)
	failed := totalFiles - successFiles
	failedPercent := float32(100) * float32(failed) / float32(totalFiles)
	return fmt.Sprintf("--------------------------------------------------------------------------------\n"+
		"Executed files:    %d\n"+
		"Executed requests: %d (%s/s)\n"+
		"Succeeded files:   %d (%s%%)\n"+
		"Failed files:      %d (%s%%)\n"+
		"Duration:          %d ms (%s)\n",
		totalFiles,
		totalRequests, strconv.FormatFloat(requestsRate, 'f', 1, 64),
		successFiles, strconv.FormatFloat(float64(successPercent), 'f', 1, 32),
		failed, strconv.FormatFloat(float64(failedPercent), 'f', 1, 32),
		durationMs, formatDuration(duration))
}

func formatDuration(d time.Duration) string {
	total := d.Milliseconds()
	return fmt.Sprintf("%dh:%dm:%ds:%dms", total/3600000, (total%3600000)/60000, (total%60000)/1000, total%1000)
}
