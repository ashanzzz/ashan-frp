package main

import (
	"bufio"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

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
	fmt.Fprintln(output, "  ashan-frp admin list      列出管理员账号")
	fmt.Fprintln(output, "  ashan-frp admin reset-password --username <用户名>")
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
	case "list":
		if len(args) != 1 {
			return errors.New("admin list 不接受额外参数")
		}
		return listAdministrators(repo, output)
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
	fmt.Fprintln(output, "  ashan-frp admin list")
	fmt.Fprintln(output, "  ashan-frp admin reset-password --username <当前用户名> [--new-username <新用户名>] [--password-stdin]")
}

func listAdministrators(repo *repository.Repository, output io.Writer) error {
	accounts, err := admincli.ListAdministrators(repo)
	if err != nil {
		return fmt.Errorf("读取管理员账号：%w", err)
	}
	if len(accounts) == 0 {
		fmt.Fprintln(output, "未找到管理员账号。请先正常启动服务，让系统完成首次管理员创建。")
		return nil
	}
	fmt.Fprintln(output, "管理员账号：")
	for _, account := range accounts {
		status := "正常"
		if account.LockedUntil != nil && time.Now().Before(*account.LockedUntil) {
			status = "已锁定"
		}
		fmt.Fprintf(output, "- %s\t角色=%s\t状态=%s\n", account.LoginName, account.Role, status)
	}
	return nil
}

func resetAdministratorPassword(repo *repository.Repository, args []string, input io.Reader, output, errorOutput io.Writer, prompt passwordPrompt) error {
	flags := flag.NewFlagSet("reset-password", flag.ContinueOnError)
	flags.SetOutput(errorOutput)
	username := flags.String("username", "", "当前管理员用户名")
	newUsername := flags.String("new-username", "", "可选的新管理员用户名")
	passwordStdin := flags.Bool("password-stdin", false, "从标准输入读取新密码")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return fmt.Errorf("不支持的位置参数：%s", strings.Join(flags.Args(), " "))
	}
	if strings.TrimSpace(*username) == "" {
		return errors.New("必须提供 --username")
	}

	var password string
	var err error
	if *passwordStdin {
		password, err = readPasswordLine(input)
		if err != nil {
			return fmt.Errorf("从标准输入读取密码：%w", err)
		}
	} else {
		if prompt == nil {
			return errors.New("当前环境无法安全读取密码，请使用 --password-stdin")
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
		Username:    *username,
		NewUsername: *newUsername,
		NewPassword: password,
	})
	if err != nil {
		return localizeAdminError(err)
	}
	fmt.Fprintf(output, "管理员账号 %q 的密码已重置，已吊销 %d 个旧令牌。\n", result.LoginName, result.RevokedTokens)
	return nil
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
	case errors.Is(err, admincli.ErrAccountNotFound):
		return errors.New("未找到该管理员账号，可先运行 admin list 查看用户名")
	case errors.Is(err, admincli.ErrNotAdministrator):
		return errors.New("该账号不是管理员账号")
	case errors.Is(err, admincli.ErrPasswordTooShort):
		return errors.New("新密码至少需要 8 个字符")
	case errors.Is(err, admincli.ErrUsernameInvalid):
		return errors.New("用户名长度必须为 1 至 64 个字符")
	case errors.Is(err, admincli.ErrUsernameTaken):
		return errors.New("新用户名已被使用")
	default:
		return err
	}
}
