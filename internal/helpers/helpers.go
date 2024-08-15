package helpers

import (
	"archive/zip"
	"bufio"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"io/ioutil"
	"math/rand"
	"os"
	"path/filepath"
	"pullcsv/internal/logger"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/bitfield/script"
	"github.com/h2non/filetype"
)

// Exists returns whether a file or path exists
func Exists(name string) bool {
	_, err := os.Stat(name)
	return !os.IsNotExist(err)
}

// Move moves a file from a source path to a destination path
// This must be used across the codebase for compatibility with Docker volumes
// and Golang (fixes Invalid cross-device link when using os.Rename)
// The function makes a tmp file at destination path, copy source file to it,
// and then renames tmp file to origin source filename
func Move(sourcePath, destPath string) error {
	sourceAbs, err := filepath.Abs(sourcePath)
	if err != nil {
		return err
	}
	destAbs, err := filepath.Abs(destPath)
	if err != nil {
		return err
	}
	if sourceAbs == destAbs {
		return nil
	}
	inputFile, err := os.Open(sourcePath)
	if err != nil {
		return err
	}

	destDir := filepath.Dir(destPath)
	if !Exists(destDir) {
		err = os.MkdirAll(destDir, 0770)
		if err != nil {
			return err
		}
	}

	tmpDstFileName := destDir + "/tmp_" + filepath.Base(destPath)
	outputFile, err := os.Create(tmpDstFileName)
	if err != nil {
		inputFile.Close()
		return err
	}

	_, err = io.Copy(outputFile, inputFile)
	inputFile.Close()
	outputFile.Close()
	if err != nil {
		if errRem := os.Remove(destPath); errRem != nil {
			return fmt.Errorf(
				"unable to os.Remove error: %s after io.Copy error: %s",
				errRem,
				err,
			)
		}

		return err
	}

	if errRename := os.Rename(tmpDstFileName, destPath); errRename != nil {
		return fmt.Errorf(
			"unable to os.Rename error: %s after io.Copy error: %s",
			errRename,
			err,
		)
	}

	return os.Remove(sourcePath)
}

func IsOlderThan(t time.Time, olderThan int) bool {
	return time.Now().Sub(t) > time.Duration(olderThan)*time.Hour
}

func EnvReplacement(s string) (result string) {
	result = s
	now := time.Now()
	yesterdayNofmt := now.AddDate(0, 0, -1)
	yyyy, mm, dd := now.Date()
	timeBeforeTodayIsYesterday := time.Date(yyyy, mm, dd, 00, 11, 0, 0, now.Location())

	if now.Compare(timeBeforeTodayIsYesterday) < 0 {
		now = yesterdayNofmt
	}

	switch {
	case strings.Contains(s, "_YESTERDAY_"):
		result = strings.ReplaceAll(s, "_YESTERDAY_", yesterdayNofmt.Format("20060102"))
	case strings.Contains(s, "_YES-TER-DAY_"):
		result = strings.ReplaceAll(s, "_YES-TER-DAY_", yesterdayNofmt.Format("2006-01-02"))
	case strings.Contains(s, "_TODAY_"):
		result = strings.ReplaceAll(s, "_TODAY_", now.Format("20060102"))
	case strings.Contains(s, "_TO-DAY_"):
		result = strings.ReplaceAll(s, "_TO-DAY_", now.Format("2006-01-02"))
	}

	return result
}

func AddSeparator(s string) (sl []string, err error) {
	for _, sr := range strings.Fields(s) {
		sr, err := filepath.Abs(sr)
		if err != nil {
			return nil, errors.New("Something was wrong with filepath.Abs")
		}
		sl = append(sl, sr+string(filepath.Separator))
	}
	return sl, err
}

