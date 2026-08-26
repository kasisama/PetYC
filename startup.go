package main

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"net"
	"strconv"
	"strings"
	"syscall"

	"qq-pet-saas/security"
)

var errStartupCancelled = errors.New("用户取消启动")

const windowsAddressInUse syscall.Errno = 10048

type appStarter func(string, int, func(string) error) error
type availablePortFinder func(string, int) (int, bool)

func runServerWithInteractiveRetry(
	runtimeConfig *security.RuntimeConfig,
	interactive bool,
	input io.Reader,
	output io.Writer,
	onReady func(string),
	start appStarter,
	findPort availablePortFinder,
) error {
	reader := bufio.NewReader(input)
	persistPort := false

	for {
		err := start(runtimeConfig.ListenAddress, runtimeConfig.Port, func(address string) error {
			if persistPort {
				if err := security.SaveRuntimeConfig(*runtimeConfig); err != nil {
					return fmt.Errorf("保存新端口 %d: %w", runtimeConfig.Port, err)
				}
				fmt.Fprintf(output, "[启动] 新端口 %d 已保存到 %s\n", runtimeConfig.Port, security.RuntimeConfigPath())
			}
			if onReady != nil {
				onReady(address)
			}
			return nil
		})
		if err == nil {
			return nil
		}
		if !interactive || !isAddressInUse(err) {
			return err
		}

		fmt.Fprintf(output, "\n[启动] 监听 %s:%d 失败：端口已被占用。\n", runtimeConfig.ListenAddress, runtimeConfig.Port)
		suggestedPort, hasSuggestion := findPort(runtimeConfig.ListenAddress, runtimeConfig.Port)
		for {
			if hasSuggestion {
				fmt.Fprintf(output, "请输入新的端口（直接回车使用建议端口 %d，输入 q 退出）：", suggestedPort)
			} else {
				fmt.Fprint(output, "请输入新的端口（1-65535，输入 q 退出）：")
			}

			line, readErr := reader.ReadString('\n')
			value := strings.TrimSpace(line)
			if readErr != nil && value == "" {
				return fmt.Errorf("读取新端口: %w", readErr)
			}
			if strings.EqualFold(value, "q") {
				return errStartupCancelled
			}
			if value == "" && hasSuggestion {
				runtimeConfig.Port = suggestedPort
				persistPort = true
				break
			}

			port, parseErr := strconv.Atoi(value)
			if parseErr != nil || port < 1 || port > 65535 {
				fmt.Fprintln(output, "端口必须是 1 到 65535 之间的整数。")
				continue
			}
			runtimeConfig.Port = port
			persistPort = true
			break
		}
	}
}

func isAddressInUse(err error) bool {
	if errors.Is(err, syscall.EADDRINUSE) {
		return true
	}
	var errno syscall.Errno
	return errors.As(err, &errno) && errno == windowsAddressInUse
}

func findNextAvailablePort(host string, currentPort int) (int, bool) {
	if strings.TrimSpace(host) == "" {
		host = "127.0.0.1"
	}
	for port := currentPort + 1; port <= 65535; port++ {
		listener, err := net.Listen("tcp", net.JoinHostPort(host, strconv.Itoa(port)))
		if err != nil {
			continue
		}
		_ = listener.Close()
		return port, true
	}
	return 0, false
}
