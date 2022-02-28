/*
@Author : LiuSaiNan
@Date : 2021-08-15 18:33
*/

package gossh

import (
	"bytes"
	"fmt"
	"github.com/lsnan/gossh/utils"
	"io"
	"net"
	"os"
	"path"
	"path/filepath"
	"sync"
	"time"

	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
)

type RunOptions struct {
	Watchers []Watcher
}

type SudoOptions struct {
	SudoUser     string
	SudoPassword string
	SudoPattern  string
	Watchers     []Watcher
}

type Connection struct {
	Host         string
	Port         int
	User         string
	Password     string
	auth         []ssh.AuthMethod
	clientConfig *ssh.ClientConfig
	sshClient    *ssh.Client
	*sftp.Client
}

func NewConnection(host, user, password string, port int, timeout int64) (*Connection, error) {
	var err error
	conn := &Connection{Host: host, Port: port, User: user, Password: password}
	conn.auth = make([]ssh.AuthMethod, 0)
	conn.auth = append(conn.auth, ssh.Password(password))
	conn.clientConfig = &ssh.ClientConfig{
		User:            user,
		Auth:            conn.auth,
		HostKeyCallback: ssh.HostKeyCallback(func(hostname string, remote net.Addr, key ssh.PublicKey) error { return nil }),
		Timeout:         time.Duration(timeout) * time.Second,
	}

	address := fmt.Sprintf("%s:%d", conn.Host, conn.Port)

	if conn.sshClient, err = ssh.Dial("tcp", address, conn.clientConfig); err != nil {
		return nil, err
	}

	if conn.Client, err = sftp.NewClient(conn.sshClient); err != nil {
		return nil, err
	}

	return conn, nil
}

func (conn *Connection) Run(cmd string, opts ...RunOptions) (stdoutByte []byte, stderrByte []byte, err error) {
	var opt RunOptions
	var stdin io.WriteCloser
	var stdout io.Reader
	var stderr = new(bytes.Buffer)

	if len(opts) >= 1 {
		opt = opts[0]
	}

	cmd = fmt.Sprintf("PATH=$PATH:/usr/bin:/usr/sbin %s", cmd)
	session, err := conn.sshClient.NewSession()
	if err != nil {
		return stdoutByte, stderrByte, err
	}
	defer session.Close()

	modes := ssh.TerminalModes{
		ssh.ECHO:          0,      // disable echoing
		ssh.TTY_OP_ISPEED: 144000, // input speed = 14.4kbaud
		ssh.TTY_OP_OSPEED: 144000, // output speed = 14.4kbaud
	}

	if err = session.RequestPty("xterm", 80, 40, modes); err != nil {
		return stdoutByte, stderrByte, err
	}
	if stdin, err = session.StdinPipe(); err != nil {
		return stdoutByte, stderrByte, err
	}
	if stdout, err = session.StdoutPipe(); err != nil {
		return stdoutByte, stderrByte, err
	}
	session.Stderr = stderr

	var wt sync.WaitGroup
	wt.Add(1)
	
	go func() {
		defer wg.Done()
		watchers(stdin, stdout, &stdoutByte, opt.Watchers)
	}()

	err = session.Run(cmd)
	wt.Wait()
	if err != nil {
		return stdoutByte, stderr.Bytes(), err
	}
	return stdoutByte, stderr.Bytes(), nil
}

func (conn *Connection) Sudo(cmd string, opts ...SudoOptions) (stdoutByte []byte, stderrByte []byte, err error) {
	var opt SudoOptions
	if len(opts) >= 1 {
		opt = opts[0]
	}

	if opt.SudoUser == "" {
		opt.SudoUser = "root"
	}

	if opt.SudoPassword == "" {
		opt.SudoPassword = conn.Password
	}

	if opt.SudoPattern == "" {
		opt.SudoPattern = "[sudo] password: "
	}

	cmd = fmt.Sprintf("sudo -S -p '%s' -H -u %s /bin/bash -l -c \"cd; %s\"", opt.SudoPattern, opt.SudoUser, cmd)
	watcher := Watcher{Pattern: opt.SudoPattern, Response: opt.SudoPassword}
	opt.Watchers = append(opt.Watchers, watcher)

	return conn.Run(cmd, RunOptions{Watchers: opt.Watchers})
}