func PrepareEnv() (dFrom, dTo []string, standName, podName string, err error) {

	envVars := []string{
		"DOWNLOAD_FROM",
		"DOWNLOAD_TO",
		"DOWNLOAD_CRON",
		"DELETE_CRON",
		"DELETE_OLDER_THAN",
		"RSYNC_PASSWORD",
		"POD_NAME",
		"STAND_NAME",
	}

	for _, envVar := range envVars {
		_, existance := os.LookupEnv(envVar)
		if !existance && envVar == "DOWNLOAD_CRON" {
			os.Setenv("DOWNLOAD_CRON", "*/10 * * * *")
		} else if !existance && envVar == "DELETE_CRON" {
			os.Setenv("DELETE_CRON", "1 */1 * * *")
		} else if !existance && envVar == "DELETE_OLDER_THAN" {
			os.Setenv("DELETE_OLDER_THAN", "48")
		} else if !existance {
			return nil, nil, "", "", errors.New("Env variable " + envVar + " is not set!")
		}
	}

	var re = regexp.MustCompile(`^[0-9]+$`)
	if !re.MatchString(os.Getenv("DELETE_OLDER_THAN")) {
		return nil, nil, "", "", errors.New("Env variable DELETE_OLDER_THAN must contain only digits!")
	}

	dFrom = strings.Fields(os.Getenv("DOWNLOAD_FROM"))

	dTo, err = AddSeparator(os.Getenv("DOWNLOAD_TO"))
	if err != nil {
		logger.Warn("Something was wrong with filepath.Abs " + " env DOWNLOAD_TO")
	}

	DuplicateEnvs(&dFrom, &dTo)

	lendFrom := len(dFrom)
	if lendFrom != len(dTo) {
		return nil, nil, "", "", errors.New("Number of items in DOWNLOAD_FROM and DOWNLOAD_TO must be equal!")
	}

	standName = os.Getenv("STAND_NAME")
	podName = os.Getenv("POD_NAME")

	return dFrom, dTo, standName, podName, nil
}

func UnzipSource(source, destination string) error {
	reader, err := zip.OpenReader(source)
	if err != nil {
		return err
	}
	defer func() {
		err = errors.Join(err, reader.Close())
	}()

	destination, err = filepath.Abs(destination)
	if err != nil {
		return err
	}

	for _, f := range reader.File {
		err := UnzipFile(f, destination)
		if err != nil {
			return err
		}
	}

	return nil
}

