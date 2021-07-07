package main

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	pg "github.com/schollz/progressbar/v3"
)

type Downloader struct {
	concurrency int
}

func (d *Downloader) Download(strUrl, filename string) error {
	if filename == "" {
		filename = path.Base(strUrl)
	}
	log.Println("filename of this download is: " + filename)
	// 获取请求头部, 检测是否支持range下载
	resp, err := http.Head(strUrl)
	if err != nil {
		return err
	}

	if resp.StatusCode == http.StatusOK && resp.Header.Get("Accept-Ranges") == "bytes" {
		// ok for range download
		log.Println("range download is avaliable")
		return d.multiDownload(strUrl, filename, int(resp.ContentLength))
	}

	return d.singleDownload(strUrl, filename)
}

func (d *Downloader) multiDownload(strUrl, filename string, contentLen int) error {
	pageSize := contentLen / d.concurrency
	// 创建临时文件夹
	partDir := d.getPartDir(filename)
	log.Println("tmp dir of this download is:" + partDir)
	os.Mkdir(partDir, 0777)
	defer os.RemoveAll(partDir)

	var wg sync.WaitGroup
	wg.Add(d.concurrency)

	rangeStart := 0
	bar := CustomedBar(contentLen, "downloading with "+strconv.Itoa(d.concurrency)+" goroutines")
	for i := 0; i < d.concurrency; i++ {
		// 并发请求
		go func(i, rangeStart int) {
			defer wg.Done()

			rangeEnd := rangeStart + pageSize
			// 最后一段保证Contenlen
			if rangeEnd > contentLen {
				rangeEnd = contentLen - 1 // range从0开始
			}

			d.downloadPartial(bar, strUrl, filename, rangeStart, rangeEnd, i)
		}(i, rangeStart)

		rangeStart += pageSize + 1
	}

	wg.Wait()
	log.Println("start merge partial files together")
	d.merge(filename, contentLen)

	return nil
}

// 合并文件
func (d *Downloader) merge(filename string, contentLen int) error {
	destFile, err := os.OpenFile(filename, os.O_CREATE|os.O_WRONLY, 0666)
	log.Println("filename of destFile " + filename)
	if err != nil {
		log.Println(err)
		return err
	}

	dstBuf := bufio.NewWriter(destFile)
	defer func() {
		destFile.Close()
		log.Println("finished files merging")
	}()
	bar := CustomedBar(contentLen, "merging file partials")

	for i := 0; i < d.concurrency; i++ {
		partFileName := d.getPartFilename(filename, i)
		partFile, err := os.Open(partFileName)
		src := bufio.NewReader(partFile)
		if err != nil {
			return err
		}

		io.Copy(io.MultiWriter(dstBuf, bar), src)
		partFile.Close()

		os.Remove(partFileName)
	}

	// 最终的落盘
	return dstBuf.Flush()
}

func (d *Downloader) singleDownload(strUrl, filename string) error {
	return nil
}

// 下载文件
func (d *Downloader) downloadPartial(bar *pg.ProgressBar, strUrl, filename string, rangeStart, rangeEnd, i int) {
	if rangeStart >= rangeEnd {
		return
	}

	req, err := http.NewRequest("GET", strUrl, nil)
	if err != nil {
		log.Fatal(err)
	}

	req.Header.Set("Range", fmt.Sprintf("bytes=%d-%d", rangeStart, rangeEnd))
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Fatal(err)
	}
	defer resp.Body.Close()

	flags := os.O_CREATE | os.O_WRONLY
	partFile, err := os.OpenFile(d.getPartFilename(filename, i), flags, 0666)

	if err != nil {
		log.Fatal(err)
	}

	defer partFile.Close()

	_, err = io.Copy(io.MultiWriter(partFile, bar), resp.Body)
	if err != nil {
		if err == io.EOF {
			return
		}
		log.Fatal(err)
	}

}

// 部分文件存放的路径
func (d *Downloader) getPartDir(filename string) string {
	return "./tmp"
}

// 构建部分文件的名字
func (d *Downloader) getPartFilename(filename string, partNum int) string {
	partDir := d.getPartDir(filename)
	return filepath.Join(partDir, filename+"-"+strconv.Itoa(partNum))
}

func NewDownloader(concurrency int) *Downloader {
	return &Downloader{concurrency: concurrency}
}

func CustomedBar(length int, desc string) *pg.ProgressBar {
	bar := pg.NewOptions64(
		int64(length),
		pg.OptionSetDescription(desc),
		pg.OptionShowBytes(true),
		pg.OptionSetWidth(10),
		pg.OptionEnableColorCodes(true),
		pg.OptionShowCount(),
		pg.OptionOnCompletion(func() {
			fmt.Fprint(os.Stderr, "\n")
		}),
		pg.OptionSetTheme(pg.Theme{
			Saucer:        "[green]=[reset]",
			SaucerHead:    "[green]>[reset]",
			SaucerPadding: " ",
			BarStart:      "[",
			BarEnd:        "]",
		}),

		pg.OptionUseANSICodes(true),
		pg.OptionThrottle(time.Second),
		pg.OptionSetPredictTime(true),
		pg.OptionFullWidth(),
	)
	return bar
}
