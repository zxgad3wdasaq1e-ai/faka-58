// storectl initializes a prebuilt storefront using only the Go standard library.
package main

import (
	"flag"
	"fmt"
	"io"
	"os"
)

func main() { os.Exit(runCLI(os.Args[1:], os.Stdout, os.Stderr)) }

func runCLI(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "用法: storectl init|info|seed [选项]")
		return 2
	}
	flags := flag.NewFlagSet(args[0], flag.ContinueOnError)
	flags.SetOutput(stderr)
	root := flags.String("root", ".", "部署根目录")
	var action func() error
	switch args[0] {
	case "init":
		domain := flags.String("domain", "xioenai.vip", "正式域名，不含协议或端口")
		local := flags.Bool("local", false, "本地 http://localhost:8080")
		image := flags.String("image", "", "带显式 tag 或 sha256 摘要的镜像")
		admin := flags.String("admin-user", "admin", "首次管理员用户名")
		subnet := flags.String("subnet", "172.29.240.0/24", "Docker 私有 IPv4 /24 网段")
		action = func() error {
			_, reused, err := initialize(initOptions{Root: *root, Domain: *domain, Local: *local, Image: *image, AdminUser: *admin, Subnet: *subnet})
			if err != nil {
				return err
			}
			if reused {
				fmt.Fprintln(stdout, "沿用现有部署配置和密钥；没有覆盖或轮换。")
			} else {
				fmt.Fprintln(stdout, "已生成私有配置；首次管理员凭据仅写入 deploy/secrets/initial-admin.txt。")
			}
			return nil
		}
	case "info":
		field := flags.String("field", "", "origin、admin_path 或 image")
		action = func() error {
			value, err := installationInfo(*root, *field)
			if err == nil {
				fmt.Fprintln(stdout, value)
			}
			return err
		}
	case "seed":
		baseURL := flags.String("base-url", "http://127.0.0.1:8080", "管理 API origin（远程仅允许 HTTPS）")
		action = func() error { return seedFromConfig(*root, *baseURL, stdout) }
	default:
		fmt.Fprintln(stderr, "仅支持 init、info、seed 子命令。")
		return 2
	}
	if err := flags.Parse(args[1:]); err != nil {
		return 2
	}
	if flags.NArg() != 0 {
		fmt.Fprintln(stderr, "不支持位置参数。")
		return 2
	}
	if err := action(); err != nil {
		fmt.Fprintln(stderr, "操作未完成:", err)
		return 1
	}
	return 0
}
