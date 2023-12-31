package main

import (
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/ananthb/chonker"
	"github.com/cavaliergopher/grab/v3"
	"github.com/dustin/go-humanize"
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
	flag.StringVar(&metricsFile, "m", "", "prometheus metrics file")
	flag.StringVar(&outputFile, "o", "", "output file")
	flag.BoolVar(&quiet, "q", false, "quiet mode")
	flag.UintVar(&workers, "w", 10, "number of workers")
}

func main() {
	flag.Parse()

	csize, err := humanize.ParseBytes(chunkSize)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	if flag.NArg() != 1 {
		flag.Usage()
		os.Exit(1)
	}

	url := flag.Arg(0)

	// Write prometheus metrics periodically
	if metricsFile != "" {
		metricsTicker := time.NewTicker(5 * time.Second)
		defer metricsTicker.Stop()

		go func() {
			for range metricsTicker.C {
				mf, err := os.OpenFile(
					metricsFile,
					os.O_CREATE|os.O_WRONLY|os.O_TRUNC,
					0644,
				)
				if err != nil {
					fmt.Println(err)
					os.Exit(1)
				}
				chonker.StatsForNerds.WritePrometheus(mf)
				if err := mf.Close(); err != nil {
					fmt.Println(err)
					os.Exit(1)
				}
			}
		}()
	}

	cc, err := chonker.NewClient(nil, csize, workers)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	gc := grab.NewClient()
	gc.HTTPClient = cc

	req, err := grab.NewRequest(outputFile, url)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	req.IgnoreRemoteTime = true

	resp := gc.Do(req)

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
				fmt.Printf(
					"\033[2K\rTransferred %s/%s (%.2f%%) in %s at %s/s. ETA %s.",
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
	if err := resp.Err(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	} else if !quiet {
		fmt.Printf(
			"\033[2K\rDownloaded %s in %s at %s/s.\n",
			humanize.IBytes(uint64(resp.BytesComplete())),
			resp.Duration().Round(time.Second),
			humanize.IBytes(uint64(resp.BytesPerSecond())),
		)
	}
}
