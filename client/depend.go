package main

import (
	"bytes"
	"context"
	"fmt"
	"jiacrontab/model"
	"log"
	"path/filepath"
	"strings"
	"time"
)

func newDepend() *depend {
	return &depend{
		depends: make(chan *dependScript, 100),
	}
}

type depend struct {
	depends chan *dependScript
}

func (d *depend) Add(t *dependScript) {
	d.depends <- t
}

func (d *depend) run() {
	go func() {
		for {
			select {
			case t := <-d.depends:
				go func(t *dependScript) {
					var reply bool
					var logContent []byte
					var errMsg string

					if t.timeout == 0 {
						// 默认超时10分钟
						t.timeout = 600
					}

					ctx, cancel := context.WithTimeout(context.Background(), time.Duration(t.timeout)*time.Second)
					args := strings.Split(t.command+" "+t.args, " ")
					startTime := time.Now()
					start := startTime.UnixNano()
					cmdList := [][]string{args}
					logPath := filepath.Join(globalConfig.logPath, "depend_task")

					err := wrapExecScript(ctx, fmt.Sprintf("%d-%s.log", t.taskId, t.id), cmdList, logPath, &logContent)
					cancel()
					costTime := time.Now().UnixNano() - start
					log.Printf("exec %s %s %s cost %.4fs %v", t.name, t.command, t.args, float64(costTime)/1000000000, err)

					t.logContent = bytes.TrimRight(logContent, "\x00")
					t.done = true
					t.err = err

					if err != nil {
						errMsg = err.Error()
					} else {
						errMsg = ""
					}

					t.dest, t.from = t.from, t.dest

					if !filterDepend(t) {
						err = rpcCall("Logic.DependDone", model.DependsTask{
							Id:           t.id,
							Name:         t.name,
							Dest:         t.dest,
							From:         t.from,
							TaskEntityId: t.taskEntityId,
							TaskId:       t.taskId,
							Command:      t.command,
							LogContent:   t.logContent,
							Err:          errMsg,
							Args:         t.args,
							Timeout:      t.timeout,
						}, &reply)

						if !reply || err != nil {
							log.Printf("task %s %s %s call Logic.DependDone failed! err:%v", t.name, t.command, t.args, err)
						}
					}

				}(t)
			}

		}
	}()
}
