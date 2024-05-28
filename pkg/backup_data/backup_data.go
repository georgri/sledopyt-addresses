package backup_data

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"time"
)

const (
	DataFolder     = "./data"
	BackupFolder   = "./data_backup"
	MaxBackupFiles = 48

	BackupEvery = 1 * time.Hour
)

var BackupFileRegexp = regexp.MustCompile(`^data-.*-.*\.tar\.gz$`)

func ArchiveDataFolder() error {
	// tar + gzip
	var buf bytes.Buffer
	err := compress(DataFolder, &buf)
	if err != nil {
		return err
	}

	// write the .tar.gz
	filename := GetBackupFileName()
	err = os.MkdirAll(filepath.Dir(filename), os.FileMode(0777))
	if err != nil {
		return err
	}

	fileToWrite, err := os.OpenFile(filename, os.O_CREATE|os.O_RDWR, os.FileMode(0666))
	if err != nil {
		return err
	}
	if _, err = io.Copy(fileToWrite, &buf); err != nil {
		return err
	}

	return nil
}

func GetBackupFileName() string {
	hostname, _ := os.Hostname()

	timestamp := time.Now().Format(time.RFC3339)

	return fmt.Sprintf("%v/data-%v-%v.tar.gz", BackupFolder, hostname, timestamp)
}

func GetLastBackupFileName() (string, error) {
	filenames, err := GetBackupFileList()
	if err != nil {
		return "", err
	}
	if len(filenames) == 0 {
		return "", fmt.Errorf("no backup files yet")
	}
	return filenames[0], nil
}

func GetBackupFileList() ([]string, error) {
	entries, err := os.ReadDir(BackupFolder)
	if err != nil {
		return nil, err
	}

	var filenames []string

	for _, e := range entries {
		filename := e.Name()
		if !BackupFileRegexp.MatchString(filename) {
			continue
		}
		filenames = append(filenames, fmt.Sprintf("%v/%v", BackupFolder, filename))
	}

	sort.Slice(filenames, func(i, j int) bool {
		return filenames[i] > filenames[j]
	})

	return filenames, err
}

func DeleteExtraBackupFiles() error {
	filenames, err := GetBackupFileList()
	if err != nil {
		return err
	}

	if len(filenames) <= MaxBackupFiles {
		return nil
	}

	for _, filename := range filenames[MaxBackupFiles:] {
		err = os.Remove(filename)
		if err != nil {
			return err
		}
	}

	return nil
}

func BackupDataForever() {
	for {
		log.Printf("backing up data...")
		err := BackupDataOnce()
		if err != nil {
			log.Printf("error while backing up data: %v", err)
		}
		time.Sleep(BackupEvery)
	}
}

func BackupDataOnce() error {
	err := ArchiveDataFolder()
	if err != nil {
		return err
	}

	err = DeleteExtraBackupFiles()
	if err != nil {
		return err
	}

	err = SendLastBackupFile()
	if err != nil {
		return err
	}

	return nil
}
