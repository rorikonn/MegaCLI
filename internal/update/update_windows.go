package update

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
)

func replaceBinaryWindows(currentExe, newBinary string) error {
	// Windows locks running executables so we cannot overwrite
	// them directly. We also cannot rely on newBinary staying
	// alive because the caller may clean up the temp directory
	// before the bat script runs.
	//
	// Strategy:
	// 1. Copy the new binary next to the current exe.
	// 2. Launch a bat script that:
	//    a. Retries renaming the running exe until it succeeds
	//       (the rename only works after this process exits).
	//    b. Moves the staged binary into place.
	//    c. Cleans up the old binary and itself.
	stagedPath := currentExe + ".new"
	if err := copyFile(newBinary, stagedPath); err != nil {
		return fmt.Errorf("failed to stage update binary: %w", err)
	}

	oldPath := currentExe + ".old"
	script := fmt.Sprintf("@echo off\r\n"+
		":retry\r\n"+
		"ren \"%s\" \"%s\" >nul 2>&1\r\n"+
		"if exist \"%s\" (\r\n"+
		"    timeout /t 1 /nobreak >nul\r\n"+
		"    goto retry\r\n"+
		")\r\n"+
		"move /y \"%s\" \"%s\" >nul 2>&1\r\n"+
		"del /f \"%s\" >nul 2>&1\r\n"+
		"(goto) 2>nul & del \"%%~f0\"\r\n",
		currentExe, filepath.Base(oldPath),
		currentExe,
		stagedPath, currentExe,
		oldPath,
	)

	scriptPath := currentExe + ".update.bat"
	if err := os.WriteFile(scriptPath, []byte(script), 0o644); err != nil {
		return fmt.Errorf("failed to create update script: %w", err)
	}

	cmd := exec.Command("cmd", "/c", scriptPath)
	cmd.SysProcAttr = &syscall.SysProcAttr{
		CreationFlags: 0x08000000 | 0x00000200, // CREATE_NO_WINDOW | CREATE_NEW_PROCESS_GROUP
	}
	return cmd.Start()
}
