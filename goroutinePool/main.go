package main

import "sync"

type SimplePool struct {
	wg   sync.WaitGroup
	work chan func() // 任务队列
}
