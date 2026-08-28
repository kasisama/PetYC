package setupwizard

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"golang.org/x/term"
	"qq-pet-saas/security"
)

var ErrInteractiveTerminalRequired = errors.New("首次配置需要交互式终端")

type PasswordReader func(prompt string) (string, error)

func RunLinux(config security.RuntimeConfig, credentials security.Credentials, input io.Reader, output io.Writer, passwordReader PasswordReader) (security.RuntimeConfig, error) {
	reader := bufio.NewReader(input)
	line := func(prompt, fallback string) (string, error) {
		fmt.Fprint(output, prompt)
		value, err := reader.ReadString('\n')
		if err != nil && !errors.Is(err, io.EOF) {
			return "", err
		}
		value = strings.TrimSpace(value)
		if value == "" {
			value = fallback
		}
		return value, nil
	}
	fmt.Fprintln(output, "QQ-Pet v0.1.2 首次配置")
	fmt.Fprintln(output, "配置只保存在当前系统账号的应用数据目录。机器人平台可以暂时跳过。")
	var initialUsername, initialPassword string
	if credentials.PasswordSetupRequired {
		username, err := line("管理员账号 [admin]: ", "admin")
		if err != nil {
			return config, err
		}
		password, err := passwordReader("管理员密码（至少 8 个字符）: ")
		if err != nil {
			return config, err
		}
		confirmation, err := passwordReader("再次输入管理员密码: ")
		if err != nil {
			return config, err
		}
		if len([]rune(password)) < 8 {
			return config, errors.New("管理员密码至少需要 8 个字符")
		}
		if len(password) > 72 {
			return config, errors.New("管理员密码不能超过 72 个字节")
		}
		if password != confirmation {
			return config, errors.New("两次输入的管理员密码不一致")
		}
		initialUsername, initialPassword = username, password
	}
	address, err := line(fmt.Sprintf("监听地址 [%s]: ", config.ListenAddress), config.ListenAddress)
	if err != nil {
		return config, err
	}
	config.ListenAddress = address
	portText, err := line(fmt.Sprintf("后台端口 [%d]: ", config.Port), strconv.Itoa(config.Port))
	if err != nil {
		return config, err
	}
	port, err := strconv.Atoi(portText)
	if err != nil || port < 1 || port > 65535 {
		return config, errors.New("后台端口必须在 1 到 65535 之间")
	}
	config.Port = port
	fmt.Fprintln(output, "接入方式: 0=稍后配置  1=OneBot/NapCat  2=QQ官方机器人  3=两者")
	choice, err := line("请选择 [0]: ", "0")
	if err != nil {
		return config, err
	}
	if choice != "0" && choice != "1" && choice != "2" && choice != "3" {
		return config, errors.New("接入方式只能选择 0、1、2 或 3")
	}
	if choice == "2" || choice == "3" {
		appID, readErr := line("QQ AppID: ", "")
		if readErr != nil {
			return config, readErr
		}
		secret, readErr := passwordReader("QQ AppSecret: ")
		if readErr != nil {
			return config, readErr
		}
		if appID == "" || secret == "" {
			return config, errors.New("QQ AppID 和 AppSecret 必须同时填写")
		}
		config.QQOfficial.AppID = appID
		config.QQOfficial.AppSecret = secret
		config.QQOfficial.GroupEventsEnabled = true
		config.QQOfficial.GuildEventsEnabled = true
	}
	if err = security.SaveRuntimeConfig(config); err != nil {
		return config, err
	}
	if credentials.PasswordSetupRequired {
		if err = security.SetInitialAdminPassword(initialUsername, initialPassword); err != nil {
			return config, err
		}
	}
	if err = security.CompleteSetup(); err != nil {
		return config, err
	}
	fmt.Fprintln(output, "首次配置完成。")
	if choice == "1" || choice == "3" {
		fmt.Fprintf(output, "OneBot 反向 WebSocket: ws://%s:%d/v1/ws\n", clientHost(config.ListenAddress), config.Port)
		fmt.Fprintf(output, "OneBot Token（仅本次显示）: %s\n", config.OneBotToken)
	}
	return config, nil
}

func RunLinuxTerminal(config security.RuntimeConfig, credentials security.Credentials) (security.RuntimeConfig, error) {
	if !term.IsTerminal(int(os.Stdin.Fd())) {
		return config, ErrInteractiveTerminalRequired
	}
	readPassword := func(prompt string) (string, error) {
		fmt.Fprint(os.Stdout, prompt)
		raw, err := term.ReadPassword(int(os.Stdin.Fd()))
		fmt.Fprintln(os.Stdout)
		return string(raw), err
	}
	return RunLinux(config, credentials, os.Stdin, os.Stdout, readPassword)
}

func clientHost(address string) string {
	if address == "0.0.0.0" || address == "::" || address == "" {
		return "127.0.0.1"
	}
	return address
}