func (conn *Connection) Scp(source, target string) error {
	// 如果source是文件, target是文件, 直接调用copy
	// 如果source是文件, target是目录, target 需要加文件名: path.Join(target, path.Base(source))
	// 如果source是目录, target是文件, 报错,目标不是一个路径
	// 如果source是目录, target是目录, 直接遍历copy, 如果target不存在,则自动创建。 如果存在,则创建下一级与 path.Join(target, path.Base(source)) 同名的目录(如果存在, 则报错)

	// 通过ssh协议传输文件的目标机器全都是linux系统, 所以将目标路径强制转换为Linux格式
	target = filepath.ToSlash(target)

	if source == "" {
		return fmt.Errorf("源文件不能为空\n")
	}
	if target == "" {
		return fmt.Errorf("目标路径不能为空\n")
	}
	if !utils.IsExists(source) {
		return fmt.Errorf("文件 %s 不存在\n", source)
	}

	if !utils.IsDir(source) {
		if conn.IsDir(target) {
			target = filepath.ToSlash(path.Join(target, path.Base(filepath.ToSlash(source))))
		}
		return conn.Copy(source, target)
	}

	if !conn.IsExists(target) {
		return conn.LoopCopy(source, target)
	}

	if !conn.IsDir(target) {
		return fmt.Errorf("远程已经存在同名文件: %s\n", target)
	}

	target = filepath.ToSlash(path.Join(target, path.Base(filepath.ToSlash(source))))
	return conn.LoopCopy(source, target)
}

func (conn *Connection) singleCopy(source, target string, path string, info os.FileInfo) error {
	relative, err := filepath.Rel(source, path)
	if err != nil {
		fmt.Println("获取相对路径失败: ", err)
	}

	if info.IsDir() {
		if err := conn.MkdirAll(filepath.ToSlash(filepath.Join(target, relative))); err != nil {
			return err
		}
	} else {
		dir, _ := filepath.Split(filepath.Join(target, relative))
		if err := conn.MkdirAll(filepath.ToSlash(dir)); err != nil {
			return err
		}
		if err := conn.Copy(path, filepath.ToSlash(filepath.Join(target, relative))); err != nil {
			return err
		}
	}
	return err
}

func (conn *Connection) LoopCopy(source, target string) error {
	if err := filepath.Walk(source, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		return conn.singleCopy(source, target, path, info)
	}); err != nil {
		return err
	}
	return nil
}

func (conn *Connection) Copy(source, target string) error {
	var sf *os.File
	var df *sftp.File
	var err error

	// 通过ssh协议传输文件的目标机器全都是linux系统, 所以将目标路径强制转换为Linux格式
	target = filepath.ToSlash(target)

	if sf, err = os.Open(source); err != nil {
		return err
	}
	defer sf.Close()

	if df, err = conn.Create(target); err != nil {
		return err
	}
	defer df.Close()
	if _, err = df.ReadFrom(sf); err != nil {
		return err
	}
	return nil
}

func (conn *Connection) IsExists(path string) bool {
	_, err := conn.Stat(path)
	if err != nil {
		if os.IsExist(err) {
			return true
		}
		return false
	}
	return true
}

func (conn *Connection) IsDir(path string) bool {
	s, err := conn.Stat(path)
	if err != nil {
		return false
	}
	return s.IsDir()
}

func (conn *Connection) IsEmpty(path string) (bool, error) {
	fs, err := conn.ReadDir(path)
	if err != nil {
		return false, err
	}
	if len(fs) == 0 {
		return true, nil
	}
	return false, err
}
