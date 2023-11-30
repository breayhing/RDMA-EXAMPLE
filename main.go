package main

/*
#cgo LDFLAGS: -libverbs
#include "rdma_operations.h"
*/
import "C"
import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"strings"
	"unsafe"
)

// 定义全局变量来存储命令行参数
var (
	tcpPort int
	ibDev   string
	ibPort  int
	gidIdx  int
	server  string
)

func init() {
	// 定义命令行参数
	flag.IntVar(&tcpPort, "port", 18515, "listen on/connect to port <port>")
	flag.StringVar(&ibDev, "ib-dev", "", "use IB device <dev>")
	flag.IntVar(&ibPort, "ib-port", 1, "use port <port> of IB device")
	flag.IntVar(&gidIdx, "gid-idx", -1, "gid index to be used in GRH")
	flag.StringVar(&server, "server", "", "connect to server at <host>")
}

// Helper function to convert Go strings to C strings
func goStrToCStr(goStr string) *C.char {
	return C.CString(goStr)
}

func main() {
	flag.Parse()

	// 将命令行参数的值传递给 C 结构体
	// 注意: 在 C 中处理字符串参数需要更多注意，因为 Go 字符串和 C 字符串在内存管理上有所不同
	C.config.tcp_port = C.uint(tcpPort)
	C.config.ib_port = C.int(ibPort)
	C.config.gid_idx = C.int(gidIdx)
	if ibDev != "" {
		C.config.dev_name = C.CString(ibDev)
		defer C.free(unsafe.Pointer(C.config.dev_name))
	}
	if server != "" {
		C.config.server_name = C.CString(server)
		defer C.free(unsafe.Pointer(C.config.server_name))
		fmt.Printf("Running in client mode. Connecting to server at %s\n", server)
	} else {
		fmt.Println("Running in server mode")
	}

	//开始初始化的部分
	var res C.struct_resources

	C.resources_init(&res)
	if C.resources_create(&res) != 0 {
		fmt.Fprintf(os.Stderr, "failed to create resources\n")
		return
	}

	// 连接队列对
	if C.connect_qp(&res) != 0 {
		fmt.Fprintf(os.Stderr, "failed to connect QPs\n")
		return
	}

	reader := bufio.NewReader(os.Stdin)

	// 交互循环
	for {
		var tempChar C.char

		// 服务器逻辑
		if C.config.server_name == nil {
			fmt.Print("Server: Enter your message (type 'exit' to end): ")
			input, _ := reader.ReadString('\n')
			input = strings.TrimSuffix(input, "\n")
			if input == "exit" {
				break
			}
			C.strcpy(res.buf, C.CString(input))
			if C.post_send(&res, C.IBV_WR_SEND) != 0 {
				fmt.Fprintf(os.Stderr, "Server: failed to post send\n")
				break
			}
		}

		// 客户端逻辑
		if C.config.server_name != nil {
			if C.poll_completion(&res) != 0 {
				fmt.Fprintf(os.Stderr, "Client: poll completion failed\n")
				break
			}
			fmt.Printf("Client: Server's message: '%s'\n", C.GoString(res.buf))

			fmt.Print("Client: Enter your message (type 'exit' to end): ")
			input, _ := reader.ReadString('\n')
			input = strings.TrimSuffix(input, "\n")
			if input == "exit" {
				break
			}
			C.strcpy(res.buf, C.CString(input))
			if C.post_send(&res, C.IBV_WR_SEND) != 0 {
				fmt.Fprintf(os.Stderr, "Client: failed to post send\n")
				break
			}
		}

		// 同步
		if C.sock_sync_data(res.sock, 1, goStrToCStr("W"), &tempChar) != 0 {
			fmt.Fprintf(os.Stderr, "sync error\n")
			break
		}

	}

	if C.resources_destroy(&res) != 0 {
		fmt.Fprintf(os.Stderr, "failed to destroy resources\n")
	}
}
