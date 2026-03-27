package cli

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/spf13/cobra"
)

var installCmd = &cobra.Command{
	Use:   "install",
	Short: "Install xrp to your PATH so it can be used from anywhere",
	RunE: func(cmd *cobra.Command, args []string) error {
		// Find the currently running binary
		self, err := os.Executable()
		if err != nil {
			return fmt.Errorf("could not determine xrp binary path: %w", err)
		}
		self, err = filepath.EvalSymlinks(self)
		if err != nil {
			return fmt.Errorf("could not resolve symlinks: %w", err)
		}

		switch runtime.GOOS {
		case "windows":
			return installWindows(self)
		case "linux", "darwin":
			return installUnix(self)
		default:
			return fmt.Errorf("unsupported platform: %s", runtime.GOOS)
		}
	},
}

func installWindows(self string) error {
	binName := "xrp.exe"
	installDir := filepath.Join(os.Getenv("LOCALAPPDATA"), "xrp", "bin")

	if err := os.MkdirAll(installDir, 0755); err != nil {
		return fmt.Errorf("could not create install directory: %w", err)
	}

	dest := filepath.Join(installDir, binName)
	fmt.Printf("📦 Installing xrp to %s...\n", dest)

	if err := copyFile(self, dest); err != nil {
		return fmt.Errorf("could not copy binary: %w", err)
	}

	// Check if already in user PATH
	currentPath := os.Getenv("PATH")
	if strings.Contains(strings.ToLower(currentPath), strings.ToLower(installDir)) {
		fmt.Printf("✅ xrp is already in your PATH.\n")
		fmt.Printf("   Run 'xrp version' to confirm.\n")
		return nil
	}

	// setx writes to HKCU — no admin rights needed
	fmt.Printf("🔧 Adding %s to your user PATH via setx...\n", installDir)
	newPath := installDir + ";" + currentPath
	out, err := exec.Command("setx", "PATH", newPath).CombinedOutput()
	if err != nil {
		return fmt.Errorf("setx failed: %w\nOutput: %s", err, string(out))
	}

	fmt.Printf("\n✅ xrp installed successfully!\n")
	fmt.Printf("⚡ Open a new terminal and run: xrp version\n")
	fmt.Printf("   (PATH changes take effect in new terminal sessions)\n")
	return nil
}

func installUnix(self string) error {
	installDir := filepath.Join(os.Getenv("HOME"), ".local", "bin")

	if err := os.MkdirAll(installDir, 0755); err != nil {
		return fmt.Errorf("could not create install directory: %w", err)
	}

	dest := filepath.Join(installDir, "xrp")
	fmt.Printf("📦 Installing xrp to %s...\n", dest)

	if err := copyFile(self, dest); err != nil {
		return fmt.Errorf("could not copy binary: %w", err)
	}

	if err := os.Chmod(dest, 0755); err != nil {
		return fmt.Errorf("could not make binary executable: %w", err)
	}

	// Check if ~/.local/bin is already in PATH
	currentPath := os.Getenv("PATH")
	if strings.Contains(currentPath, installDir) {
		fmt.Printf("\n✅ xrp installed successfully!\n")
		fmt.Printf("   Run: xrp version\n")
		return nil
	}

	// Print shell-specific instructions
	fmt.Printf("\n✅ xrp installed to %s\n", dest)
	fmt.Printf("⚡ Add the following to your shell profile (~/.bashrc, ~/.zshrc, etc.):\n\n")
	fmt.Printf("   export PATH=\"%s:$PATH\"\n\n", installDir)
	fmt.Printf("   Then run: source ~/.bashrc  (or open a new terminal)\n")
	return nil
}

func copyFile(src, dst string) error {
	// Remove destination if it exists (allows in-place update)
	_ = os.Remove(dst)

	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return out.Sync()
}

func init() {
	rootCmd.AddCommand(installCmd)
}
