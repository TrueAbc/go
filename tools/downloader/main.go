package main

import (
	"log"
	"os"
	"runtime"

	"github.com/urfave/cli/v2"
)

func main() {
	// 默认的并发数量
	concurrencyN := runtime.NumCPU() + 1
	app := &cli.App{
		Name:  "downloader",
		Usage: "file concurrency downloader",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:     "url",
				Aliases:  []string{"u"},
				Usage:    "'URL' to download",
				Required: true,
			},
			&cli.StringFlag{
				Name:     "output",
				Aliases:  []string{"o"},
				Usage:    "Output filename",
				Required: true,
			},
			&cli.IntFlag{
				Name:    "concurrency",
				Aliases: []string{"n"},
				Value:   concurrencyN,
				Usage:   "Concurrency 'number'",
			},
		},
		Action: func(c *cli.Context) error {
			strURL := c.String("url")
			filename := c.String("output")
			concurrency := c.Int("concurrency")

			loader := NewDownloader(concurrency, strURL, filename)

			return loader.Download()
		},
	}
	err := app.Run(os.Args)
	if err != nil {
		log.Fatal(err)
	}
}
