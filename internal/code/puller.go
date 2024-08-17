package code

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func PullCode(codeUrl string, outputDir string) error {
	url, err := url.Parse(codeUrl)
	if err != nil {
		return fmt.Errorf("failed to parse url: %w", err)
	}

	switch url.Scheme {
	case "http", "https":
		req, err := http.NewRequest(http.MethodGet, codeUrl, nil)
		if err != nil {
			return fmt.Errorf("failed to create request: %w", err)
		}
		client := http.Client{}
		resp, err := client.Do(req)
		if err != nil {
			return fmt.Errorf("failed to get code: %w", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("failed to get code: %s", resp.Status)
		}

		// Code is a tarball so we must read it and write it to the output directory
		file, err := os.CreateTemp(outputDir, "code-*.tar.gz")
		if err != nil {
			return fmt.Errorf("failed to create file: %w", err)
		}
		defer file.Close()

		_, err = io.Copy(file, resp.Body)
		if err != nil {
			return fmt.Errorf("failed to write file: %w", err)
		}

		// unpack the tarball
		cmd := exec.Command("tar", "-xzf", file.Name(), "-C", outputDir)
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("failed to unpack tarball: %w", err)
		}
		return nil
	case "file":
		// Check if code is a tarball or not
		_, err := os.Stat(url.Path)
		if err != nil {
			return fmt.Errorf("failed to stat file: %w", err)
		}

		// Check if code is a tarball or not

		// Check if the file is a tarball by its extension
		if strings.HasSuffix(url.Path, ".tar.gz") || strings.HasSuffix(url.Path, ".tgz") {
			// unpack the tarball
			cmd := exec.Command("tar", "-xzf", url.Path, "-C", outputDir)
			if err := cmd.Run(); err != nil {
				return fmt.Errorf("failed to unpack tarball: %w", err)
			}
		} else {
			// Copy the file to the output directory
			destPath := filepath.Join(outputDir, filepath.Base(url.Path))
			srcFile, err := os.Open(url.Path)
			if err != nil {
				return fmt.Errorf("failed to open source file: %w", err)
			}
			defer srcFile.Close()

			destFile, err := os.Create(destPath)
			if err != nil {
				return fmt.Errorf("failed to create destination file: %w", err)
			}
			defer destFile.Close()

			_, err = io.Copy(destFile, srcFile)
			if err != nil {
				return fmt.Errorf("failed to copy file: %w", err)
			}
		}
		return nil
	case "git":
		// Clone the git repository
		cmd := exec.Command("git", "clone", codeUrl, outputDir)
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("failed to clone git repository: %w", err)
		}
		return nil
	case "s3":
		// Download the file from S3
		cmd := exec.Command("aws", "s3", "cp", codeUrl, outputDir)
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("failed to download file from S3: %w", err)
		}
		// Check if the file is a tarball by its extension
		if strings.HasSuffix(url.Path, ".tar.gz") || strings.HasSuffix(url.Path, ".tgz") {
			// unpack the tarball
			cmd := exec.Command("tar", "-xzf", url.Path, "-C", outputDir)
			if err := cmd.Run(); err != nil {
				return fmt.Errorf("failed to unpack tarball: %w", err)
			}
		} else {
			// Copy the file to the output directory
			destPath := filepath.Join(outputDir, filepath.Base(url.Path))
			srcFile, err := os.Open(url.Path)
			if err != nil {
				return fmt.Errorf("failed to open source file: %w", err)
			}
			defer srcFile.Close()

			destFile, err := os.Create(destPath)
			if err != nil {
				return fmt.Errorf("failed to create destination file: %w", err)
			}
			defer destFile.Close()

			_, err = io.Copy(destFile, srcFile)
			if err != nil {
				return fmt.Errorf("failed to copy file: %w", err)
			}
		}
		return nil
	}

	return nil
}
