package main

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"time"

	pg "github.com/schollz/progressbar/v3"
)

const tmpDir = "./tmp"

type Downloader struct {
	concurrency int
	url         string
	filename    string

	contentLen int // 记录单次下载的长度

	totalSize int
	tasks     []chan struct{} //  用于在文件合并的过程中进行控制
}

// 记录当前
func (d *Downloader) SetTotalSize(oldval, newval int) int {
	if oldval == d.totalSize {
		d.totalSize = newval + 1
	}
	return d.totalSize
}

func (d *Downloader) Download() error {

	log.Println("filename of this download is: " + d.filename)

	// 获取请求头部, 检测是否支持range下载
	resp, err := http.Head(d.url)
	if err != nil {
		return err
	}

	if resp.StatusCode == http.StatusOK && resp.Header.Get("Accept-Ranges") == "bytes" {
		// ok for range download
		log.Println("range download is avaliable")
		d.contentLen = int(resp.ContentLength)
		return d.multiDownload(d.url, d.filename, int(resp.ContentLength))
	}

	return d.singleDownload(d.url, d.filename)
}

func (d *Downloader) multiDownload(strUrl, filename string, contentLen int) error {
	pageSize := (contentLen - d.totalSize) / d.concurrency
	// 创建临时文件夹
	partDir := d.getPartDir(filename)
	log.Println("tmp dir of this download is:" + partDir)
	os.Mkdir(partDir, 0777)
	defer os.RemoveAll(partDir)

	rangeStart := 0
	bar := CustomedBar(contentLen, "downloading with "+strconv.Itoa(d.concurrency)+" goroutines")
	for i := 0; i < d.concurrency; i++ {
		// 并发请求
		go func(i, rangeStart int) {

			rangeEnd := rangeStart + pageSize
			// 最后一段保证Contenlen
			if rangeEnd > contentLen {
				rangeEnd = contentLen - 1 // range从0开始
			}

			d.downloadPartial(bar, strUrl, filename, rangeStart, rangeEnd)
			// 实现多协程的精确控制
			<-d.tasks[i]
			fmt.Println("goroutine " + strconv.Itoa(i))
			d.merge(d.filename, rangeStart, rangeEnd)
			d.tasks[i+1] <- struct{}{}

		}(i, rangeStart)

		// 下载完成之后在内部进行拼接
		rangeStart += pageSize + 1
	}
	// 启动了所有的线程之后, 向0号线程发送合并信号
	fmt.Println("start sendig merging signal")
	d.tasks[0] <- struct{}{}
	// 最终收到最后一个线程发送的结束指令
	<-d.tasks[d.concurrency]

	return nil
}

// 合并文件, 非线程安全, 通过downloader的通道进行控制
func (d *Downloader) merge(filename string, start, end int) error {
	newval := d.SetTotalSize(start, end)
	fmt.Printf("start:%d, end:%d, new:%d\n", start, end, newval)
	if newval != end+1 {
		return errors.New("merge failed for file")
	}

	destFile, err := os.OpenFile(filename, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		log.Println(err)
		return err
	}

	dstBuf := bufio.NewWriter(destFile)
	defer func() {
		destFile.Close()
	}()

	partFileName := d.getPartFilename(filename, start, end)
	partFile, err := os.Open(partFileName)
	src := bufio.NewReader(partFile)
	if err != nil {
		return err
	}

	io.Copy(dstBuf, src)
	partFile.Close()
	fmt.Println(partFileName)
	os.Remove(partFileName)

	// 最终的落盘
	return dstBuf.Flush()
}

func (d *Downloader) singleDownload(strUrl, filename string) error {
	return nil
}

// 下载文件
func (d *Downloader) downloadPartial(bar *pg.ProgressBar, strUrl, filename string, rangeStart, rangeEnd int) {
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
	partFile, err := os.OpenFile(d.getPartFilename(filename, rangeStart, rangeEnd), flags, 0666)

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
	return tmpDir
}

// 构建部分文件的名字
func (d *Downloader) getPartFilename(filename string, rangeStart, rangeEnd int) string {
	partDir := d.getPartDir(filename)
	return filepath.Join(partDir, filename+"-"+strconv.Itoa(rangeStart)+"-"+strconv.Itoa(rangeEnd))
}

func NewDownloader(concurrency int, url, filename string) *Downloader {
	tasks := make([]chan struct{}, 0, concurrency+1)
	for i := 0; i < concurrency+1; i++ {
		tasks = append(tasks, make(chan struct{}))
	}
	return &Downloader{
		concurrency: concurrency,
		url:         url,
		filename:    filename,
		contentLen:  -1,
		totalSize:   0,
		tasks:       tasks,
	}
}

// 单纯涉及显示进度条的结构
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
