/*
findlock.go

Copyright (c) 2016, Luka Napotnik <luka@zeta.si>
All rights reserved.

Redistribution and use in source and binary forms, with or without
modification, are permitted provided that the following conditions are met:
    * Redistributions of source code must retain the above copyright
      notice, this list of conditions and the following disclaimer.
    * Redistributions in binary form must reproduce the above copyright
      notice, this list of conditions and the following disclaimer in the
      documentation and/or other materials provided with the distribution.
    * Neither the name of the <organization> nor the
      names of its contributors may be used to endorse or promote products
      derived from this software without specific prior written permission.

THIS SOFTWARE IS PROVIDED BY THE COPYRIGHT HOLDERS AND CONTRIBUTORS "AS IS" AND
ANY EXPRESS OR IMPLIED WARRANTIES, INCLUDING, BUT NOT LIMITED TO, THE IMPLIED
WARRANTIES OF MERCHANTABILITY AND FITNESS FOR A PARTICULAR PURPOSE ARE
DISCLAIMED. IN NO EVENT SHALL <COPYRIGHT HOLDER> BE LIABLE FOR ANY
DIRECT, INDIRECT, INCIDENTAL, SPECIAL, EXEMPLARY, OR CONSEQUENTIAL DAMAGES
(INCLUDING, BUT NOT LIMITED TO, PROCUREMENT OF SUBSTITUTE GOODS OR SERVICES;
LOSS OF USE, DATA, OR PROFITS; OR BUSINESS INTERRUPTION) HOWEVER CAUSED AND
ON ANY THEORY OF LIABILITY, WHETHER IN CONTRACT, STRICT LIABILITY, OR TORT
(INCLUDING NEGLIGENCE OR OTHERWISE) ARISING IN ANY WAY OUT OF THE USE OF THIS
SOFTWARE, EVEN IF ADVISED OF THE POSSIBILITY OF SUCH DAMAGE.

*/

package main

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"
)

type call struct {
	name     string
	filename string
	args     []string
	line     int
}
type entry struct {
	typ     string
	running time.Duration
	calls   []call
}

type trace struct {
	entries []entry
}

func (t *trace) nextEntry() *entry {
	return &t.entries[0]
}

func (t *trace) query(s string) []entry {
	r := []entry{}
	for i := range t.entries {
		if strings.Contains(t.entries[i].typ, s) {
			r = append(r, t.entries[i])
		}
	}
	return r
}

func getArgs(l string) []string {
	args := strings.Split(l, ",")
	for i := range args {
		args[i] = strings.Trim(args[i], " ")
	}
	return args
}

func (t *trace) load(r *bufio.Reader) error {
	entryIdx := 0
	callIdx := 0
	lineN := 0
	insideEntry := false
	for {
		lineN++
		line, err := r.ReadString('\n')
		if err != nil {
			break
		}
		if strings.Index(line, "goroutine") == 0 {
			insideEntry = true
			i := strings.Index(line, "[")
			i2 := strings.Index(line, "]")

			if i == -1 {
				return fmt.Errorf("line %d: missing '[' that marks the beginning of goroutine info", lineN)
			}
			if i2 == -1 {
				return fmt.Errorf("line %d: missing ']' that marks the end of goroutine info", lineN)
			}
			args := getArgs(line[i+1 : i2])
			runTime := time.Duration(0)
			if len(args) > 1 {
				i := strings.Index(args[1], " ")
				n, _ := strconv.Atoi(args[1][:i])
				runTime = time.Duration(n) * time.Minute
			}
			e := entry{typ: args[0], running: runTime}
			t.entries = append(t.entries, e)
			continue
		}
		if len(line) == 1 {
			insideEntry = false
			entryIdx++
			continue
		}
		if insideEntry {
			if line[0] == ' ' || line[0] == '\t' {
				line = strings.TrimLeft(line, " \t")
				i := strings.Index(line, ":")
				if i == -1 {
					return fmt.Errorf("line %d: didn't find the ':' separator", lineN)
				}
				t.entries[entryIdx].calls[callIdx].filename = line[:i]
				i2 := strings.Index(line[i:], " ")
				if i2 == -1 {
					i2 = strings.Index(line[i:], "\n")
					if i2 == -1 {
						return fmt.Errorf("line %d: didn't find empty space separator", lineN)
					}
				}
				n, err := strconv.Atoi(line[i+1 : i+i2])
				if err != nil {
					panic(err)
				}
				t.entries[entryIdx].calls[callIdx].line = n
			} else {
				if strings.Contains(line, "created by") {
					insideEntry = false
					continue
				}
				fnStart := strings.LastIndex(line, "(")
				if fnStart == -1 {
					continue
				}
				fnStop := strings.LastIndex(line, ")")
				if entryIdx >= len(t.entries) {
					break
				}
				callIdx = len(t.entries[entryIdx].calls)
				c := call{name: line[:fnStart]}
				c.args = getArgs(line[fnStart+1 : fnStop])
				t.entries[entryIdx].calls = append(t.entries[entryIdx].calls, c)
			}
		}
	}
	return nil
}

