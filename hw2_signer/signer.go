package main

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
	// "runtime"
)

// Over Heat Lock
var OHL = &sync.Mutex{}

const goroutinesNum = 5

type Task struct {
	wg  *sync.WaitGroup
	in  chan interface{}
	out chan interface{}
}

func fastDataSignerMd5(data string) string {
	OHL.Lock()
	out := DataSignerMd5(data)
	time.Sleep(1 * time.Millisecond)
	OHL.Unlock()
	return out

}

func sanitize(dataRaw interface{}) string {
	switch dataRaw.(type) {
	case int:
		return strconv.Itoa(dataRaw.(int))
	case string:
		return dataRaw.(string)
	default:
		panic("This function is supposed to use only data of string type")
	}
}

func SingleHash(in, out chan interface{}) {
	wg := &sync.WaitGroup{}
	for i := range in {
		wg.Add(1)
		go func(i interface{}, out chan interface{}, wg *sync.WaitGroup) {
			defer wg.Done()
			data := sanitize(i)
			a := make(chan string)
			go func(data string) {
				a <- DataSignerCrc32(data)
			}(data)

			b := make(chan string)
			go func(data string) {
				b <- DataSignerCrc32(fastDataSignerMd5(data))
			}(data)
			out <- fmt.Sprint(<-a, "~", <-b)
		}(i, out, wg)
	}
	wg.Wait()

}

func MultiHash(in, out chan interface{}) {
	wg := &sync.WaitGroup{}
	for i := range in {
		wg.Add(1)
		go func(i interface{}, out chan interface{}, wg *sync.WaitGroup) {
			defer wg.Done()
			data := sanitize(i)
			arr := make([]string, 6)
			inner_wg := &sync.WaitGroup{}
			for i := 0; i < 6; i++ {
				inner_wg.Add(1)
				go func(i int, wg *sync.WaitGroup, out *string) {
					defer inner_wg.Done()
					*out = DataSignerCrc32(strconv.Itoa(i) + data)
				}(i, wg, &arr[i])
			}
			inner_wg.Wait()
			out <- strings.Join(arr[:], "")
		}(i, out, wg)
	}
	wg.Wait()
}

func CombineResults(in, out chan interface{}) {
	out_s := make([]string, 0, 10)
	for i_raw := range in {
		i := sanitize(i_raw)
		out_s = append(out_s, i)
	}
	sort.Strings(out_s)
	s := strings.Join(out_s, "_")
	out <- s
}


func RunJob(job job, wg *sync.WaitGroup, in, out chan interface{}) {
	defer wg.Done()
	job(in, out)

}

func ExecutePipeline(jobs ...job) {
	task := Task{
		&sync.WaitGroup{},
		make(chan interface{}),
		make(chan interface{}),
	}
	tasks := make([]Task, 0, 10)
	tasks = append(tasks, task)

	ignitor := jobs[0]
	jobs = jobs[1:]
	for i, job := range jobs {
		task := Task{
			&sync.WaitGroup{},
			tasks[i].out,
			make(chan interface{}),
		}
		tasks = append(tasks, task)
		task.wg.Add(1)
		go RunJob(job, task.wg, task.in, task.out)
	}
	ignitor(tasks[0].in, tasks[0].out)
	for _, task := range tasks {
		close(task.in)
		task.wg.Wait()
	}
	close(tasks[len(tasks)-1].out)
}

func main() {
	freeFlowJobs := []job{
		job(func(in, out chan interface{}) {
			out <- 1
		}),
		job(SingleHash),
		job(func(in, out chan interface{}) {
			for i := range in {
				fmt.Println(i)
			}
		}),
	}
	ExecutePipeline(freeFlowJobs...)
}
