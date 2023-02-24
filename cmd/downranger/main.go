package main

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
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
		sourceURL, err := url.Parse(Source)
		if err != nil {
			return err
		}

		contentLength, err := ranger.GetContentLength(sourceURL, http.DefaultClient)
		if err != nil {
			return err
		}

		dest, err := os.Create(Destination)
		if err != nil {
			return err
		}

		nr := ranger.NewRanger(ChunkSize)
		loader := ranger.DefaultHTTPLoader(sourceURL)
		rs := ranger.NewRangedSource(contentLength, loader, nr)

		mw := io.MultiWriter(dest, progressbar.DefaultBytes(
			contentLength,
			"Downloading",
		))
		_, err = io.Copy(mw, rs.LookaheadReader(Parallelism))

		return err
	},
}

func init() {
	rootCmd.Flags().StringVarP(&Source, "source", "s", "", "Source URL to download from.")
	_ = rootCmd.MarkFlagRequired("source")

	rootCmd.Flags().StringVarP(&Destination, "destination", "d", "", "Destination path to store the file in. Will be overwritten if it exists.")
	_ = rootCmd.MarkFlagRequired("destination")
	_ = rootCmd.MarkFlagFilename("destination")

	rootCmd.Flags().IntVarP(&Parallelism, "parallelism", "p", 4, "Number of chunks to download in parallel.")
	rootCmd.Flags().Int64VarP(&ChunkSize, "chunksize", "c", 8e6, "Default chunk size")
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func main() {
	_ = rootCmd.Execute()
}
