# gossh
远程连接ssh 执行命令, 可以sudo执行, 可以scp拷贝数据

# 使用实例

```
package main

import (
	"fmt"
	"github.com/lsnan/gossh"
)

func main() {
	conn, err := gossh.NewConnection("192.168.1.1", "username", "password", 22, 10)
	if err != nil {
		fmt.Println("连接失败 ", err)
		return
	}

	// 普通用户执行命令
	sdtout, sdterr, err := conn.Run("ls -l /tmp/")
	fmt.Println("错误信息: ", err)
	fmt.Println("输出信息:", string(sdterr))
	fmt.Println("输出信息:", string(sdtout))

	// sudo 执行命令
	sdtout, sdterr, err = conn.Sudo("ls -l /root/")
	fmt.Println("错误信息: ", err)
	fmt.Println("输出信息:", string(sdterr))
	fmt.Println("输出信息:", string(sdtout))

	// scp 文件到远程服务器
	conn.Scp("E:\\test.a", "/home/ls/abc")
}

```
