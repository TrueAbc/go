package main

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"log"
	"math"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"time"

	pg "github.com/schollz/progressbar/v3"
)

// 固定路径用于进行断点续传
var baseDir = path.Join("", "abcDownloader")

func init() {
	_, err := os.Stat(baseDir) //os.Stat获取文件信息
	if err != nil {
		if os.IsExist(err) {
			return
		}
		os.Mkdir(baseDir, 0770)
	}
}

var tmpDir = path.Join(baseDir, "tmp")

type Downloader struct {
	concurrency int
	url         string
	filename    string

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
		log.Println("range download is avaliable, contentlen is " + strconv.Itoa(int(resp.ContentLength)))
		return d.multiDownload(d.url, d.filename, int(resp.ContentLength))
	}

	return d.singleDownload(d.url, d.filename)
}

func (d *Downloader) multiDownload(strUrl, filename string, contentLen int) error {
	pageSize := int(math.Ceil((float64(contentLen) - float64(d.totalSize)) / float64(d.concurrency)))
	// 创建临时文件夹
	partDir := d.getPartDir(filename)
	log.Println("tmp dir of this download is:" + partDir)
	log.Println("range download of this time froms " + strconv.Itoa(d.totalSize) + " to " + strconv.Itoa(contentLen))
	err := os.MkdirAll(partDir, 0777)
	if err != nil {
		log.Println(err)
		return err
	}
	defer os.RemoveAll(partDir)

	rangeStart := d.totalSize
	if rangeStart > contentLen {
		return nil
	}
	bar := CustomedBar(contentLen, "downloading with "+strconv.Itoa(d.concurrency)+" goroutines")
	bar.Add(rangeStart)

	for i := 0; i < d.concurrency; i++ {
		// 并发请求
		go func(i, rangeStart int) {

			rangeEnd := rangeStart + pageSize
			// 最后一段保证Contenlen
			if rangeEnd >= contentLen {
				rangeEnd = contentLen - 1
			}

			d.downloadPartial(bar, strUrl, filename, rangeStart, rangeEnd)
			// 实现多协程的精确控制
			<-d.tasks[i]
			d.merge(d.filename, rangeStart, rangeEnd)
			d.tasks[i+1] <- struct{}{}

		}(i, rangeStart)

		// 下载完成之后在内部进行拼接
		rangeStart += pageSize + 1
	}
	// 启动了所有的线程之后, 向0号线程发送合并信号
	log.Println("start sendig merging signal")
	d.tasks[0] <- struct{}{}
	// 最终收到最后一个线程发送的结束指令
	<-d.tasks[d.concurrency]
	log.Println("finish merging process")

	return nil
}

// 合并文件, 非线程安全, 通过downloader的通道进行控制
func (d *Downloader) merge(filename string, start, end int) error {
	newval := d.SetTotalSize(start, end)
	if newval != end+1 {
		return errors.New("merge failed for file")
	}

	destFile, err := os.OpenFile(path.Join(baseDir, filename), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		log.Println(err)
		return err
	}

	defer func() {
		fmt.Println("dest file is closed")
		destFile.Close()
	}()

	partFileName := d.getPartFilename(filename, start, end)
	partFile, err := os.Open(partFileName)
	if err != nil {

		return err
	}

	_, err = io.Copy(destFile, partFile)
	if err != nil {
		log.Println(err)
	}
	partFile.Close()
	os.Remove(partFileName)
	log.Println("merge of tmp file:" + partFileName)

	return nil
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
	// filename 加上url的hash值
	var res = fmt.Sprintf("%x", sha256.Sum256([]byte(url)))
	filename = res + "_" + filename

	fi, err := os.Stat(path.Join(baseDir, filename))
	var totalSize int
	if err == nil {
		log.Println("start from breakpoint " + strconv.Itoa(int(fi.Size())))
		totalSize = int(fi.Size())
	}

	return &Downloader{
		concurrency: concurrency,
		url:         url,
		filename:    filename,
		totalSize:   totalSize,
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
