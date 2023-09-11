//go:build windows

package manager

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
)

func Uninstall() error {
	logrus.Infof("Attempting to uninstall plugin")
	_, fileExists, _, ClassesRootKeyExists, err := verifyInstall()
	if err != nil {
		return fmt.Errorf("failed to detect installation status: %v", err)
	}

	if !fileExists && !ClassesRootKeyExists {
		logrus.Info("Did not find anything to uninstall")
		return nil
	}

	logrus.Info("Beginning uninstallation process...")
	err = os.WriteFile(fmt.Sprintf("%s\\%s", baseDir, uninstallFileName), uninstaller, os.ModePerm)
	if err != nil {
		return fmt.Errorf("failed to write install script: %v", err)
	}

	logrus.Info("Executing uninstallation script...")
	cmd := exec.Command("powershell.exe", "-File", fmt.Sprintf("%s\\%s", baseDir, uninstallFileName))
	_, err = cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to execute uninstallation script: %v", err)
	}
	logrus.Info("Successfully executed uninstallation script")

	logrus.Infof("Attempting to remove DLL and tlb files (%s, %s): ", fmt.Sprintf("%s\\%s", baseDir, dllFileName), fmt.Sprintf("%s\\%s", baseDir, tlbFileName))

	if fileExists {
		if successfullyDeletedArtifacts := deleteArtifacts(); !successfullyDeletedArtifacts {
			logrus.Infof("ERROR: Failed to remove DLL directory: %v\n", err)
		} else {
			logrus.Info("DLL and tlb removal complete")
		}
	}

	// write cleanup script to host
	err = os.WriteFile(cleanupFileName, cleanup, os.ModePerm)
	if err != nil {
		return fmt.Errorf("failed to write cleanup script to host: %v", err)
	}

	logrus.Infof("Removal successful! To removal all account provider artifacts please run %s manually", fmt.Sprintf("%s\\%s", baseDir, cleanupFileName))

	return nil
}

func deleteArtifacts() bool {
	// continuously try to remove the actual files.
	// We retry this process a few times because there may
	// still be instances of CCG referencing the DLL. Windows
	// will prevent the file from being deleted if any references
	// still exist. Eventually, the CCG instances will terminate and
	// all references will disappear, at which point the file can be
	// deleted.
	//
	// It goes without saying that if you're uninstalling this plugin,
	// you shouldn't be running workloads which need to use the plugin.
	//
	// This can and should be improved.
	successfulRemoval := false
	for i := 0; i < 10; i++ {
		// remove plugin dll
		err := os.Remove(fmt.Sprintf("%s\\%s", baseDir, dllFileName))
		if err != nil && !strings.Contains(err.Error(), "does not exist") {
			logrus.Infof("encountered error removing tlb directory, some CCG instances may still be referencing the plugin. Will retry in 1 minute: %v", err)
			time.Sleep(1 * time.Minute)
			continue
		}
		// remove type library generated by regsvc
		err = os.Remove(fmt.Sprintf("%s\\%s", baseDir, tlbFileName))
		if err != nil && !strings.Contains(err.Error(), "does not exist") {
			logrus.Infof("encountered error removing tlb , some CCG instances may still be referencing the plugin. Will retry in 1 minute: %v", err)
			time.Sleep(1 * time.Minute)
			continue
		}
		// remove installation script. It's useless without the associated dll
		err = os.Remove(fmt.Sprintf("%s\\%s", baseDir, installFileName))
		if err == nil && !strings.Contains(err.Error(), "does not exist") {
			logrus.Infof("encountered error removing installation script: %v", err)
			successfulRemoval = true
			break
		}
	}
	return successfulRemoval
}
