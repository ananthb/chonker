package main

import (
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/ananthb/chonker"
	"github.com/cavaliergopher/grab/v3"
	"github.com/dustin/go-humanize"
	"golang.org/x/term"
)

var (
	chunkSize   string
	metricsFile string
	outputFile  string
	quiet       bool
	workers     uint
)

func init() {
	flag.StringVar(&chunkSize, "c", "1MiB", "chunk size (e.g. 1MiB, 1GiB)")
	flag.StringVar(&metricsFile, "m", "", "write prometheus metrics to file (default: disabled)")
	flag.StringVar(&outputFile, "o", "", "output file or directory (default: current directory)")
	flag.BoolVar(&quiet, "q", false, "quiet")
	flag.UintVar(&workers, "w", 10, "number of workers")
}

func main() {
	flag.Parse()

	csize, err := humanize.ParseBytes(chunkSize)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	if flag.NArg() != 1 {
		flag.Usage()
		os.Exit(1)
	}

	url := flag.Arg(0)

	// Write prometheus metrics periodically
	var metricsTicker *time.Ticker
	if metricsFile != "" {
		metricsTicker = time.NewTicker(5 * time.Second)
		defer metricsTicker.Stop()

		go func() {
			for range metricsTicker.C {
				writeMetricsFile(metricsFile)
			}
		}()
	}

	cc, err := chonker.NewClient(nil, csize, workers)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	gc := grab.NewClient()
	gc.HTTPClient = cc

	req, err := grab.NewRequest(outputFile, url)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	req.IgnoreRemoteTime = true

	resp := gc.Do(req)

	isTerm := term.IsTerminal(int(os.Stdout.Fd()))
	if !quiet {
		fmt.Printf("Downloading %s\n", resp.Request.URL())
		if resp.DidResume {
			fmt.Printf("Resuming download from (%.2f%%)\n", 100*resp.Progress())
		}
	}

	var updateTicker *time.Ticker
	if !quiet {
		updateTicker = time.NewTicker(1 * time.Second)
		defer updateTicker.Stop()
		go func() {
			for range updateTicker.C {
				if isTerm {
					// Clear the current line
					fmt.Print("\033[2K\r")
				}
				fmt.Printf(
					"Transferred %s/%s (%.2f%%) in %s at %s/s. ETA %s.",
					humanize.IBytes(uint64(resp.BytesComplete())),
					humanize.IBytes(uint64(resp.Size())),
					100*resp.Progress(),
					resp.Duration().Round(time.Second),
					humanize.IBytes(uint64(resp.BytesPerSecond())),
					humanize.Time(resp.ETA()),
				)
			}
		}()
	}

	<-resp.Done
	if updateTicker != nil {
		updateTicker.Stop()
	}
	if metricsTicker != nil {
		metricsTicker.Stop()
		writeMetricsFile(metricsFile)
	}

	if err := resp.Err(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	if !quiet {
		if isTerm {
			fmt.Print("\033[2K\r")
		}
		fmt.Printf(
			"Downloaded %s in %s at %s/s.\n",
			humanize.IBytes(uint64(resp.BytesComplete())),
			resp.Duration().Round(time.Second),
			humanize.IBytes(uint64(resp.BytesPerSecond())),
		)
	}
}

func writeMetricsFile(filename string) {
	f, err := os.Create(filename)
	if err != nil {
		panic(err)
	}
	chonker.StatsForNerds.WritePrometheus(f)
	if err := f.Close(); err != nil {
		panic(err)
	}
}
