package main

import (
	"crypto/md5"
	_ "crypto/md5"
	"encoding/hex"
	_ "encoding/hex"
	"errors"
	"fmt"
	"github.com/t3rm1n4l/go-mega"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const appExt = "mgbackup"
const appRemoteFolder = "mgbackup"
const configurationFile = "config/conf.yaml"

type credential struct {
	user     string
	password string
}

func main() {
	var err error
	var accounts []credential

	if !isBackupOperation() && !isRestoreOperation() {
		showHelp()
		return
	}

	appPath, err := os.Getwd()
	panicIfError(err)
	configurationFilePath := filepath.Join(appPath, configurationFile)

	conf, err := getConfiguration(configurationFilePath)
	panicIfError(err)

	accounts, err = initAccountsInfo(conf)
	panicIfError(err)

	sessions, err := generateAllSessions(accounts)
	panicIfError(err)

	createRemoteAppFolderIfNotExists(sessions)

	if isRestoreOperation() {
		backupPath, err := getBackupPathFromCommandLineArgument()
		panicIfError(err)
		restorePath, err := getRestorePathFromCommandLineArgument()
		panicIfError(err)
		downloadRemoteBackup(sessions, backupPath, restorePath)
	} else if isBackupOperation() {
		backupPath, err := getBackupPathFromCommandLineArgument()
		panicIfError(err)
		makeBackup(sessions, backupPath)
	}

	fmt.Println()
}

// Retry runs N times until it succeeds or error
func retry(what string, fn func() error) error {
	const maxTries = 10
	var err error

	sleep := 100 * time.Millisecond
	for i := 1; i <= maxTries; i++ {
		err = fn()
		if err == nil {
			return nil
		}
		if err != mega.EAGAIN {
			break
		}
		fmt.Printf("\n - %s failed %d/%d - retrying after %v sleep", what, i, maxTries, sleep)
		time.Sleep(sleep)
		sleep *= 2
	}
	fmt.Printf("\n - %s failed: %v", what, err)

	return err
}

func newMegaSession(response chan *mega.Mega, user string, password string) {
	megaSession := mega.New()

	err := retry("Login", func() error {
		return megaSession.Login(user, password)
	})

	if err == nil {
		response <- megaSession
	} else {
		response <- nil
	}
}

// UploadFile uploads a temporary file of a given size returning the node, name and its MD5SUM
func uploadFile(response chan error, session *mega.Mega, filePath string, parent *mega.Node) {
	resp, _ := session.GetUser()
	var err error = nil

	err = retry(fmt.Sprintf("Upload %q", filePath), func() error {
		_, err := session.UploadFile(filePath, parent, "", nil)

		return err
	})

	if err == nil {
		fmt.Printf("\n - Uploaded file %s to %s", filePath, resp.Email)
	} else {
		fmt.Printf("\n - Error uploading the file %s to %s", filePath, resp.Email)
	}

	response <- err
}

func generateAllSessions(accounts []credential) ([]*mega.Mega, error) {
	var allSessions []*mega.Mega
	var session *mega.Mega

	responses := make(chan *mega.Mega, len(accounts))

	for _, account := range accounts {
		go newMegaSession(responses, account.user, account.password)
	}

	fmt.Printf("\n There are %d mega accounts configured:", len(accounts))

	for i := 0; i < len(accounts); i++ {
		session = <-responses
		if session == nil {
			return nil, errors.New(" - There is an error with the connection")
		}

		allSessions = append(allSessions, session)
		resp, _ := allSessions[i].GetUser()
		fmt.Printf("\n - %s", resp.Email)
	}

	return allSessions, nil
}

func getAppRemoteFolder(session *mega.Mega) *mega.Node {
	node, err := session.FS.PathLookup(session.FS.GetRoot(), []string{appRemoteFolder})

	if err != nil {
		return nil
	}

	return node[0]
}

func listAppRemoteFolder(session *mega.Mega) []*mega.Node {
	remoteFolder := getAppRemoteFolder(session)
	nodes, _ := session.FS.GetChildren(remoteFolder)

	return nodes
}

func getMD5Hash(text string) string {
	hasher := md5.New()
	hasher.Write([]byte(text))

	return hex.EncodeToString(hasher.Sum(nil))
}

func cleanPreviousBackupInAppRemoteFolder(sessions []*mega.Mega, backupFolder string) {
	for _, session := range sessions {
		nodes := listAppRemoteFolder(session)
		for _, node := range nodes {
			if strings.Contains(node.GetName(), getMD5Hash(backupFolder)) {
				retry("Delete", func() error {
					return session.Delete(node, true)
				})
			}
		}
	}
}

func initAccountsInfo(configuration *conf) ([]credential, error) {
	var credentials []credential

	if len(configuration.Account1Login) > 0 {
		credentials = append(credentials, credential{user: configuration.Account1Login, password: configuration.Account1Password})
	}

	if len(configuration.Account2Login) > 0 {
		credentials = append(credentials, credential{user: configuration.Account2Login, password: configuration.Account2Password})
	}

	if len(configuration.Account3Login) > 0 {
		credentials = append(credentials, credential{user: configuration.Account3Login, password: configuration.Account3Password})
	}

	if len(configuration.Account4Login) > 0 {
		credentials = append(credentials, credential{user: configuration.Account4Login, password: configuration.Account4Password})
	}

	if len(credentials) == 0 {
		return nil, errors.New(" - There aren't Mega accounts configured")
	}

	return credentials, nil
}

func areUploadResponsesSuccessful(responses []error) bool {
	for _, response := range responses {
		if response != nil {
			return false
		}
	}

	return true
}

func downloadRemoteBackup(sessions []*mega.Mega, backupFolder string, downloadDestinationPath string) {
	zipFile := filepath.Join(downloadDestinationPath, getMD5Hash(backupFolder)+"."+appExt)
	err := downloadRemoteBackupFiles(sessions, backupFolder, downloadDestinationPath)
	panicIfError(err)
	downloadedFiles, err := getFilesInDirectoryOrderByName(downloadDestinationPath)
	panicIfError(err)
	err = concatFiles(downloadedFiles, zipFile)
	panicIfError(err)
	_, err = unzipFile(zipFile, downloadDestinationPath)
	showIfError(err)
	removeLocalTemporaryFiles(zipFile, downloadedFiles)
}

func downloadRemoteBackupFiles(sessions []*mega.Mega, backupFolder string, downloadDestinationPath string) error {
	responses := make(chan error, len(sessions))
	var downloadFiles []error

	for _, session := range sessions {
		nodes := listAppRemoteFolder(session)
		for _, node := range nodes {
			if strings.Contains(node.GetName(), getMD5Hash(backupFolder)) {
				go downloadRemoteFile(responses, session, node, downloadDestinationPath)
				time.Sleep(2 * time.Second)
			}
		}
	}

	for i := 0; i < len(sessions); i++ {
		downloadFiles = append(downloadFiles, <-responses)
	}

	for _, file := range downloadFiles {
		if file != nil {
			return file
		}
	}

	return nil
}

func downloadRemoteFile(response chan error, session *mega.Mega, src *mega.Node, restorePath string) {
	err := retry("Download", func() error {
		return session.DownloadFile(src, filepath.Join(restorePath, src.GetName()), nil)
	})

	response <- err
}

func isRestoreOperation() bool {
	return len(os.Args) == 4 && strings.ToLower(os.Args[1]) == "-r"
}

func isBackupOperation() bool {
	return len(os.Args) == 3 && strings.ToLower(os.Args[1]) == "-b"
}

func getRestorePathFromCommandLineArgument() (string, error) {
	return filepath.Abs(os.Args[3])
}

func getBackupPathFromCommandLineArgument() (string, error) {
	return filepath.Abs(os.Args[2])
}

func makeBackup(sessions []*mega.Mega, backupPath string) {
	var fileChunks []string = nil
	var responses chan error
	var uploadResponses []error

	outputFile, err := getOutPutFile(backupPath)
	panicIfError(err)
	err = zipFile(backupPath, outputFile)
	panicIfError(err)

	fileChunks, err = split(outputFile, len(sessions))
	panicIfError(err)

	responses = make(chan error, len(fileChunks))

	cleanPreviousBackupInAppRemoteFolder(sessions, backupPath)

	if fileChunks != nil {
		for i, filePath := range fileChunks {
			go uploadFile(responses, sessions[i], filePath, getAppRemoteFolder(sessions[i]))
		}
	}

	for i := 0; i < len(fileChunks); i++ {
		uploadResponses = append(uploadResponses, <-responses)
	}

	if areUploadResponsesSuccessful(uploadResponses) == true {
		fmt.Printf("\n - Upload successful\n")
	} else {
		// If something went bad clean the files uploaded to mega during the failed backup process
		cleanPreviousBackupInAppRemoteFolder(sessions, backupPath)
		fmt.Printf("\n - Upload fail\n")
	}

	removeLocalTemporaryFiles(outputFile, fileChunks)
}

func createRemoteAppFolderIfNotExists(sessions []*mega.Mega) {
	for _, session := range sessions {
		if getAppRemoteFolder(session) == nil {
			session.CreateDir(appRemoteFolder, session.FS.GetRoot())
			time.Sleep(2 * time.Second)
		}
	}
}

func showHelp() {
	fmt.Println("mgbackup:" +
		"\n Backup a folder: mgbackup -b <folder_local_path>" +
		"\n Restore a remote backup: mgbackup -r <folder_local_path> <folder_destination>")
}
