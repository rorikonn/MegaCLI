package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/megacli/megacli/internal/ipc"
	"github.com/spf13/cobra"
)

var instancesCmd = &cobra.Command{
	Use:   "instances",
	Short: "List running MegaCli instances",
	Long:  "Discover and list other MegaCli instances running on this machine.",
	RunE: func(cmd *cobra.Command, args []string) error {
		instances := ipc.Discover(os.Getpid())
		if len(instances) == 0 {
			fmt.Println("No other MegaCli instances found.")
			return nil
		}

		fmt.Printf("Found %d instance(s):\n\n", len(instances))
		for _, inst := range instances {
			fmt.Printf("  PID:     %d\n", inst.PID)
			fmt.Printf("  Port:    %d\n", inst.Port)
			fmt.Printf("  CWD:     %s\n", inst.CWD)
			fmt.Printf("  Agents:  %s\n", strings.Join(inst.Agents, ", "))
			if inst.Name != "" {
				fmt.Printf("  Name:    %s\n", inst.Name)
			}
			fmt.Printf("  Started: %s\n", inst.StartTime.Format("2006-01-02 15:04:05"))
			fmt.Println()
		}
		return nil
	},
}

var instancesStatusCmd = &cobra.Command{
	Use:   "status [pid]",
	Short: "Query a remote instance's status",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		var pid int
		if _, err := fmt.Sscanf(args[0], "%d", &pid); err != nil {
			return fmt.Errorf("invalid PID: %s", args[0])
		}

		instances := ipc.Discover(os.Getpid())
		var target *ipc.InstanceInfo
		for i := range instances {
			if instances[i].PID == pid {
				target = &instances[i]
				break
			}
		}
		if target == nil {
			return fmt.Errorf("instance PID %d not found", pid)
		}

		client := ipc.NewClientFromInstance(*target)
		status, err := client.QueryStatus()
		if err != nil {
			return fmt.Errorf("failed to query status: %w", err)
		}

		fmt.Printf("Instance PID %d (port %d)\n", status.Instance.PID, status.Instance.Port)
		fmt.Printf("  CWD:  %s\n", status.Instance.CWD)
		fmt.Printf("  Busy: %v\n\n", status.Busy)
		fmt.Println("  Agents:")
		for _, a := range status.Agents {
			fmt.Printf("    - %s (%s) [%s]\n", a.Name, a.Role, a.Status)
		}
		return nil
	},
}

func init() {
	instancesCmd.AddCommand(instancesStatusCmd)
}
