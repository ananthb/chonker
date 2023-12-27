package main

import (
	"flag"
	"log"
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
		log.Fatal(err)
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
					log.Fatal(err)
				}
				chonker.StatsForNerds.WritePrometheus(mf)
				if err := mf.Close(); err != nil {
					log.Fatal(err)
				}
			}
		}()
	}

	cc, err := chonker.NewClient(nil, csize, workers)
	if err != nil {
		log.Fatal(err)
	}

	gc := grab.NewClient()
	gc.HTTPClient = cc

	req, err := grab.NewRequest(outputFile, url)
	if err != nil {
		log.Fatal(err)
	}
	req.IgnoreRemoteTime = true

	resp := gc.Do(req)

	if !quiet {
		if resp.DidResume {
			log.Printf("Resuming download from %s...\n", resp.Request.URL())
		} else {
			log.Printf("Downloading %s...\n", resp.Request.URL())
		}
	}

	var updateTicker *time.Ticker
	if !quiet {
		updateTicker = time.NewTicker(1 * time.Second)
		defer updateTicker.Stop()
		go func() {
			for range updateTicker.C {
				log.Printf(
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
	if resp.Err() != nil {
		log.Fatal(resp.Err())
	} else if !quiet {
		log.Printf(
			"Downloaded %s in %s at %s/s.",
			humanize.IBytes(uint64(resp.BytesComplete())),
			resp.Duration().Round(time.Second),
			humanize.IBytes(uint64(resp.BytesPerSecond())),
		)
	}
}
