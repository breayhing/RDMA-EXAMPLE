package main

/*
#cgo LDFLAGS: -libverbs
#include "rdma_operations.h"
*/
import "C"
import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

// 定义全局变量来存储命令行参数
var (
	tcpPort int
	ibDev   string
	ibPort  int
	gidIdx  int
	server  string
)

// Helper function to convert Go strings to C strings
func goStrToCStr(goStr string) *C.char {
	return C.CString(goStr)
}

func main() {

	// 处理 serverName
	args := os.Args
	serverName := ""
	fmt.Println(len(args))
	// COMMENT:设置为2为阈值
	if len(args) == 2 {
		serverName = args[1]
		C.config.server_name = goStrToCStr(args[1])
		fmt.Printf("Client: servername=%s\n", serverName)
	} else if len(args) > 2 {
		os.Exit(1)
	}

	if serverName != "" {
		fmt.Printf("Running in client mode. Connecting to server at %s\n", serverName)
	} else {
		fmt.Println("Running in server mode")
	}

	C.print_config()
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
