package main

import (
	"bufio"
	"fmt"
	"io/ioutil"
	"os"
	"os/user"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"text/tabwriter"
	"time"
)

func main() {
	filterCh := make(chan string)

	go func() {
		reader := bufio.NewReader(os.Stdin)
		for {
			fmt.Print("Enter a filter: ")
			filter, _ := reader.ReadString('\n')
			filter = strings.TrimSpace(filter)
			filterCh <- filter
		}
	}()

	var filter string
	var filterMu sync.Mutex

	printSystemInfo()

	fmt.Println(strings.Repeat("-", 50))

	for {
		select {
		case newFilter := <-filterCh:
			filterMu.Lock()
			filter = newFilter
			filterMu.Unlock()
		default:
		}

		clearScreen()
		printSystemInfo()

		fmt.Println(strings.Repeat("-", 50))

		filterMu.Lock()
		processes := getProcesses(filter)
		filterMu.Unlock()

		w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', tabwriter.AlignRight|tabwriter.Debug)
		for _, process := range processes {
			fmt.Fprintln(w, process)
		}
		w.Flush()

		time.Sleep(2 * time.Second)
	}
}

func printSystemInfo() {
	os := runtime.GOOS
	fmt.Println("OS:", os)

	numCPU := runtime.NumCPU()
	fmt.Println("Number of CPUs:", numCPU)

	totalMem, usedMem, memUsage := getMemoryInfo()
	fmt.Printf("Total Memory: %.2f GB\n", totalMem/1024/1024)
	fmt.Printf("Used Memory: %.2f GB\n", usedMem/1024/1024)
	fmt.Printf("Memory Usage: %.2f%%\n", memUsage*100)
	printUsageBar("Memory Usage:", memUsage)

	cpuUsage := getCPUUsage()
	fmt.Printf("CPU Usage: %.2f%%\n", cpuUsage*100)
	printUsageBar("CPU Usage:", cpuUsage)
}

func getProcesses(filter string) []string {
	files, _ := ioutil.ReadDir("/proc")
	var output []string
	for _, file := range files {
		if file.IsDir() && isNumeric(file.Name()) {
			pid := file.Name()
			cmdline, _ := ioutil.ReadFile("/proc/" + pid + "/cmdline")
			stat, _ := ioutil.ReadFile("/proc/" + pid + "/stat")
			meminfo, _ := ioutil.ReadFile("/proc/meminfo")
			fields := strings.Fields(string(stat))
			utime, _ := strconv.ParseFloat(fields[13], 64)
			stime, _ := strconv.ParseFloat(fields[14], 64)
			cutime, _ := strconv.ParseFloat(fields[15], 64)
			cstime, _ := strconv.ParseFloat(fields[16], 64)
			starttime, _ := strconv.ParseFloat(fields[21], 64)
			vsize, _ := strconv.ParseFloat(fields[22], 64)
			totalMem, _ := strconv.ParseFloat(strings.Fields(string(meminfo))[1], 64)
			cpuUsage := (utime + stime + cutime + cstime) / starttime
			memUsage := (vsize / 1024) / totalMem
			status := fields[2]
			user := getUserForPid(pid)
			cmd := strings.ReplaceAll(string(cmdline), "\x00", " ")
			cmd = filepath.Base(cmd) 			
      cmd = truncateString(cmd, 50) 
			if strings.Contains(user, filter) || strings.Contains(pid, filter) || strings.Contains(cmd, filter) {
				output = append(output, fmt.Sprintf("%s\t%s\t%s\t%.2f%%\t%.2f%%\t%s", pid, user, status, cpuUsage*100, memUsage*100, cmd))
			}
		}
	}
	return output
}

func getMemoryInfo() (totalMem float64, usedMem float64, memUsage float64) {
	meminfo, _ := ioutil.ReadFile("/proc/meminfo")
	lines := strings.Split(string(meminfo), "\n")
	totalMem, _ = strconv.ParseFloat(strings.Fields(lines[0])[1], 64) 
  freeMem, _ := strconv.ParseFloat(strings.Fields(lines[1])[1], 64) 
  usedMem = totalMem - freeMem
	memUsage = usedMem / totalMem
	return
}

func getCPUUsage() (cpuUsage float64) {
	cpuinfo1, _ := ioutil.ReadFile("/proc/stat")
	time.Sleep(500 * time.Millisecond)
	cpuinfo2, _ := ioutil.ReadFile("/proc/stat")
	cpuTimes1 := strings.Fields(strings.Split(string(cpuinfo1), "\n")[0])[1:]
	cpuTimes2 := strings.Fields(strings.Split(string(cpuinfo2), "\n")[0])[1:]
	var total1, total2, idle1, idle2 float64
	for i, v := range cpuTimes1 {
		t1, _ := strconv.ParseFloat(v, 64)
		t2, _ := strconv.ParseFloat(cpuTimes2[i], 64)
		total1 += t1
		total2 += t2
		if i == 3 {
			idle1 = t1
			idle2 = t2
		}
	}
	cpuUsage = 1 - (idle2-idle1)/(total2-total1)
	return
}

func printUsageBar(label string, usage float64) {
	numBlocks := int(usage * 20)
	fmt.Print(label, " [")
	for i := 0; i < 20; i++ {
		if i < numBlocks {
			fmt.Print("=")
		} else {
			fmt.Print(" ")
		}
	}
	fmt.Println("]")
}

func clearScreen() {
	fmt.Print("\033[H\033[2J")
}

func truncateString(str string, num int) string {
	if len(str) <= num {
		return str
	}
	if num > 3 {
		return str[:num-3] + "..."
	}
	return str[:num]
}

func isNumeric(s string) bool {
	_, err := strconv.ParseFloat(s, 64)
	return err == nil
}

func getUserForPid(pid string) string {
	uidByte, _ := ioutil.ReadFile("/proc/" + pid + "/status")
	uidStr := strings.Split(string(uidByte), "\n")[7]
	uid, _ := strconv.Atoi(strings.Split(uidStr, "\t")[1])
	u, _ := user.LookupId(strconv.Itoa(uid))
	return u.Username
}
