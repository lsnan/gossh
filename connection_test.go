// Created by LiuSainan on 2022-04-28 18:38:47

package gossh

import (
	"fmt"
	"testing"
)

func TestNewConnection(t *testing.T) {
	conn, err := NewConnection("192.168.1.1", "username", "password", 22, 10)
	if err != nil {
		fmt.Println("连接失败 ", err)
		return
	}

	// conn, err := NewConnectionUseKeyFile("192.168.1.1", "username", "/home/username/.ssh/id_rsa", 22, 10)
	// if err != nil {
	// 	fmt.Println("连接失败 ", err)
	// 	return
	// }

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
	// conn.Scp("E:\\test.a", "/home/ls/abc")
}
