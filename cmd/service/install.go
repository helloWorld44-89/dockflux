package service

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/spf13/cobra"
)

const unitTemplate = `[Unit]
Description=dockflux watch daemon
After=network-online.target docker.service
Wants=network-online.target

[Service]
Type=simple
User={{.User}}
WorkingDirectory={{.WorkDir}}
ExecStart={{.Binary}} watch --all --interval {{.Interval}}
Restart=on-failure
RestartSec=10s
StandardOutput=journal
StandardError=journal
SyslogIdentifier=dockflux

[Install]
WantedBy=multi-user.target
`

const unitPath = "/etc/systemd/system/dockflux-watch.service"

var installCmd = &cobra.Command{
	Use:   "install",
	Short: "Install and enable the dockflux-watch systemd service",
	RunE:  runInstall,
}

func init() {
	installCmd.Flags().String("user", "", "User to run the service as (default: current user)")
	installCmd.Flags().String("workdir", "", "Working directory containing dockflux.yml (default: current dir)")
	installCmd.Flags().String("binary", "", "Path to the dockflux binary (default: auto-detected)")
	installCmd.Flags().String("interval", "60s", "Reconciliation interval")
	installCmd.Flags().Bool("start", true, "Start the service immediately after install")
}

func runInstall(cmd *cobra.Command, args []string) error {
	if os.Geteuid() != 0 {
		return fmt.Errorf("service install requires root (run with sudo)")
	}

	r := bufio.NewReader(os.Stdin)

	binary, _ := cmd.Flags().GetString("binary")
	if binary == "" {
		var err error
		binary, err = exec.LookPath("dockflux")
		if err != nil {
			// Fall back to the running executable's path
			binary, _ = os.Executable()
		}
	}

	workdir, _ := cmd.Flags().GetString("workdir")
	if workdir == "" {
		wd, _ := os.Getwd()
		workdir = promptDefault(r, "Working directory (where dockflux.yml lives)", wd)
	}
	workdir, _ = filepath.Abs(workdir)

	user, _ := cmd.Flags().GetString("user")
	if user == "" {
		user = promptDefault(r, "Run service as user", currentUser())
	}

	interval, _ := cmd.Flags().GetString("interval")
	interval = promptDefault(r, "Reconciliation interval (e.g. 30s, 2m, 5m)", interval)

	start, _ := cmd.Flags().GetBool("start")

	// Render the unit file
	type unitVars struct {
		User     string
		WorkDir  string
		Binary   string
		Interval string
	}

	tmpl, err := template.New("unit").Parse(unitTemplate)
	if err != nil {
		return err
	}

	f, err := os.OpenFile(unitPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		return fmt.Errorf("writing unit file: %w", err)
	}
	defer f.Close()

	if err := tmpl.Execute(f, unitVars{
		User:     user,
		WorkDir:  workdir,
		Binary:   binary,
		Interval: interval,
	}); err != nil {
		return fmt.Errorf("rendering unit template: %w", err)
	}

	fmt.Printf("Wrote %s\n", unitPath)

	if err := systemctl("daemon-reload"); err != nil {
		return err
	}
	if err := systemctl("enable", "dockflux-watch.service"); err != nil {
		return err
	}

	if start {
		if err := systemctl("start", "dockflux-watch.service"); err != nil {
			return err
		}
		fmt.Println("Service started. Check status with: dockflux service status")
	} else {
		fmt.Println("Service enabled but not started. Run: systemctl start dockflux-watch")
	}

	return nil
}

func systemctl(args ...string) error {
	cmd := exec.Command("systemctl", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("systemctl %s: %w", strings.Join(args, " "), err)
	}
	return nil
}

func currentUser() string {
	if u := os.Getenv("SUDO_USER"); u != "" {
		return u // prefer the user who invoked sudo
	}
	if u := os.Getenv("USER"); u != "" {
		return u
	}
	return "root"
}

func promptDefault(r *bufio.Reader, label, def string) string {
	fmt.Printf("%s [%s]: ", label, def)
	line, _ := r.ReadString('\n')
	val := strings.TrimSpace(line)
	if val == "" {
		return def
	}
	return val
}
