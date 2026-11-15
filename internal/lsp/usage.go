package lsp

import (
	"os/exec"
	"strconv"
	"strings"
)

// Process is the server's process number; 0 when it has none.
func (client *Client) Process() int {
	if client.command == nil || client.command.Process == nil {
		return 0
	}
	return client.command.Process.Pid
}

// DataFolder is the folder the server keeps its index in; "" when it has none.
func (client *Client) DataFolder() string { return client.data }

// MemoryOf is how many bytes of memory a process and those it started take, as the system counts
// what each holds in memory now; 0 when it cannot tell. A server started through a script (jdtls's)
// runs its JVM as a process of that script, and imports Gradle builds in daemons it starts.
func MemoryOf(process int) int64 {
	output, err := exec.Command("ps", "-A", "-o", "pid=,ppid=,rss=").Output()
	if err != nil || process == 0 {
		return 0
	}
	children, memory := map[int][]int{}, map[int]int64{}
	for _, line := range strings.Split(string(output), "\n") {
		fields := strings.Fields(line)
		if len(fields) != 3 {
			continue
		}
		pid, _ := strconv.Atoi(fields[0])
		parent, _ := strconv.Atoi(fields[1])
		kilobytes, _ := strconv.ParseInt(fields[2], 10, 64)
		children[parent] = append(children[parent], pid)
		memory[pid] = kilobytes << 10
	}
	var total int64
	for queue := []int{process}; len(queue) > 0; queue = queue[1:] {
		total += memory[queue[0]]
		queue = append(queue, children[queue[0]]...)
	}
	return total
}