func loadTrace(r io.Reader) (*trace, error) {
	t := trace{}
	if err := t.load(bufio.NewReader(r)); err != nil {
		return nil, err
	}
	return &t, nil
}

func findLockCall(calls []call) int {
	for i := range calls {
		if (strings.Contains(calls[i].name, "Lock") || strings.Contains(calls[i].name, "RLock")) && strings.Contains(calls[i].filename, "rwmutex.go") {
			return i
		}
	}
	return -1
}

func flattenTrace(calls []call) string {
	flat := ""
	for callIdx := range calls {
		flat += fmt.Sprintf("%s %s:%d\n",
			calls[callIdx].name,
			calls[callIdx].filename,
			calls[callIdx].line)
	}
	return flat
}

func printLocks(t *trace) {
	lockMap := make(map[string][]*entry)
	waits := t.query("semacquire")
	if len(waits) == 0 {
		fmt.Printf("No locks detected!")
		return
	}
	for i := range waits {
		lockI := findLockCall(waits[i].calls) + 1
		if lockI == -1 {
			panic("no call to Lock() or RLock() found")
		}
		// Get the mutex pointer which is a call deeper (sync.Locker.Lock)
		if lockI == 0 {
			lockI = 1
		}
		mutexPtr := waits[i].calls[lockI-1].args[0]
		l := lockMap[mutexPtr]
		l = append(l, &waits[i])
		lockMap[mutexPtr] = l
	}
	fmt.Printf("\033[41mDETECTED %d POSSIBLE DEADLOCK(S)\033[0m\n", len(lockMap))
	type unique struct {
		c     *call
		calls []call
	}
	for k, v := range lockMap {
		u := []unique{}
		lockI := findLockCall(v[0].calls)
		u = append(u, unique{&v[0].calls[lockI+1], v[0].calls})
		for i := range v {
			calls1 := ""
			calls2 := ""
			uniq := true
			idx := findLockCall(v[i].calls) + 1
			// flatten call trace of the current goroutine
			calls1 = flattenTrace(v[i].calls)
			// now flatten every call trace for all unique entries
			// and compare
			for j := range u {
				calls2 = flattenTrace(u[j].calls)
				if calls1 == calls2 {
					// There already is a call trace inside the slice, skip.
					uniq = false
					break
				}
			}
			if uniq {
				u = append(u, unique{&v[i].calls[idx], v[i].calls})
			}
		}
		// Pretty-print the result
		fmt.Printf("- %d call(s) to Lock() for %s, %d unique:\n", len(v), k, len(u))
		for i := range u {
			fmt.Printf("  ┌┤ \033[32m%s\033[0m @ \033[33m%s:%d\033[0m\n", u[i].c.name, u[i].c.filename, u[i].c.line)
			for j := range u[i].calls {
				if j < len(u[i].calls)-1 {
					fmt.Printf("  ├ ")
				} else {
					fmt.Printf("  └ ")
				}
				fmt.Printf("%d: %-40s @ %s:%d\n", j, u[i].calls[j].name, u[i].calls[j].filename, u[i].calls[j].line)
			}

		}
	}
}

func main() {
	var (
		input            io.Reader
		flagLockDetector = flag.Bool("l", true, "run deadlock detector")
	)
	flag.Parse()

	if !(*flagLockDetector) {
		fmt.Printf("Deadlock detector disabled, doesn't make sense ...\n")
		return
	}
	if flag.NArg() > 0 {
		f, err := os.Open(flag.Arg(0))
		if err != nil {
			fmt.Printf("%v\n", err)
			return
		}
		input = f
		defer f.Close()
	} else {
		// Read from stdin
		input = os.Stdin
	}
	t, err := loadTrace(input)
	if err != nil {
		fmt.Printf("%v\n", err)
		return
	}
	if *flagLockDetector {
		printLocks(t)
	}
}
