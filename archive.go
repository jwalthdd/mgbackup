package main

import (
	"archive/zip"
	"crypto/md5"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

func zipFile(pathInputFolder, pathOutPutFilename string) error {
	zipfile, err := os.Create(pathOutPutFilename)
	if err != nil {
		return err
	}
	defer zipfile.Close()

	archive := zip.NewWriter(zipfile)
	defer archive.Close()

	info, err := os.Stat(pathInputFolder)
	if err != nil {
		return nil
	}

	var baseDir string
	if info.IsDir() {
		baseDir = filepath.Base(pathInputFolder)
	}

	filepath.Walk(pathInputFolder, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		header, err := zip.FileInfoHeader(info)
		if err != nil {
			return err
		}

		if baseDir != "" {
			header.Name = filepath.Join(baseDir, strings.TrimPrefix(path, pathInputFolder))
		}

		if info.IsDir() {
			header.Name += "/"
		} else {
			header.Method = zip.Deflate
		}

		writer, err := archive.CreateHeader(header)
		if err != nil {
			return err
		}

		if info.IsDir() {
			return nil
		}

		file, err := os.Open(path)
		if err != nil {
			return err
		}
		defer file.Close()
		_, err = io.Copy(writer, file)
		return err
	})

	return nil
}

func unzipFile(src string, dest string) ([]string, error) {

	var filenames []string

	r, err := zip.OpenReader(src)
	if err != nil {
		return filenames, err
	}
	defer r.Close()

	for _, f := range r.File {

		// Store filename/path for returning and using later on
		fpath := filepath.Join(dest, f.Name)

		if !strings.HasPrefix(fpath, filepath.Clean(dest)+string(os.PathSeparator)) {
			return filenames, fmt.Errorf("%s: illegal file path", fpath)
		}

		filenames = append(filenames, fpath)

		if f.FileInfo().IsDir() {
			os.MkdirAll(fpath, os.ModePerm)
			continue
		}

		if err = os.MkdirAll(filepath.Dir(fpath), os.ModePerm); err != nil {
			return filenames, err
		}

		outFile, err := os.OpenFile(fpath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
		if err != nil {
			return filenames, err
		}

		rc, err := f.Open()
		if err != nil {
			return filenames, err
		}

		_, err = io.Copy(outFile, rc)

		// Close the file without defer to close before next iteration of loop
		outFile.Close()
		rc.Close()

		if err != nil {
			return filenames, err
		}
	}

	return filenames, nil
}

func getFilesInDirectoryOrderByName(directoryPath string) ([]string, error) {
	var files []string
	fileInfo, err := ioutil.ReadDir(directoryPath)
	if err != nil {
		return nil, err
	}

	for _, file := range fileInfo {
		if !file.IsDir() {
			files = append(files, filepath.Join(directoryPath, file.Name()))
		}
	}

	return files, nil
}

func split(fileToBeChunked string, amountChunks int) ([]string, error) {
	tempDir := os.TempDir() + string(os.PathSeparator)
	fileName := filepath.Base(fileToBeChunked)
	filesPath := make([]string, amountChunks)
	file, err := os.Open(fileToBeChunked)

	if err != nil {
		return nil, err
	}

	defer file.Close()

	fileInfo, _ := file.Stat()
	fileSize := fileInfo.Size()

	totalPartsNum := uint64(amountChunks)
	fileChunk := float64(math.Ceil(float64(fileSize) / float64(amountChunks)))
	fmt.Printf("\n Splitting to %d pieces.", totalPartsNum)

	for i := uint64(0); i < totalPartsNum; i++ {
		partSize := int(math.Min(fileChunk, float64(fileSize-int64(uint64(i)*uint64(fileChunk)))))
		partBuffer := make([]byte, partSize)

		file.Read(partBuffer)

		index, _ := strconv.Atoi(strconv.FormatUint(i, 10))

		fileName := tempDir + fileName + "_" + fmt.Sprintf("%03d", index)
		_, err := os.Create(fileName)

		if err != nil {
			return nil, err
		}

		// Write/save buffer to disk
		ioutil.WriteFile(fileName, partBuffer, os.ModeAppend)

		fmt.Printf("\n - Split to %s", fileName)

		filesPath[i] = fileName
	}

	return filesPath, nil
}

func concatFiles(inputFilesPath []string, outputFilePath string) error {
	out, err := os.OpenFile(outputFilePath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer out.Close()

	for _, chunk := range inputFilesPath {
		in, err := os.Open(chunk)
		if err != nil {
			return err
		}

		_, err = io.Copy(out, in)
		if err != nil {
			in.Close()
			return err
		}

		in.Close()
	}

	return nil
}

func getOutPutFile(pathBackupFolder string) (string, error) {

	md5StatPath, err := getMD5StatPath(pathBackupFolder)

	if err != nil {
		return "", errors.New("- The folder doesn't exist or not has read permission")
	}

	// md5(path) + md5(path stat)
	return os.TempDir() + string(os.PathSeparator) +
		getMD5Hash(pathBackupFolder) + "_" +
		md5StatPath + "." + appExt, nil
}

func getMD5StatPath(path string) (string, error) {
	fileInfo, err := os.Stat(path)

	if err != nil {
		return "", err
	}

	hasher := md5.New()
	hasher.Write([]byte(fileInfo.ModTime().String()))

	return hex.EncodeToString(hasher.Sum(nil)), nil
}

func removeLocalTemporaryFiles(zipFile string, zipFileChunks []string) {
	os.Remove(zipFile)

	for _, file := range zipFileChunks {
		os.Remove(file)
	}
}