func UnzipFile(f *zip.File, destination string) error {
	//Check if file paths are not vulnerable to Zip Slip
	filePath := filepath.Join(destination, f.Name)
	if !strings.HasPrefix(filePath, filepath.Clean(destination)+string(os.PathSeparator)) {
		return fmt.Errorf("invalid file path: %s", filePath)
	}

	if f.FileInfo().IsDir() {
		if err := os.MkdirAll(filePath, os.ModePerm); err != nil {
			return err
		}
		return nil
	}

	if err := os.MkdirAll(filepath.Dir(filePath), os.ModePerm); err != nil {
		return err
	}

	destinationFile, err := os.OpenFile(filePath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
	if err != nil {
		return err
	}
	defer func() {
		err = errors.Join(err, destinationFile.Close())
	}()

	zippedFile, err := f.Open()
	if err != nil {
		return err
	}
	defer func() {
		err = errors.Join(err, zippedFile.Close())
	}()

	if _, err := io.Copy(destinationFile, zippedFile); err != nil {
		return err
	}
	return nil
}

func UngzipFile(fileName, destination string) error {
	reader, err := os.Open(fileName)
	if err != nil {
		return err
	}
	defer func() {
		err = errors.Join(err, reader.Close())
	}()

	archive, err := gzip.NewReader(reader)
	if err != nil {
		return err
	}
	defer func() {
		err = errors.Join(err, archive.Close())
	}()

	destination = filepath.Join(destination, archive.Name)
	writer, err := os.Create(destination)
	if err != nil {
		return err
	}
	defer func() {
		err = errors.Join(err, writer.Close())
	}()

	_, err = io.Copy(writer, archive)
	return err
}

func UnarchiveFile(fileName, destination string) error {
	buf, err := ioutil.ReadFile(fileName)
	if err != nil {
		return err
	}

	kind, err := filetype.Match(buf)
	if err != nil {
		return err
	}

	if kind == filetype.Unknown {
		return errors.New("Unknown file type " + fileName)
	}

	switch kind.MIME.Value {
	case "application/zip":
		if err := UnzipSource(fileName, destination); err != nil {
			return err
		}
	case "application/gzip":
		if err := UngzipFile(fileName, destination); err != nil {
			return err
		}
	default:
		return errors.New("application/type is not zip or gzip")
	}

	return nil
}

func GetRsyncExitCodeMeaning(code int) (meaning string) {
	codeMeanings := map[int]string{
		0:  "Success",
		1:  "Syntax or usage error",
		2:  "Protocol incompatibility",
		3:  "Errors selecting input/output files, dirs",
		4:  "Requested action not supported: an attempt was made to manipulate 64-bit files on a platform that cannot support them; or an option was specified that is supported by the client and not by the server.",
		5:  "Error starting client-server protocol",
		6:  "Daemon unable to append to log-file",
		10: "Error in socket I/O (maybe there is a problem with DNS resolution)",
		11: "Error in file I/O (maybe there is no destination)",
		12: "Error in rsync protocol data stream",
		13: "Errors with program diagnostics",
		14: "Error in IPC code",
		20: "Received SIGUSR1 or SIGINT",
		21: "Some error returned by waitpid()",
		22: "Error allocating core memory buffers",
		23: "Partial transfer due to error (maybe there are not files on remote side by the mask)",
		24: "Partial transfer due to vanished source files",
		25: "The --max-delete limit stopped deletions",
		30: "Timeout in data send/receive",
		35: "Timeout waiting for daemon connection",
	}
	if _, found := codeMeanings[code]; found {
		return codeMeanings[code]
	} else {
		return "Unknown"
	}
}

func GetUniqueSlice(s []string) []string {
	inResult := make(map[string]bool)
	var result []string
	for _, str := range s {
		if _, ok := inResult[str]; !ok {
			inResult[str] = true
			result = append(result, str)
		}
	}
	return result
}

func SaveExcludeFile(pp, fileName string) (resultToFile []string, err error) {
	f, err := os.Open(fileName)
	if err != nil {
		return nil, err
	}
	defer func() {
		err = errors.Join(err, f.Close())
	}()

	var fileExFN []string
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		fileExFN = append(fileExFN, scanner.Text())
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	var listingpp []string
	err = filepath.Walk(pp, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			listingpp = append(listingpp, filepath.Base(path))
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	lenListingpp := len(listingpp)
	lenFileExFN := len(fileExFN)

	resultToFile = fileExFN
	if lenListingpp > 0 && lenFileExFN > 0 {
		for _, val1 := range listingpp {
			found := false
			for _, val2 := range fileExFN {
				if filepath.Base(val1) == val2 {
					found = true
					break
				}
			}
			if !found {
				resultToFile = append(resultToFile, val1)
			}
		}
	} else if lenListingpp > 0 && lenFileExFN == 0 {
		resultToFile = listingpp
	}

	sort.Strings(resultToFile)
	return resultToFile, err
}

func GetExludeFileName(dFromPath, dToPath string) (excludeFileName string, err error) {
	var deploymentName, tmp1ExcludeFileName string
	re := regexp.MustCompile(`(.+)-([a-z0-9]{8,10}-[a-z0-9]{5})`)
	if deploymentNameSL := re.FindAllStringSubmatch(os.Getenv("POD_NAME"), -1); len(deploymentNameSL) != 0 {
		deploymentName = deploymentNameSL[0][1]
		re = regexp.MustCompile(`(rsync.+@.+)(\/[a-z].+/.+$)`)
		if tmp1ExcludeFileNameSL := re.FindAllStringSubmatch(dFromPath, -1); len(tmp1ExcludeFileNameSL) != 0 {
			tmp1ExcludeFileName = tmp1ExcludeFileNameSL[0][2]

			re = regexp.MustCompile(`\/`)
			tmp2ExcludeFileName := re.ReplaceAllString(tmp1ExcludeFileName, "_")
			dToPathReplaced := re.ReplaceAllString(dToPath, "_")

			re = regexp.MustCompile(`\*`)
			excludeFileName = deploymentName + re.ReplaceAllString(tmp2ExcludeFileName, "") + "-" + os.Getenv("STAND_NAME") + dToPathReplaced + "-excludeFile"
		} else {
			err = errors.New("Couldn't get full OS path on rsyncd server from DOWNLOAD_FROM env variable")
		}
	} else {
		err = errors.New("Couldn't get deployment name from env variable POD_NAME")
	}

	return excludeFileName, err
}

func GetRandStr(length int) string {
	rand.Seed(time.Now().UnixNano())
	b := make([]byte, length)
	rand.Read(b)
	return fmt.Sprintf("%x", b)[:length]
}

func GetCountLines(filename string) (countLines int, err error) {
	countLines, err = script.File(filename).CountLines()
	if err != nil {
		err = errors.New("Could not get CountLines from " + filename + ", the error: " + err.Error())
	}
	return countLines, err
}

func GetFileSize(filename string) (fileSize int64, err error) {
	fi, err := os.Stat(filename)
	if err != nil {
		err = errors.New("Could not get fileSize for " + filename + ", the error: " + err.Error())
		return 0, err
	}
	// get the size
	fileSize = fi.Size()

	return fileSize, err
}

func LogEveryFileAndMoveIt(dTo, p string) (err error) {
	err = filepath.WalkDir(p, func(path string, di fs.DirEntry, err error) error {
		diInfoGet, err := di.Info()
		if err != nil {
			logger.Warn("Could not get info about " + path + ", the error: " + err.Error())
		}
		if !diInfoGet.IsDir() {
			fName := diInfoGet.Name()
			fFullName := p + string(filepath.Separator) + fName
			countLines, err := GetCountLines(fFullName)
			if err != nil {
				logger.Warn(err.Error())
			}
			fileSize, err := GetFileSize(fFullName)
			if err != nil {
				logger.Warn(err.Error())
			}
			logger.Info("The file " + fName + " was downloaded, it has " + strconv.Itoa(countLines) + " lines, size is: " + strconv.FormatInt(fileSize, 10))
			err = Move(fFullName, dTo+fName)
			if err != nil {
				logger.Warn("Could not move the file " + path + ", the error: " + err.Error())
			}
		}

		return nil
	})
	if err != nil {
		err = errors.New("Could not walk through " + p + ", the error: " + err.Error())
	}
	return err
}

func DeleteFiles(p string) (err error) {
	logger.Info("Start deleting old files in " + p)

	err = filepath.WalkDir(p, func(path string, di fs.DirEntry, err error) error {
		diInfoGet, err := di.Info()
		if err != nil {
			logger.Warn("Could not get info about " + path + ", the error: " + err.Error())
		}

		partialFileNameRe := regexp.MustCompile(`\..*\.\w{6}`)
		deleteOlderThan, err := strconv.Atoi(os.Getenv("DELETE_OLDER_THAN"))
		if err != nil {
			logger.Warn("Could not convert env variable DELETE_OLDER_THAN to integer. Skip deleting files..." + err.Error())
			return err
		}
		if IsOlderThan(diInfoGet.ModTime(), deleteOlderThan) && !diInfoGet.IsDir() {
			logger.Info("The file " + path + " is older than 48 hours and will be deleted")
			err := os.Remove(path)
			if err != nil {
				logger.Warn("Could not delete the file " + path + ", the error: " + err.Error())
			}
		} else if IsOlderThan(diInfoGet.ModTime(), 4) && partialFileNameRe.MatchString(diInfoGet.Name()) && !diInfoGet.IsDir() {
			logger.Info("Partial rsync file " + path + " older than 4 hours and will be deleted")
			err := os.Remove(path)
			if err != nil {
				logger.Warn("Could not delete partial rsync file " + path + ", the error: " + err.Error())
			}
		}
		return nil
	})

	logger.Info("Stop deleting old files in " + p)

	return err
}

func Rsync(cmd string) (exitCode int) {
	pipeExec := script.Exec(cmd)
	pipeExec.Wait()
	exitCode = pipeExec.ExitStatus()

	return exitCode
}

func WorkWithArchives(p string) (wwaerr error) {
	logger.Info("Start unarchiving files in " + p)
	fArchives, _ := script.ListFiles(p).Slice()
	re := regexp.MustCompile("(.*zip|.*gz|.*gzip)")
	for _, fArhive := range fArchives {
		if re.MatchString(fArhive) {
			logger.Info("Unarchive the file " + fArhive)
			if err := UnarchiveFile(fArhive, p); err == nil {
				logger.Info("Remove the file " + fArhive)
				os.Remove(fArhive)
			} else {
				wwaerr = errors.New("Something was wrong with unarchive the file " + fArhive + ", the error: " + err.Error())
			}

		}
	}
	logger.Info("Stop unarchiving files in " + p)

	return wwaerr
}

func TruncateExcludeFile(fileName string, i, truncateLines int, fileSize int64) (err error) {
	fi, err := os.Stat(fileName)
	if err != nil {
		return errors.New("Could not find " + fileName + ", the error: " + err.Error())
	}
	if fi.Size() > fileSize {
		tmpFile := "/tmp/exFN" + strconv.Itoa(i)
		if _, err := script.File(fileName).Last(truncateLines).WriteFile(tmpFile); err != nil {
			err = errors.New("Could not write tmp file" + tmpFile + ", the error: " + err.Error())
		}
		Move(tmpFile, fileName) //unhandled error - fix it!
	}

	return err
}

func DuplicateEnvs(dFrom, dTo *[]string) {
	for i := 0; i < len(*dFrom); i++ {
		if strings.Contains((*dFrom)[i], "_TODAY_") {
			*dFrom = append(*dFrom, strings.ReplaceAll((*dFrom)[i], "_TODAY_", "_TO-DAY_"))
			*dTo = append(*dTo, (*dTo)[i])
		}

		if strings.Contains((*dFrom)[i], "_YESTERDAY_") {
			*dFrom = append(*dFrom, strings.ReplaceAll((*dFrom)[i], "_YESTERDAY_", "_YES-TER-DAY_"))
			*dTo = append(*dTo, (*dTo)[i])
		}
	}
}

func GetOldestNewestCountFiles(path string) (newestFileTimestamp, oldestFileTimestamp int64, countFiles int) {
	newestFileTimestamp = 0
	countFiles = 0
	oldestFileTimestamp = time.Now().Unix()
	files, err := os.ReadDir(path)
	if err != nil {
		logger.Warn("The function GetOldestNewestCountFiles couldn't read path '" + path + "' and returned -1 for all metrics. " + "The error: " + err.Error())
		return -1, -1, -1
	}

	for _, file := range files {
		fInfo, _ := file.Info()
		currentFileTimestamp := fInfo.ModTime().Unix()
		if !fInfo.IsDir() && currentFileTimestamp < oldestFileTimestamp {
			oldestFileTimestamp = fInfo.ModTime().Unix()
		}

		if currentFileTimestamp > newestFileTimestamp {
			newestFileTimestamp = currentFileTimestamp
		}

		countFiles++
	}

	return newestFileTimestamp, oldestFileTimestamp, countFiles
}
