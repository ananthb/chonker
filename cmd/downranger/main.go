package main

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"os"

	"github.com/schollz/progressbar/v3"
	"github.com/spf13/cobra"
	"github.com/sudhirj/ranger"
)

var Source string
var Destination string
var Parallelism int
var ChunkSize int64
var rootCmd = &cobra.Command{
	Use:   "downranger",
	Short: "downranger is a very fast large object downloader. It saturates your connection by downloading multiple byte ranges in parallel.",
	Long:  `A fast downloader that works by fetching byte chunks in parallel.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		req, err := http.NewRequest("GET", Source, nil)
		if err != nil {
			return err
		}
		log.Println("Built request.")
		dest, err := os.Create(Destination)
		if err != nil {
			return err
		}
		log.Println("Created destination.")
		rc := ranger.NewRangingHTTPClient(ranger.NewRanger(ChunkSize), http.DefaultClient, Parallelism)
		resp, err := rc.Do(req)
		if err != nil {
			return err
		}
		log.Println("Starting download...")
		bar := progressbar.DefaultBytes(
			resp.ContentLength,
			"Downloading",
		)
		_, err = io.Copy(io.MultiWriter(dest, bar), resp.Body)

		return err
	},
}

func init() {
	rootCmd.Flags().StringVarP(&Source, "source", "s", "", "Source URL to download from.")
	rootCmd.MarkFlagRequired("source")

	rootCmd.Flags().StringVarP(&Destination, "destination", "d", "", "Destination path to store the file in. Will be overwritten if it exists.")
	rootCmd.MarkFlagRequired("destination")
	rootCmd.MarkFlagFilename("destination")

	rootCmd.Flags().IntVarP(&Parallelism, "parallelism", "p", 4, "Number of chunks to download in parallel.")
	rootCmd.Flags().Int64VarP(&ChunkSize, "chunksize", "c", 8e6, "Default chunk size")
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func main() {
	rootCmd.Execute()
}
