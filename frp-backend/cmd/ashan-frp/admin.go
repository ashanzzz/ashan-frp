package main

import (
	"bufio"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"ashan-frp/internal/admincli"
	"ashan-frp/internal/config"
	"ashan-frp/internal/database"
	"ashan-frp/internal/repository"
	"ashan-frp/internal/terminal"
)

type passwordPrompt func(label string) (string, error)

func printRootUsage(output io.Writer) {
	fmt.Fprintln(output, "用法：")
	fmt.Fprintln(output, "  ashan-frp                 启动服务")
	fmt.Fprintln(output, "  ashan-frp serve           启动服务")
	fmt.Fprintln(output, "  ashan-frp admin reset-password")
}

func runAdminCommand(cfg config.Config, args []string, input io.Reader, output, errorOutput io.Writer, prompt passwordPrompt) error {
	db, err := database.Open(cfg)
	if err != nil {
		return fmt.Errorf("打开数据库：%w", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		return fmt.Errorf("读取数据库连接：%w", err)
	}
	defer sqlDB.Close()
	return executeAdminCommand(repository.New(db), args, input, output, errorOutput, prompt)
}

func executeAdminCommand(repo *repository.Repository, args []string, input io.Reader, output, errorOutput io.Writer, prompt passwordPrompt) error {
	if len(args) == 0 {
		printAdminUsage(errorOutput)
		return errors.New("缺少管理员子命令")
	}
	switch args[0] {
	case "reset-password":
		return resetAdministratorPassword(repo, args[1:], input, output, errorOutput, prompt)
	case "help", "-h", "--help":
		printAdminUsage(output)
		return nil
	default:
		printAdminUsage(errorOutput)
		return fmt.Errorf("未知管理员子命令：%s", args[0])
	}
}

func printAdminUsage(output io.Writer) {
	fmt.Fprintln(output, "管理员命令：")
	fmt.Fprintln(output, "  ashan-frp admin reset-password")
	fmt.Fprintln(output, "  ashan-frp admin reset-password --new-username <新用户名> --password-stdin")
}

func resetAdministratorPassword(repo *repository.Repository, args []string, input io.Reader, output, errorOutput io.Writer, prompt passwordPrompt) error {
	flags := flag.NewFlagSet("reset-password", flag.ContinueOnError)
	flags.SetOutput(errorOutput)
	newUsername := flags.String("new-username", "", "新的管理员用户名（自动化模式必填）")
	passwordStdin := flags.Bool("password-stdin", false, "从标准输入读取新密码")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return fmt.Errorf("不支持的位置参数：%s", strings.Join(flags.Args(), " "))
	}
	var username string
	var password string
	var err error
	if *passwordStdin {
		username = strings.TrimSpace(*newUsername)
		if username == "" {
			return errors.New("使用 --password-stdin 时必须提供 --new-username")
		}
		password, err = readPasswordLine(input)
		if err != nil {
			return fmt.Errorf("从标准输入读取密码：%w", err)
		}
	} else {
		if prompt == nil {
			return errors.New("当前环境无法安全读取密码，请使用 --password-stdin")
		}
		username, err = promptText(input, output, "请输入新的管理员用户名：")
		if err != nil {
			return err
		}
		password, err = prompt("请输入新密码：")
		if err != nil {
			return err
		}
		confirmation, confirmErr := prompt("请再次输入新密码：")
		if confirmErr != nil {
			return confirmErr
		}
		if password != confirmation {
			return errors.New("两次输入的密码不一致")
		}
	}

	result, err := admincli.ResetPassword(repo, admincli.ResetRequest{
		NewUsername: username,
		NewPassword: password,
	})
	if err != nil {
		return localizeAdminError(err)
	}
	fmt.Fprintf(output, "管理员账号 %q 的密码已重置，已吊销 %d 个旧令牌。\n", result.LoginName, result.RevokedTokens)
	return nil
}

func promptText(input io.Reader, output io.Writer, label string) (string, error) {
	fmt.Fprint(output, label)
	value, err := readPasswordLine(input)
	if err != nil {
		return "", fmt.Errorf("读取用户名：%w", err)
	}
	return strings.TrimSpace(value), nil
}

func readPasswordLine(input io.Reader) (string, error) {
	line, err := bufio.NewReader(input).ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return "", err
	}
	if err != nil && len(line) == 0 {
		return "", err
	}
	return strings.TrimRight(line, "\r\n"), nil
}

func terminalPasswordPrompt(input *os.File, output io.Writer) passwordPrompt {
	return func(label string) (string, error) {
		if !terminal.IsTerminal(int(input.Fd())) {
			return "", errors.New("标准输入不是交互终端，请使用 --password-stdin")
		}
		fmt.Fprint(output, label)
		password, err := terminal.ReadPassword(int(input.Fd()))
		fmt.Fprintln(output)
		if err != nil {
			return "", fmt.Errorf("读取密码失败：%w", err)
		}
		return string(password), nil
	}
}

func localizeAdminError(err error) error {
	switch {
	case errors.Is(err, admincli.ErrNoAdministrator):
		return errors.New("未找到管理员账号，请先正常启动服务完成首次初始化")
	case errors.Is(err, admincli.ErrMultipleAdmins):
		return errors.New("检测到多个管理员账号，数据库不符合单管理员约束，已拒绝重置")
	case errors.Is(err, admincli.ErrPasswordTooShort):
		return errors.New("新密码至少需要 8 个字符")
	case errors.Is(err, admincli.ErrUsernameInvalid):
		return errors.New("用户名长度必须为 1 至 64 个字符")
	default:
		return err
	}
}
